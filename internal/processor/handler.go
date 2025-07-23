package processor

import (
	"context"
	"fmt"
	"time"

	"gohopper/internal/logger"
	"gohopper/internal/queue"
)

// DefaultMessageHandler implements MessageHandler interface
type DefaultMessageHandler struct {
	logger *logger.Logger
}

// NewDefaultMessageHandler creates a new default message handler
func NewDefaultMessageHandler() *DefaultMessageHandler {
	return &DefaultMessageHandler{
		logger: logger.NewLogger(),
	}
}

// ProcessMessage processes a message based on its type
func (h *DefaultMessageHandler) ProcessMessage(ctx context.Context, message *queue.EventMessage) error {
	h.logger.Info(ctx, "Processing message", logger.Fields{
		"message_id":   message.ID,
		"message_type": message.Type,
		"source":       message.Source,
		"priority":     message.Metadata.Priority,
	})

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	processingTime := h.getProcessingTime(message.Type)

	select {
	case <-time.After(processingTime):
	case <-ctx.Done():
		return fmt.Errorf("processing timeout: %w", ctx.Err())
	}

	switch message.Type {
	case "user.created":
		return h.processUserCreated(ctx, message)
	case "user.updated":
		return h.processUserUpdated(ctx, message)
	case "order.created":
		return h.processOrderCreated(ctx, message)
	case "payment.processed":
		return h.processPaymentProcessed(ctx, message)
	case "notification.sent":
		return h.processNotificationSent(ctx, message)
	default:
		return h.processUnknownMessage(ctx, message)
	}
}

// processUserCreated processes user.created events
func (h *DefaultMessageHandler) processUserCreated(ctx context.Context, message *queue.EventMessage) error {
	h.logger.Info(ctx, "Processing user.created event", logger.Fields{
		"message_id": message.ID,
		"user_id":    h.extractUserID(message),
	})

	select {
	case <-ctx.Done():
		return fmt.Errorf("user.created processing cancelled: %w", ctx.Err())
	default:
	}

	if h.shouldSimulateError(message) {
		return fmt.Errorf("simulated error processing user.created event")
	}

	if h.shouldSimulateTimeout(message) {
		select {
		case <-time.After(60 * time.Second):
			return fmt.Errorf("user.created processing timeout")
		case <-ctx.Done():
			return fmt.Errorf("user.created processing cancelled: %w", ctx.Err())
		}
	}

	h.logger.Info(ctx, "User created successfully", logger.Fields{
		"message_id": message.ID,
		"user_id":    h.extractUserID(message),
	})

	return nil
}

// processUserUpdated processes user.updated events
func (h *DefaultMessageHandler) processUserUpdated(ctx context.Context, message *queue.EventMessage) error {
	h.logger.Info(ctx, "Processing user.updated event", logger.Fields{
		"message_id": message.ID,
		"user_id":    h.extractUserID(message),
	})

	select {
	case <-ctx.Done():
		return fmt.Errorf("user.updated processing cancelled: %w", ctx.Err())
	default:
	}

	if h.shouldSimulateError(message) {
		return fmt.Errorf("simulated error processing user.updated event")
	}

	h.logger.Info(ctx, "User updated successfully", logger.Fields{
		"message_id": message.ID,
		"user_id":    h.extractUserID(message),
	})

	return nil
}

// processOrderCreated processes order.created events
func (h *DefaultMessageHandler) processOrderCreated(ctx context.Context, message *queue.EventMessage) error {
	h.logger.Info(ctx, "Processing order.created event", logger.Fields{
		"message_id": message.ID,
	})

	select {
	case <-ctx.Done():
		return fmt.Errorf("order.created processing cancelled: %w", ctx.Err())
	default:
	}

	if h.shouldSimulateError(message) {
		return fmt.Errorf("simulated error processing order.created event")
	}

	if h.shouldSimulateTimeout(message) {
		select {
		case <-time.After(45 * time.Second):
			return fmt.Errorf("order.created processing timeout")
		case <-ctx.Done():
			return fmt.Errorf("order.created processing cancelled: %w", ctx.Err())
		}
	}

	h.logger.Info(ctx, "Order created successfully", logger.Fields{
		"message_id": message.ID,
	})

	return nil
}

// processPaymentProcessed processes payment.processed events
func (h *DefaultMessageHandler) processPaymentProcessed(ctx context.Context, message *queue.EventMessage) error {
	h.logger.Info(ctx, "Processing payment.processed event", logger.Fields{
		"message_id": message.ID,
	})

	select {
	case <-ctx.Done():
		return fmt.Errorf("payment.processed processing cancelled: %w", ctx.Err())
	default:
	}

	if h.shouldSimulateError(message) {
		return fmt.Errorf("simulated error processing payment.processed event")
	}

	h.logger.Info(ctx, "Payment processed successfully", logger.Fields{
		"message_id": message.ID,
	})

	return nil
}

// processNotificationSent processes notification.sent events
func (h *DefaultMessageHandler) processNotificationSent(ctx context.Context, message *queue.EventMessage) error {
	h.logger.Info(ctx, "Processing notification.sent event", logger.Fields{
		"message_id": message.ID,
	})

	select {
	case <-ctx.Done():
		return fmt.Errorf("notification.sent processing cancelled: %w", ctx.Err())
	default:
	}

	if h.shouldSimulateError(message) {
		return fmt.Errorf("simulated error processing notification.sent event")
	}

	h.logger.Info(ctx, "Notification sent successfully", logger.Fields{
		"message_id": message.ID,
	})

	return nil
}

// processUnknownMessage processes unknown message types
func (h *DefaultMessageHandler) processUnknownMessage(ctx context.Context, message *queue.EventMessage) error {
	h.logger.Warn(ctx, "Processing unknown message type", logger.Fields{
		"message_id":   message.ID,
		"message_type": message.Type,
	})

	return nil
}

// getProcessingTime returns processing time based on message type
func (h *DefaultMessageHandler) getProcessingTime(messageType string) time.Duration {
	switch messageType {
	case "user.created":
		return 200 * time.Millisecond
	case "user.updated":
		return 150 * time.Millisecond
	case "order.created":
		return 300 * time.Millisecond
	case "payment.processed":
		return 500 * time.Millisecond
	case "notification.sent":
		return 100 * time.Millisecond
	default:
		return 50 * time.Millisecond
	}
}

// extractUserID extracts user ID from message data
func (h *DefaultMessageHandler) extractUserID(message *queue.EventMessage) string {
	if userID, ok := message.Data["user_id"].(string); ok {
		return userID
	}
	return "unknown"
}

// shouldSimulateError determines if we should simulate an error for testing
func (h *DefaultMessageHandler) shouldSimulateError(message *queue.EventMessage) bool {
	if message.Metadata.RetryCount == 0 {
		lastChar := message.ID[len(message.ID)-1]
		return lastChar == '0' || lastChar == '5'
	}

	return false
}

// shouldSimulateTimeout determines if we should simulate a timeout for testing
func (h *DefaultMessageHandler) shouldSimulateTimeout(message *queue.EventMessage) bool {
	if message.Metadata.RetryCount == 0 {
		lastChar := message.ID[len(message.ID)-1]
		return lastChar == '2' || lastChar == '7'
	}

	return false
}
