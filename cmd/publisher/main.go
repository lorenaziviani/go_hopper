package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gohopper/internal/logger"
	"gohopper/internal/queue"

	"github.com/joho/godotenv"
)

func main() {
	var cliMode bool
	flag.BoolVar(&cliMode, "cli", false, "Run in CLI mode to publish test events")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	// Create logger
	loggerInstance := logger.NewLogger()
	ctx := logger.CreateTraceContext()

	loggerInstance.Info(ctx, "Gohopper Publisher starting", logger.Fields{
		"mode":     "publisher",
		"cli_mode": cliMode,
	})

	// Create queue manager
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

	loggerInstance.Info(ctx, "Publisher configured successfully", nil)

	if cliMode {
		publishTestEvents(ctx, queueManager, loggerInstance)
		return
	}

	runContinuousPublisher(ctx, queueManager, loggerInstance)
}

// publishTestEvents publishes some test events and exits
func publishTestEvents(ctx context.Context, queueManager *queue.Manager, loggerInstance *logger.Logger) {
	loggerInstance.Info(ctx, "Publishing test events in CLI mode", nil)

	eventTypes := []string{
		"user.created",
		"user.updated",
		"order.created",
		"payment.processed",
		"notification.sent",
	}

	for i, eventType := range eventTypes {
		// Create event data
		data := map[string]interface{}{
			"user_id":    fmt.Sprintf("user-%d", i+1),
			"email":      fmt.Sprintf("user%d@example.com", i+1),
			"timestamp":  time.Now().Unix(),
			"event_data": fmt.Sprintf("Test data for %s", eventType),
		}

		// Create event message
		message := queue.NewEventMessage(eventType, data, "gohopper-cli")

		// Add some metadata
		message.AddHeader("test_mode", true)
		message.AddTag("test")
		message.AddTag("cli")
		message.SetPriority(i + 1)

		// Publish event
		routingKey := fmt.Sprintf("event.%s", eventType)
		if err := queueManager.PublishEventMessage(ctx, message, routingKey); err != nil {
			loggerInstance.Error(ctx, "Failed to publish test event", err, logger.Fields{
				"event_type":  eventType,
				"routing_key": routingKey,
			})
			continue
		}

		loggerInstance.Info(ctx, "Test event published", logger.Fields{
			"message_id":  message.ID,
			"event_type":  eventType,
			"routing_key": routingKey,
			"trace_id":    message.TraceID,
		})

		time.Sleep(500 * time.Millisecond)
	}

	loggerInstance.Info(ctx, "Test events published successfully", nil)
}

// runContinuousPublisher runs the publisher in continuous mode
func runContinuousPublisher(ctx context.Context, queueManager *queue.Manager, loggerInstance *logger.Logger) {
	// Configure graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	stopChan := make(chan bool)

	go publishEvents(ctx, queueManager, loggerInstance, stopChan)

	<-sigChan
	loggerInstance.Info(ctx, "Shutting down publisher", nil)
	stopChan <- true
}

// publishEvents publishes events periodically
func publishEvents(ctx context.Context, queueManager *queue.Manager, loggerInstance *logger.Logger, stopChan chan bool) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	eventCounter := 1

	for {
		select {
		case <-ticker.C:
			// Create event data
			data := map[string]interface{}{
				"user_id":    fmt.Sprintf("user-%d", eventCounter),
				"email":      fmt.Sprintf("user%d@example.com", eventCounter),
				"name":       fmt.Sprintf("User %d", eventCounter),
				"timestamp":  time.Now().Unix(),
				"event_data": fmt.Sprintf("User data for event %d", eventCounter),
			}

			// Create event message
			message := queue.NewEventMessage("user.created", data, "gohopper-publisher")

			// Add metadata
			message.AddHeader("continuous_mode", true)
			message.AddTag("continuous")
			message.AddTag("user")
			message.SetPriority(1)

			// Publish event
			routingKey := "event.user.created"
			if err := queueManager.PublishEventMessage(ctx, message, routingKey); err != nil {
				loggerInstance.Error(ctx, "Failed to publish event", err, logger.Fields{
					"event_counter": eventCounter,
					"routing_key":   routingKey,
				})
				continue
			}

			loggerInstance.Info(ctx, "Event published", logger.Fields{
				"message_id":    message.ID,
				"event_counter": eventCounter,
				"routing_key":   routingKey,
				"trace_id":      message.TraceID,
			})
			eventCounter++

		case <-stopChan:
			loggerInstance.Info(ctx, "Stopping event publication", nil)
			return
		}
	}
}
