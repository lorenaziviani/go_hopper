package queue

import (
	"context"
	"fmt"
	"os"
	"time"

	"gohopper/internal/logger"

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
	logger   *logger.Logger
	encoder  MessageEncoder
}

// NewManager creates a new instance of the queue manager
func NewManager() *Manager {
	return &Manager{
		host:     getEnv("RABBITMQ_HOST", "localhost"),
		port:     getEnv("RABBITMQ_PORT", "5672"),
		user:     getEnv("RABBITMQ_USER", "guest"),
		password: getEnv("RABBITMQ_PASSWORD", "guest"),
		vhost:    getEnv("RABBITMQ_VHOST", "/"),
		logger:   logger.NewLogger(),
		encoder:  &JSONEncoder{},
	}
}

// Connect establishes a connection to the RabbitMQ
func (m *Manager) Connect() error {
	ctx := logger.CreateTraceContext()
	m.logger.Info(ctx, "Connecting to RabbitMQ", logger.Fields{
		"host": m.host,
		"port": m.port,
		"user": m.user,
	})

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
		m.logger.Warn(ctx, fmt.Sprintf("Connection attempt %d failed", i+1), logger.Fields{
			"attempt": i + 1,
			"error":   err.Error(),
		})
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err != nil {
		m.logger.Error(ctx, "Failed to connect to RabbitMQ", err, logger.Fields{
			"max_attempts": 5,
		})
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	m.channel, err = m.conn.Channel()
	if err != nil {
		m.logger.Error(ctx, "Failed to open channel", err, nil)
		return fmt.Errorf("failed to open channel: %w", err)
	}

	m.logger.Info(ctx, "Successfully connected to RabbitMQ", logger.Fields{
		"connection_id": m.conn.LocalAddr().String(),
	})
	return nil
}

// SetupQueues configures the queues and exchanges
func (m *Manager) SetupQueues() error {
	ctx := logger.CreateTraceContext()
	m.logger.Info(ctx, "Configuring queues and exchanges", nil)

	// Configure QoS
	err := m.channel.Qos(
		1,
		0,
		false,
	)
	if err != nil {
		m.logger.Error(ctx, "Failed to set QoS", err, nil)
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
		m.logger.Error(ctx, "Failed to declare exchange", err, logger.Fields{
			"exchange_name": exchangeName,
		})
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
		m.logger.Error(ctx, "Failed to declare queue", err, logger.Fields{
			"queue_name": queueName,
		})
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
		m.logger.Error(ctx, "Failed to declare DLQ exchange", err, logger.Fields{
			"dlq_exchange_name": dlqExchangeName,
		})
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
		m.logger.Error(ctx, "Failed to declare DLQ", err, logger.Fields{
			"dlq_name": dlqName,
		})
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
		m.logger.Error(ctx, "Failed to bind queue", err, logger.Fields{
			"queue_name":    queueName,
			"routing_key":   routingKey,
			"exchange_name": exchangeName,
		})
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
		m.logger.Error(ctx, "Failed to bind DLQ", err, logger.Fields{
			"dlq_name":          dlqName,
			"dlq_exchange_name": dlqExchangeName,
		})
		return fmt.Errorf("failed to bind DLQ: %w", err)
	}

	m.logger.Info(ctx, "Queues and exchanges configured successfully", logger.Fields{
		"main_queue":    queueName,
		"main_exchange": exchangeName,
		"dlq_name":      dlqName,
		"dlq_exchange":  dlqExchangeName,
		"routing_key":   routingKey,
	})
	return nil
}

// PublishMessage publishes a message to the exchange
func (m *Manager) PublishMessage(routingKey string, body []byte) error {
	if m.channel == nil {
		return fmt.Errorf("channel not initialized")
	}

	ctx := logger.CreateTraceContext()
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
		m.logger.Error(ctx, "Failed to publish message", err, logger.Fields{
			"exchange_name": exchangeName,
			"routing_key":   routingKey,
			"body_size":     len(body),
		})
		return fmt.Errorf("failed to publish message: %w", err)
	}

	m.logger.Info(ctx, "Message published successfully", logger.Fields{
		"exchange_name": exchangeName,
		"routing_key":   routingKey,
		"body_size":     len(body),
	})

	return nil
}

// PublishEventMessage publishes an EventMessage using the encoder
func (m *Manager) PublishEventMessage(ctx context.Context, message *EventMessage, routingKey string) error {
	if message == nil {
		return fmt.Errorf("message cannot be nil")
	}

	body, err := m.encoder.Encode(message)
	if err != nil {
		m.logger.Error(ctx, "Failed to encode message", err, logger.Fields{
			"message_id":   message.ID,
			"message_type": message.Type,
		})
		return fmt.Errorf("failed to encode message: %w", err)
	}

	err = m.PublishMessage(routingKey, body)
	if err != nil {
		m.logger.Error(ctx, "Failed to publish event message", err, logger.Fields{
			"message_id":   message.ID,
			"message_type": message.Type,
			"routing_key":  routingKey,
		})
		return err
	}

	m.logger.LogMessageReceived(ctx, message.ID, message.Type, message.Source, logger.Fields{
		"routing_key": routingKey,
		"trace_id":    message.TraceID,
	})

	return nil
}

// ConsumeMessages starts consuming messages from the queue
func (m *Manager) ConsumeMessages(ctx context.Context, consumerTag string, handler func(*EventMessage, amqp.Delivery) error) error {
	if m.channel == nil {
		return fmt.Errorf("channel not initialized")
	}

	queueName := getEnv("QUEUE_NAME", "events")

	msgs, err := m.channel.Consume(
		queueName,   // queue
		consumerTag, // consumer
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		m.logger.Error(ctx, "Failed to start consuming", err, logger.Fields{
			"queue_name":   queueName,
			"consumer_tag": consumerTag,
		})
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	m.logger.Info(ctx, "Started consuming messages", logger.Fields{
		"queue_name":   queueName,
		"consumer_tag": consumerTag,
	})

	go func() {
		for {
			select {
			case msg := <-msgs:
				m.processMessage(ctx, msg, handler)
			case <-ctx.Done():
				m.logger.Info(ctx, "Stopping message consumption", logger.Fields{
					"consumer_tag": consumerTag,
					"reason":       "context cancelled",
				})
				return
			}
		}
	}()

	return nil
}

// ConsumeDLQMessages starts consuming messages from the DLQ
func (m *Manager) ConsumeDLQMessages(ctx context.Context, consumerTag string, handler func(*EventMessage, amqp.Delivery) error) error {
	if m.channel == nil {
		return fmt.Errorf("channel not initialized")
	}

	dlqName := getEnv("DLQ_NAME", "events_dlq")

	msgs, err := m.channel.Consume(
		dlqName,     // queue
		consumerTag, // consumer
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		m.logger.Error(ctx, "Failed to start consuming DLQ", err, logger.Fields{
			"dlq_name":     dlqName,
			"consumer_tag": consumerTag,
		})
		return fmt.Errorf("failed to start consuming DLQ: %w", err)
	}

	m.logger.Info(ctx, "Started consuming DLQ messages", logger.Fields{
		"dlq_name":     dlqName,
		"consumer_tag": consumerTag,
	})

	// Process messages in a separate goroutine
	go func() {
		for {
			select {
			case msg := <-msgs:
				m.processDLQMessage(ctx, msg, handler)
			case <-ctx.Done():
				m.logger.Info(ctx, "Stopping DLQ message consumption", logger.Fields{
					"consumer_tag": consumerTag,
					"reason":       "context cancelled",
				})
				return
			}
		}
	}()

	return nil
}

// processMessage processes a single message
func (m *Manager) processMessage(ctx context.Context, delivery amqp.Delivery, handler func(*EventMessage, amqp.Delivery) error) {
	message, err := m.encoder.Decode(delivery.Body)
	if err != nil {
		m.logger.Error(ctx, "Failed to decode message", err, logger.Fields{
			"delivery_tag": delivery.DeliveryTag,
			"body_size":    len(delivery.Body),
		})

		if err := delivery.Reject(false); err != nil {
			m.logger.Error(ctx, "Failed to reject undecodable message", err, nil)
		}
		return
	}

	messageCtx := context.WithValue(ctx, "trace_id", message.TraceID)

	m.logger.LogMessageReceived(messageCtx, message.ID, message.Type, message.Source, logger.Fields{
		"delivery_tag": delivery.DeliveryTag,
		"routing_key":  delivery.RoutingKey,
		"exchange":     delivery.Exchange,
	})

	if err := handler(message, delivery); err != nil {
		m.logger.Error(messageCtx, "Message processing failed", err, logger.Fields{
			"message_id":   message.ID,
			"message_type": message.Type,
			"delivery_tag": delivery.DeliveryTag,
		})
	} else {
		m.logger.Info(messageCtx, "Message processed successfully", logger.Fields{
			"message_id":   message.ID,
			"message_type": message.Type,
			"delivery_tag": delivery.DeliveryTag,
		})
	}
}

// processDLQMessage processes a single DLQ message
func (m *Manager) processDLQMessage(ctx context.Context, delivery amqp.Delivery, handler func(*EventMessage, amqp.Delivery) error) {
	// Decode message
	message, err := m.encoder.Decode(delivery.Body)
	if err != nil {
		m.logger.Error(ctx, "Failed to decode DLQ message", err, logger.Fields{
			"delivery_tag": delivery.DeliveryTag,
			"body_size":    len(delivery.Body),
		})

		// Reject the message
		if err := delivery.Reject(false); err != nil {
			m.logger.Error(ctx, "Failed to reject undecodable DLQ message", err, nil)
		}
		return
	}

	// Create context with message trace ID
	messageCtx := context.WithValue(ctx, "trace_id", message.TraceID)

	m.logger.LogMessageReceived(messageCtx, message.ID, message.Type, message.Source, logger.Fields{
		"delivery_tag": delivery.DeliveryTag,
		"routing_key":  delivery.RoutingKey,
		"exchange":     delivery.Exchange,
		"dlq_reason":   message.Metadata.DLQReason,
	})

	// Process the message
	if err := handler(message, delivery); err != nil {
		m.logger.Error(messageCtx, "DLQ message processing failed", err, logger.Fields{
			"message_id":   message.ID,
			"message_type": message.Type,
			"delivery_tag": delivery.DeliveryTag,
		})
	} else {
		m.logger.Info(messageCtx, "DLQ message processed successfully", logger.Fields{
			"message_id":   message.ID,
			"message_type": message.Type,
			"delivery_tag": delivery.DeliveryTag,
		})
	}
}

// GetQueueStats returns queue statistics
func (m *Manager) GetQueueStats() (map[string]interface{}, error) {
	if m.channel == nil {
		return nil, fmt.Errorf("channel not initialized")
	}

	queueName := getEnv("QUEUE_NAME", "events")
	dlqName := getEnv("DLQ_NAME", "events_dlq")

	mainQueue, err := m.channel.QueueInspect(queueName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect main queue: %w", err)
	}

	dlq, err := m.channel.QueueInspect(dlqName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect DLQ: %w", err)
	}

	return map[string]interface{}{
		"main_queue": map[string]interface{}{
			"name":      mainQueue.Name,
			"messages":  mainQueue.Messages,
			"consumers": mainQueue.Consumers,
		},
		"dlq": map[string]interface{}{
			"name":      dlq.Name,
			"messages":  dlq.Messages,
			"consumers": dlq.Consumers,
		},
	}, nil
}

// Close closes the connection to RabbitMQ
func (m *Manager) Close() error {
	ctx := logger.CreateTraceContext()

	if m.channel != nil {
		if err := m.channel.Close(); err != nil {
			m.logger.Error(ctx, "Error closing channel", err, nil)
		}
	}

	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			m.logger.Error(ctx, "Error closing connection", err, nil)
		}
	}

	m.logger.Info(ctx, "RabbitMQ connection closed", nil)
	return nil
}

// getEnv gets the environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
