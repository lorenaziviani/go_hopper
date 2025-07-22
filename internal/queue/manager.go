package queue

import (
	"log"
	"os"
)

type Manager struct {
	host     string
	port     string
	user     string
	password string
	vhost    string
}

// NewManager creates a new instance of the queue manager
func NewManager() *Manager {
	return &Manager{
		host:     getEnv("RABBITMQ_HOST", "localhost"),
		port:     getEnv("RABBITMQ_PORT", "5672"),
		user:     getEnv("RABBITMQ_USER", "guest"),
		password: getEnv("RABBITMQ_PASSWORD", "guest"),
		vhost:    getEnv("RABBITMQ_VHOST", "/"),
	}
}

// Connect establishes a connection to the RabbitMQ
func (m *Manager) Connect() error {
	log.Println("Connecting to RabbitMQ...")

	// TODO: Implement connection to RabbitMQ
	// - Use streadway/amqp
	// - Configure connection with retry
	// - Configure channel

	return nil
}

// SetupQueues configures the queues and exchanges
func (m *Manager) SetupQueues() error {
	log.Println("Configuring queues and exchanges...")

	// TODO: Implement queue and exchange configuration
	// - Create main exchange
	// - Create main queue
	// - Create DLQ and exchange
	// - Configure bindings

	return nil
}

// getEnv gets the environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
