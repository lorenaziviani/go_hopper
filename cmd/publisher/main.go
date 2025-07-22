package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gohopper/internal/queue"

	"github.com/joho/godotenv"
)

// Event represents an event to be published
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Data      string    `json:"data"`
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
}

func main() {
	var cliMode bool
	flag.BoolVar(&cliMode, "cli", false, "Run in CLI mode to publish test events")
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found")
	}

	log.Println("Gohopper Publisher starting...")

	// Create queue manager
	queueManager := queue.NewManager()

	// Connect to RabbitMQ
	if err := queueManager.Connect(); err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer queueManager.Close()

	// Configure queues and exchanges
	if err := queueManager.SetupQueues(); err != nil {
		log.Fatalf("Failed to setup queues: %v", err)
	}

	log.Println("Publisher configured successfully!")

	if cliMode {
		publishTestEvents(queueManager)
		return
	}

	runContinuousPublisher(queueManager)
}

// publishTestEvents publishes some test events and exits
func publishTestEvents(queueManager *queue.Manager) {
	log.Println("Publishing test events in CLI mode...")

	eventTypes := []string{
		"user.created",
		"user.updated",
		"order.created",
		"payment.processed",
		"notification.sent",
	}

	for i, eventType := range eventTypes {
		event := Event{
			ID:        fmt.Sprintf("test-event-%d", i+1),
			Type:      eventType,
			Data:      fmt.Sprintf("Test data for %s", eventType),
			Timestamp: time.Now(),
			Source:    "gohopper-cli",
		}

		eventJSON, err := json.Marshal(event)
		if err != nil {
			log.Printf("Failed to marshal event: %v", err)
			continue
		}

		// Publish event
		routingKey := fmt.Sprintf("event.%s", eventType)
		if err := queueManager.PublishMessage(routingKey, eventJSON); err != nil {
			log.Printf("Failed to publish event: %v", err)
			continue
		}

		log.Printf("Published test event: %s (routing key: %s)", event.ID, routingKey)

		time.Sleep(500 * time.Millisecond)
	}

	log.Println("Test events published successfully!")
}

// runContinuousPublisher runs the publisher in continuous mode
func runContinuousPublisher(queueManager *queue.Manager) {
	// Configure graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	stopChan := make(chan bool)

	go publishEvents(queueManager, stopChan)

	<-sigChan
	log.Println("Shutting down publisher...")
	stopChan <- true
}

// publishEvents publishes events periodically
func publishEvents(queueManager *queue.Manager, stopChan chan bool) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	eventCounter := 1

	for {
		select {
		case <-ticker.C:
			event := Event{
				ID:        fmt.Sprintf("event-%d", eventCounter),
				Type:      "user.created",
				Data:      fmt.Sprintf("User data for event %d", eventCounter),
				Timestamp: time.Now(),
				Source:    "gohopper-publisher",
			}

			eventJSON, err := json.Marshal(event)
			if err != nil {
				log.Printf("Failed to marshal event: %v", err)
				continue
			}

			// publish event
			routingKey := "event.user.created"
			if err := queueManager.PublishMessage(routingKey, eventJSON); err != nil {
				log.Printf("Failed to publish event: %v", err)
				continue
			}

			log.Printf("Published event: %s (routing key: %s)", event.ID, routingKey)
			eventCounter++

		case <-stopChan:
			log.Println("Stopping event publication...")
			return
		}
	}
}
