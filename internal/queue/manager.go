package queue

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/streadway/amqp"
)

type Manager struct {
	host     string
	port     string
	user     string
	password string
	vhost    string
	conn     *amqp.Connection
	channel  *amqp.Channel
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

	// Build connection URL
	url := fmt.Sprintf("amqp://%s:%s@%s:%s%s",
		m.user, m.password, m.host, m.port, m.vhost)

	// Try to connect with retry
	var err error
	for i := 0; i < 5; i++ {
		m.conn, err = amqp.Dial(url)
		if err == nil {
			break
		}
		log.Printf("Connection attempt %d failed: %v", i+1, err)
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	m.channel, err = m.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}

	log.Println("Successfully connected to RabbitMQ")
	return nil
}

// SetupQueues configures the queues and exchanges
func (m *Manager) SetupQueues() error {
	log.Println("Configuring queues and exchanges...")

	// Configure QoS
	err := m.channel.Qos(
		1,
		0,
		false,
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Declare main exchange
	exchangeName := getEnv("EXCHANGE_NAME", "events_exchange")
	err = m.channel.ExchangeDeclare(
		exchangeName, // name
		"topic",      // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare main queue
	queueName := getEnv("QUEUE_NAME", "events")
	_, err = m.channel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Declare DLQ exchange
	dlqExchangeName := getEnv("DLQ_EXCHANGE", "events_dlq_exchange")
	err = m.channel.ExchangeDeclare(
		dlqExchangeName, // name
		"topic",         // type
		true,            // durable
		false,           // auto-deleted
		false,           // internal
		false,           // no-wait
		nil,             // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ exchange: %w", err)
	}

	// Declare DLQ
	dlqName := getEnv("DLQ_NAME", "events_dlq")
	_, err = m.channel.QueueDeclare(
		dlqName, // name
		true,    // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare DLQ: %w", err)
	}

	// Bind main queue with the exchange
	routingKey := getEnv("ROUTING_KEY", "event.*")
	err = m.channel.QueueBind(
		queueName,    // queue name
		routingKey,   // routing key
		exchangeName, // exchange
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	// Bind DLQ with the DLQ exchange
	err = m.channel.QueueBind(
		dlqName,         // queue name
		"#",             // routing key (all messages)
		dlqExchangeName, // exchange
		false,           // no-wait
		nil,             // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to bind DLQ: %w", err)
	}

	log.Println("Queues and exchanges configured successfully")
	return nil
}

// PublishMessage publishes a message to the exchange
func (m *Manager) PublishMessage(routingKey string, body []byte) error {
	if m.channel == nil {
		return fmt.Errorf("channel not initialized")
	}

	exchangeName := getEnv("EXCHANGE_NAME", "events_exchange")

	err := m.channel.Publish(
		exchangeName, // exchange
		routingKey,   // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Close closes the connection to RabbitMQ
func (m *Manager) Close() error {
	if m.channel != nil {
		if err := m.channel.Close(); err != nil {
			log.Printf("Error closing channel: %v", err)
		}
	}

	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			log.Printf("Error closing connection: %v", err)
		}
	}

	log.Println("RabbitMQ connection closed")
	return nil
}

// getEnv gets the environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
