package queue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EventMessage represents the standard message structure for the system
type EventMessage struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Data          map[string]interface{} `json:"data"`
	Metadata      MessageMetadata        `json:"metadata"`
	Timestamp     time.Time              `json:"timestamp"`
	Source        string                 `json:"source"`
	Version       string                 `json:"version"`
	TraceID       string                 `json:"trace_id"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
}

// MessageMetadata contains additional message metadata
type MessageMetadata struct {
	Priority   int                    `json:"priority"`
	RetryCount int                    `json:"retry_count"`
	TTL        *time.Duration         `json:"ttl,omitempty"`
	Headers    map[string]interface{} `json:"headers,omitempty"`
	Tags       []string               `json:"tags,omitempty"`
}

// MessageEncoder interface for message encoding
type MessageEncoder interface {
	Encode(message *EventMessage) ([]byte, error)
	Decode(data []byte) (*EventMessage, error)
}

// JSONEncoder implements JSON encoding
type JSONEncoder struct{}

// NewEventMessage creates a new event message
func NewEventMessage(eventType string, data map[string]interface{}, source string) *EventMessage {
	return &EventMessage{
		ID:   uuid.New().String(),
		Type: eventType,
		Data: data,
		Metadata: MessageMetadata{
			Priority:   1,
			RetryCount: 0,
			Headers:    make(map[string]interface{}),
			Tags:       []string{},
		},
		Timestamp:     time.Now().UTC(),
		Source:        source,
		Version:       "1.0.0",
		TraceID:       uuid.New().String(),
		CorrelationID: "",
	}
}

// Encode encodes the message to JSON
func (e *JSONEncoder) Encode(message *EventMessage) ([]byte, error) {
	if message == nil {
		return nil, fmt.Errorf("message cannot be nil")
	}

	return json.Marshal(message)
}

// Decode decodes JSON to message
func (e *JSONEncoder) Decode(data []byte) (*EventMessage, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data cannot be empty")
	}

	var message EventMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, fmt.Errorf("failed to decode message: %w", err)
	}

	return &message, nil
}

// SetCorrelationID defines the correlation ID of the message
func (m *EventMessage) SetCorrelationID(correlationID string) {
	m.CorrelationID = correlationID
}

// SetPriority defines the priority of the message
func (m *EventMessage) SetPriority(priority int) {
	m.Metadata.Priority = priority
}

// AddHeader adds a header to the message
func (m *EventMessage) AddHeader(key string, value interface{}) {
	if m.Metadata.Headers == nil {
		m.Metadata.Headers = make(map[string]interface{})
	}
	m.Metadata.Headers[key] = value
}

// AddTag adds a tag to the message
func (m *EventMessage) AddTag(tag string) {
	m.Metadata.Tags = append(m.Metadata.Tags, tag)
}

// SetTTL defines the TTL of the message
func (m *EventMessage) SetTTL(ttl time.Duration) {
	m.Metadata.TTL = &ttl
}

// IncrementRetryCount increments the retry count
func (m *EventMessage) IncrementRetryCount() {
	m.Metadata.RetryCount++
}

// IsRetryable checks if the message can be retried
func (m *EventMessage) IsRetryable(maxRetries int) bool {
	return m.Metadata.RetryCount < maxRetries
}

// GetAge returns the age of the message
func (m *EventMessage) GetAge() time.Duration {
	return time.Since(m.Timestamp)
}

// String returns the string representation of the message
func (m *EventMessage) String() string {
	return fmt.Sprintf("EventMessage{ID: %s, Type: %s, Source: %s, TraceID: %s}",
		m.ID, m.Type, m.Source, m.TraceID)
}
