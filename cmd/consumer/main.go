package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gohopper/internal/logger"
	"gohopper/internal/processor"
	"gohopper/internal/queue"

	"github.com/joho/godotenv"
	"github.com/streadway/amqp"
)

func main() {
	var workerCount int
	var consumerTag string

	flag.IntVar(&workerCount, "workers", 5, "Number of worker goroutines")
	flag.StringVar(&consumerTag, "tag", "gohopper-consumer", "Consumer tag")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		fmt.Printf("Warning: .env file not found\n")
	}

	loggerInstance := logger.NewLogger()
	ctx := logger.CreateTraceContext()

	loggerInstance.Info(ctx, "Gohopper Consumer starting", logger.Fields{
		"mode":         "consumer",
		"worker_count": workerCount,
		"consumer_tag": consumerTag,
	})

	queueManager := queue.NewManager()

	// Connect to RabbitMQ
	if err := queueManager.Connect(); err != nil {
		loggerInstance.Fatal(ctx, "Failed to connect to RabbitMQ", logger.Fields{
			"error": err.Error(),
		})
	}
	defer queueManager.Close()

	// Configure queues and exchanges
	if err := queueManager.SetupQueues(); err != nil {
		loggerInstance.Fatal(ctx, "Failed to setup queues", logger.Fields{
			"error": err.Error(),
		})
	}

	loggerInstance.Info(ctx, "Consumer configured successfully", nil)

	handler := processor.NewDefaultMessageHandler()

	workerPool := processor.NewWorkerPool(workerCount, handler)

	// Start worker pool
	if err := workerPool.Start(ctx); err != nil {
		loggerInstance.Fatal(ctx, "Failed to start worker pool", logger.Fields{
			"error": err.Error(),
		})
	}

	consumerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start consuming messages
	if err := queueManager.ConsumeMessages(consumerCtx, consumerTag, func(message *queue.EventMessage, delivery amqp.Delivery) error {
		return processMessageWithWorkerPool(consumerCtx, message, delivery, workerPool)
	}); err != nil {
		loggerInstance.Fatal(ctx, "Failed to start consuming messages", logger.Fields{
			"error": err.Error(),
		})
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go reportStats(ctx, queueManager, workerPool)

	// Wait for shutdown signal
	<-sigChan
	loggerInstance.Info(ctx, "Shutting down consumer", nil)

	if err := workerPool.Stop(ctx); err != nil {
		loggerInstance.Error(ctx, "Error stopping worker pool", err, nil)
	}

	cancel()
	loggerInstance.Info(ctx, "Consumer stopped successfully", nil)
}

// processMessageWithWorkerPool processes a message using the worker pool
func processMessageWithWorkerPool(ctx context.Context, message *queue.EventMessage, delivery amqp.Delivery, workerPool *processor.WorkerPool) error {
	job := &processor.Job{
		Message:    message,
		Delivery:   delivery,
		ReceivedAt: time.Now(),
		RetryCount: message.Metadata.RetryCount,
		MaxRetries: getMaxRetries(),
		RetryDelay: getRetryDelay(),
	}

	if err := workerPool.SubmitJob(ctx, job); err != nil {
		return fmt.Errorf("failed to submit job to worker pool: %w", err)
	}

	return nil
}

// reportStats reports queue and worker pool statistics periodically
func reportStats(ctx context.Context, queueManager *queue.Manager, workerPool *processor.WorkerPool) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	loggerInstance := logger.NewLogger()

	for {
		select {
		case <-ticker.C:
			queueStats, err := queueManager.GetQueueStats()
			if err != nil {
				loggerInstance.Error(ctx, "Failed to get queue stats", err, nil)
				continue
			}

			workerStats := workerPool.GetStats()

			loggerInstance.Info(ctx, "System statistics", logger.Fields{
				"queue_stats":  queueStats,
				"worker_stats": workerStats,
			})

		case <-ctx.Done():
			return
		}
	}
}

// getMaxRetries gets the maximum number of retries from environment
func getMaxRetries() int {
	if retries := os.Getenv("MAX_RETRIES"); retries != "" {
		if val, err := strconv.Atoi(retries); err == nil {
			return val
		}
	}
	return 3
}

// getRetryDelay gets the retry delay from environment
func getRetryDelay() time.Duration {
	if delay := os.Getenv("RETRY_DELAY"); delay != "" {
		if val, err := time.ParseDuration(delay); err == nil {
			return val
		}
	}
	return 1000 * time.Millisecond
}
