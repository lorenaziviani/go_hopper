package logger

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type contextKey string

const (
	TraceIDKey contextKey = "trace_id"
)

// Logger wrapper for logrus with trace ID support
type Logger struct {
	logger *logrus.Logger
}

// Fields type for additional log fields
type Fields map[string]interface{}

// NewLogger creates a new configured logger
func NewLogger() *Logger {
	logger := logrus.New()

	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
		},
	})

	level := getLogLevel()
	logger.SetLevel(level)

	logger.SetOutput(os.Stdout)

	return &Logger{logger: logger}
}

// getLogLevel gets the log level from the environment variables
func getLogLevel() logrus.Level {
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "fatal":
		return logrus.FatalLevel
	case "panic":
		return logrus.PanicLevel
	default:
		return logrus.InfoLevel
	}
}

// WithTraceID adds trace ID to the context
func WithTraceID(ctx context.Context) context.Context {
	traceID := uuid.New().String()
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// GetTraceID gets the trace ID from the context
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// WithFields adds fields to the logger
func (l *Logger) WithFields(fields Fields) *logrus.Entry {
	return l.logger.WithFields(logrus.Fields(fields))
}

// WithContext adds context (including trace ID) to the logger
func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	fields := Fields{}

	if traceID := GetTraceID(ctx); traceID != "" {
		fields["trace_id"] = traceID
	}

	return l.logger.WithFields(logrus.Fields(fields))
}

// WithMessage adds message information to the logger
func (l *Logger) WithMessage(messageID, messageType, source string) *logrus.Entry {
	return l.logger.WithFields(logrus.Fields{
		"message_id":   messageID,
		"message_type": messageType,
		"source":       source,
	})
}

// Debug logs a debug message
func (l *Logger) Debug(ctx context.Context, message string, fields Fields) {
	l.WithContext(ctx).WithFields(logrus.Fields(fields)).Debug(message)
}

// Info logs an info message
func (l *Logger) Info(ctx context.Context, message string, fields Fields) {
	l.WithContext(ctx).WithFields(logrus.Fields(fields)).Info(message)
}

// Warn logs a warning message
func (l *Logger) Warn(ctx context.Context, message string, fields Fields) {
	l.WithContext(ctx).WithFields(logrus.Fields(fields)).Warn(message)
}

// Error logs an error message
func (l *Logger) Error(ctx context.Context, message string, err error, fields Fields) {
	if fields == nil {
		fields = Fields{}
	}
	fields["error"] = err.Error()
	l.WithContext(ctx).WithFields(logrus.Fields(fields)).Error(message)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(ctx context.Context, message string, fields Fields) {
	l.WithContext(ctx).WithFields(logrus.Fields(fields)).Fatal(message)
}

// LogMessage logs information about a message
func (l *Logger) LogMessage(ctx context.Context, level logrus.Level, message string, messageID, messageType, source string, fields Fields) {
	if fields == nil {
		fields = Fields{}
	}

	fields["message_id"] = messageID
	fields["message_type"] = messageType
	fields["source"] = source

	entry := l.WithContext(ctx).WithFields(logrus.Fields(fields))

	switch level {
	case logrus.DebugLevel:
		entry.Debug(message)
	case logrus.InfoLevel:
		entry.Info(message)
	case logrus.WarnLevel:
		entry.Warn(message)
	case logrus.ErrorLevel:
		entry.Error(message)
	case logrus.FatalLevel:
		entry.Fatal(message)
	case logrus.PanicLevel:
		entry.Panic(message)
	default:
		entry.Info(message)
	}
}

// LogMessageReceived logs when a message is received
func (l *Logger) LogMessageReceived(ctx context.Context, messageID, messageType, source string, fields Fields) {
	l.LogMessage(ctx, logrus.InfoLevel, "Message received", messageID, messageType, source, fields)
}

// LogMessageProcessed logs when a message is processed successfully
func (l *Logger) LogMessageProcessed(ctx context.Context, messageID, messageType, source string, processingTime time.Duration, fields Fields) {
	if fields == nil {
		fields = Fields{}
	}
	fields["processing_time_ms"] = processingTime.Milliseconds()
	l.LogMessage(ctx, logrus.InfoLevel, "Message processed successfully", messageID, messageType, source, fields)
}

// LogMessageFailed logs when a message processing fails
func (l *Logger) LogMessageFailed(ctx context.Context, messageID, messageType, source string, err error, retryCount int, fields Fields) {
	if fields == nil {
		fields = Fields{}
	}
	fields["error"] = err.Error()
	fields["retry_count"] = retryCount
	l.LogMessage(ctx, logrus.ErrorLevel, "Message processing failed", messageID, messageType, source, fields)
}

// LogMessageRetry logs when a message is being retried
func (l *Logger) LogMessageRetry(ctx context.Context, messageID, messageType, source string, retryCount int, delay time.Duration, fields Fields) {
	if fields == nil {
		fields = Fields{}
	}
	fields["retry_count"] = retryCount
	fields["retry_delay_ms"] = delay.Milliseconds()
	l.LogMessage(ctx, logrus.WarnLevel, "Message retry scheduled", messageID, messageType, source, fields)
}

// LogMessageDLQ logs when a message is sent to DLQ
func (l *Logger) LogMessageDLQ(ctx context.Context, messageID, messageType, source string, reason string, fields Fields) {
	if fields == nil {
		fields = Fields{}
	}
	fields["dlq_reason"] = reason
	l.LogMessage(ctx, logrus.ErrorLevel, "Message sent to DLQ", messageID, messageType, source, fields)
}

// FormatTraceID formata o trace ID para exibição
func FormatTraceID(traceID string) string {
	if len(traceID) > 8 {
		return traceID[:8] + "..."
	}
	return traceID
}

// CreateTraceContext cria um contexto com trace ID
func CreateTraceContext() context.Context {
	return WithTraceID(context.Background())
}

// LogWithTraceID logs com trace ID específico
func (l *Logger) LogWithTraceID(traceID, level, message string, fields Fields) {
	if fields == nil {
		fields = Fields{}
	}
	fields["trace_id"] = traceID

	ctx := context.WithValue(context.Background(), TraceIDKey, traceID)

	switch level {
	case "debug":
		l.Debug(ctx, message, fields)
	case "info":
		l.Info(ctx, message, fields)
	case "warn":
		l.Warn(ctx, message, fields)
	case "error":
		l.Error(ctx, message, fmt.Errorf("error"), fields)
	default:
		l.Info(ctx, message, fields)
	}
}
