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

	cfg := configs.Load()

	loggerInstance := logger.NewLogger()
	ctx := logger.CreateTraceContext()

	loggerInstance.Info(ctx, "Gohopper Consumer starting", logger.Fields{
		"mode":         "consumer",
		"worker_count": workerCount,
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

	loggerInstance.Info(ctx, "Consumer configured successfully", nil)

	handler := processor.NewDefaultMessageHandler()
	workerPool := processor.NewWorkerPool(workerCount, handler)

	if err := workerPool.Start(ctx); err != nil {
		loggerInstance.Fatal(ctx, "Failed to start worker pool", logger.Fields{
			"error": err.Error(),
		})
	}

	mainCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup

	// Start consuming messages
	if err := queueManager.ConsumeMessages(mainCtx, consumerTag, func(message *queue.EventMessage, delivery amqp.Delivery) error {
		return processMessageWithWorkerPool(mainCtx, message, delivery, workerPool, cfg)
	}); err != nil {
		loggerInstance.Fatal(ctx, "Failed to start consuming messages", logger.Fields{
			"error": err.Error(),
		})
	}

	// Setup graceful shutdown with signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	wg.Add(1)
	go func() {
		defer wg.Done()
		reportStats(mainCtx, queueManager, workerPool, cfg)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		healthCheck(mainCtx, workerPool, loggerInstance, cfg)
	}()

	loggerInstance.Info(ctx, "Consumer started successfully", logger.Fields{
		"worker_count": workerCount,
		"consumer_tag": consumerTag,
	})

	// Wait for shutdown signal
	<-sigChan
	loggerInstance.Info(ctx, "Shutdown signal received", logger.Fields{
		"signal": "SIGINT/SIGTERM",
	})

	cancel()

	if err := workerPool.Stop(ctx); err != nil {
		loggerInstance.Error(ctx, "Error stopping worker pool", err, nil)
	}

	// Wait for all goroutines to finish with timeout
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

	loggerInstance.Info(ctx, "Consumer stopped successfully", nil)
}

// processMessageWithWorkerPool processes a message using the worker pool
func processMessageWithWorkerPool(ctx context.Context, message *queue.EventMessage, delivery amqp.Delivery, workerPool *processor.WorkerPool, cfg *configs.Config) error {
	job := &processor.Job{
		Message:      message,
		Delivery:     delivery,
		ReceivedAt:   time.Now(),
		RetryCount:   message.Metadata.RetryCount,
		MaxRetries:   cfg.Worker.MaxRetries,
		RetryDelay:   cfg.Worker.RetryDelay,
		RetryTimeout: cfg.Worker.RetryTimeout,
	}

	if err := workerPool.SubmitJob(ctx, job); err != nil {
		return fmt.Errorf("failed to submit job to worker pool: %w", err)
	}

	return nil
}

// reportStats reports queue and worker pool statistics periodically
func reportStats(ctx context.Context, queueManager *queue.Manager, workerPool *processor.WorkerPool, cfg *configs.Config) {
	ticker := time.NewTicker(cfg.Consumer.StatsReportInterval)
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
			loggerInstance.Info(ctx, "Stats reporting stopped", nil)
			return
		}
	}
}

// healthCheck performs periodic health checks on the worker pool
func healthCheck(ctx context.Context, workerPool *processor.WorkerPool, loggerInstance *logger.Logger, cfg *configs.Config) {
	ticker := time.NewTicker(cfg.Consumer.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := workerPool.GetStats()

			if !workerPool.IsActive() {
				loggerInstance.Warn(ctx, "Worker pool is not active", logger.Fields{
					"stats": stats,
				})
			}

			if stats["context_done"].(bool) {
				loggerInstance.Warn(ctx, "Worker pool context is done", logger.Fields{
					"stats": stats,
				})
			}

		case <-ctx.Done():
			loggerInstance.Info(ctx, "Health check stopped", nil)
			return
		}
	}
}
