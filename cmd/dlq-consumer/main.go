package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"gohopper/configs"
	"gohopper/internal/logger"
	"gohopper/internal/queue"

	"github.com/joho/godotenv"
	"github.com/streadway/amqp"
)

func main() {
	var consumerTag string

	flag.StringVar(&consumerTag, "tag", "gohopper-dlq-consumer", "DLQ Consumer tag")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		fmt.Printf("Warning: .env file not found\n")
	}

	cfg := configs.Load()

	loggerInstance := logger.NewLogger()
	ctx := logger.CreateTraceContext()

	loggerInstance.Info(ctx, "Gohopper DLQ Consumer starting", logger.Fields{
		"mode":         "dlq-consumer",
		"consumer_tag": consumerTag,
		"config":       cfg,
	})

	queueManager := queue.NewManager()

	if err := queueManager.Connect(); err != nil {
		loggerInstance.Fatal(ctx, "Failed to connect to RabbitMQ", logger.Fields{
			"error": err.Error(),
		})
	}
	defer queueManager.Close()

	if err := queueManager.SetupQueues(); err != nil {
		loggerInstance.Fatal(ctx, "Failed to setup queues", logger.Fields{
			"error": err.Error(),
		})
	}

	loggerInstance.Info(ctx, "DLQ Consumer configured successfully", nil)

	mainCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	if err := queueManager.ConsumeDLQMessages(mainCtx, consumerTag, func(message *queue.EventMessage, delivery amqp.Delivery) error {
		return processDLQMessage(mainCtx, message, delivery, loggerInstance)
	}); err != nil {
		loggerInstance.Fatal(ctx, "Failed to start consuming DLQ messages", logger.Fields{
			"error": err.Error(),
		})
	}

	// Setup graceful shutdown with signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	wg.Add(1)
	go func() {
		defer wg.Done()
		reportDLQStats(mainCtx, queueManager, loggerInstance, cfg)
	}()

	loggerInstance.Info(ctx, "DLQ Consumer started successfully", logger.Fields{
		"consumer_tag": consumerTag,
	})

	<-sigChan
	loggerInstance.Info(ctx, "Shutdown signal received", logger.Fields{
		"signal": "SIGINT/SIGTERM",
	})

	cancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		loggerInstance.Info(ctx, "All goroutines stopped gracefully", nil)
	case <-time.After(cfg.Consumer.ShutdownTimeout):
		loggerInstance.Warn(ctx, "Timeout waiting for goroutines to stop", logger.Fields{
			"timeout": cfg.Consumer.ShutdownTimeout,
		})
	}

	loggerInstance.Info(ctx, "DLQ Consumer stopped successfully", nil)
}

// processDLQMessage processes a message from the DLQ
func processDLQMessage(ctx context.Context, message *queue.EventMessage, delivery amqp.Delivery, loggerInstance *logger.Logger) error {
	loggerInstance.Info(ctx, "Processing DLQ message", logger.Fields{
		"message_id":   message.ID,
		"message_type": message.Type,
		"source":       message.Source,
		"dlq_reason":   message.Metadata.DLQReason,
		"final_error":  message.Metadata.FinalError,
		"retry_count":  message.Metadata.RetryCount,
	})

	switch message.Metadata.DLQReason {
	case "max_retries_exceeded":
		return handleMaxRetriesDLQ(ctx, message, delivery, loggerInstance)
	case "processing_timeout":
		return handleTimeoutDLQ(ctx, message, delivery, loggerInstance)
	case "non_retryable_failure":
		return handleNonRetryableDLQ(ctx, message, delivery, loggerInstance)
	default:
		return handleUnknownDLQ(ctx, message, delivery, loggerInstance)
	}
}

// handleMaxRetriesDLQ handles messages that exceeded max retries
func handleMaxRetriesDLQ(ctx context.Context, message *queue.EventMessage, delivery amqp.Delivery, loggerInstance *logger.Logger) error {
	loggerInstance.Warn(ctx, "Processing max retries exceeded message", logger.Fields{
		"message_id":  message.ID,
		"retry_count": message.Metadata.RetryCount,
		"final_error": message.Metadata.FinalError,
	})

	if err := delivery.Ack(false); err != nil {
		loggerInstance.Error(ctx, "Failed to acknowledge DLQ message", err, logger.Fields{
			"message_id": message.ID,
		})
		return err
	}

	loggerInstance.Info(ctx, "DLQ message processed successfully", logger.Fields{
		"message_id": message.ID,
		"dlq_reason": message.Metadata.DLQReason,
	})

	return nil
}

// handleTimeoutDLQ handles timeout messages
func handleTimeoutDLQ(ctx context.Context, message *queue.EventMessage, delivery amqp.Delivery, loggerInstance *logger.Logger) error {
	loggerInstance.Warn(ctx, "Processing timeout message", logger.Fields{
		"message_id":  message.ID,
		"final_error": message.Metadata.FinalError,
	})

	if err := delivery.Ack(false); err != nil {
		loggerInstance.Error(ctx, "Failed to acknowledge DLQ message", err, logger.Fields{
			"message_id": message.ID,
		})
		return err
	}

	loggerInstance.Info(ctx, "DLQ timeout message processed", logger.Fields{
		"message_id": message.ID,
	})

	return nil
}

// handleNonRetryableDLQ handles non-retryable failure messages
func handleNonRetryableDLQ(ctx context.Context, message *queue.EventMessage, delivery amqp.Delivery, loggerInstance *logger.Logger) error {
	loggerInstance.Error(ctx, "Processing non-retryable failure message", fmt.Errorf(message.Metadata.FinalError), logger.Fields{
		"message_id":  message.ID,
		"final_error": message.Metadata.FinalError,
	})

	if err := delivery.Ack(false); err != nil {
		loggerInstance.Error(ctx, "Failed to acknowledge DLQ message", err, logger.Fields{
			"message_id": message.ID,
		})
		return err
	}

	loggerInstance.Info(ctx, "DLQ non-retryable message processed", logger.Fields{
		"message_id": message.ID,
	})

	return nil
}

// handleUnknownDLQ handles unknown DLQ reasons
func handleUnknownDLQ(ctx context.Context, message *queue.EventMessage, delivery amqp.Delivery, loggerInstance *logger.Logger) error {
	loggerInstance.Warn(ctx, "Processing unknown DLQ reason", logger.Fields{
		"message_id":  message.ID,
		"dlq_reason":  message.Metadata.DLQReason,
		"final_error": message.Metadata.FinalError,
	})

	if err := delivery.Ack(false); err != nil {
		loggerInstance.Error(ctx, "Failed to acknowledge DLQ message", err, logger.Fields{
			"message_id": message.ID,
		})
		return err
	}

	loggerInstance.Info(ctx, "DLQ unknown message processed", logger.Fields{
		"message_id": message.ID,
	})

	return nil
}

// reportDLQStats reports DLQ statistics periodically
func reportDLQStats(ctx context.Context, queueManager *queue.Manager, loggerInstance *logger.Logger, cfg *configs.Config) {
	ticker := time.NewTicker(cfg.Consumer.StatsReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			queueStats, err := queueManager.GetQueueStats()
			if err != nil {
				loggerInstance.Error(ctx, "Failed to get queue stats", err, nil)
				continue
			}

			if dlqStats, ok := queueStats["dlq"].(map[string]interface{}); ok {
				loggerInstance.Info(ctx, "DLQ statistics", logger.Fields{
					"dlq_stats": dlqStats,
				})
			}

		case <-ctx.Done():
			loggerInstance.Info(ctx, "DLQ stats reporting stopped", nil)
			return
		}
	}
}
