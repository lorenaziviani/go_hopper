package configs

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	RabbitMQ RabbitMQConfig
	App      AppConfig
	Worker   WorkerConfig
	Queue    QueueConfig
	Consumer ConsumerConfig
}

type RabbitMQConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
}

type AppConfig struct {
	Name     string
	LogLevel string
}

type WorkerConfig struct {
	PoolSize        int
	MaxRetries      int
	RetryDelay      time.Duration
	RetryTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type QueueConfig struct {
	Name         string
	ExchangeName string
	RoutingKey   string
	DLQName      string
	DLQExchange  string
}

type ConsumerConfig struct {
	ShutdownTimeout     time.Duration
	StatsReportInterval time.Duration
	HealthCheckInterval time.Duration
}

// Load gets the environment variables and returns a Config struct
func Load() *Config {
	return &Config{
		RabbitMQ: RabbitMQConfig{
			Host:     getEnv("RABBITMQ_HOST", "localhost"),
			Port:     getEnv("RABBITMQ_PORT", "5672"),
			User:     getEnv("RABBITMQ_USER", "guest"),
			Password: getEnv("RABBITMQ_PASSWORD", "guest"),
			VHost:    getEnv("RABBITMQ_VHOST", "/"),
		},
		App: AppConfig{
			Name:     getEnv("APP_NAME", "gohopper"),
			LogLevel: getEnv("LOG_LEVEL", "info"),
		},
		Worker: WorkerConfig{
			PoolSize:        getEnvAsInt("WORKER_POOL_SIZE", 5),
			MaxRetries:      getEnvAsInt("MAX_RETRIES", 3),
			RetryDelay:      getEnvAsDuration("RETRY_DELAY", 1000*time.Millisecond),
			RetryTimeout:    getEnvAsDuration("RETRY_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getEnvAsDuration("WORKER_SHUTDOWN_TIMEOUT", 30*time.Second),
		},
		Queue: QueueConfig{
			Name:         getEnv("QUEUE_NAME", "events"),
			ExchangeName: getEnv("EXCHANGE_NAME", "events_exchange"),
			RoutingKey:   getEnv("ROUTING_KEY", "event.*"),
			DLQName:      getEnv("DLQ_NAME", "events_dlq"),
			DLQExchange:  getEnv("DLQ_EXCHANGE", "events_dlq_exchange"),
		},
		Consumer: ConsumerConfig{
			ShutdownTimeout:     getEnvAsDuration("CONSUMER_SHUTDOWN_TIMEOUT", 10*time.Second),
			StatsReportInterval: getEnvAsDuration("STATS_REPORT_INTERVAL", 30*time.Second),
			HealthCheckInterval: getEnvAsDuration("HEALTH_CHECK_INTERVAL", 60*time.Second),
		},
	}
}

// getEnv gets the environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets the environment variable as an int
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsDuration gets the environment variable as a Duration
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
