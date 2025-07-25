package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusMetrics holds all Prometheus metrics
type PrometheusMetrics struct {
	messagesProcessedTotal prometheus.Counter
	messagesFailedTotal    prometheus.Counter
	retryAttemptsTotal     prometheus.Counter

	processingDurationSeconds prometheus.Histogram

	activeWorkers prometheus.Gauge
	queueSize     prometheus.Gauge

	registry *prometheus.Registry
}

// NewPrometheusMetrics creates a new Prometheus metrics instance
func NewPrometheusMetrics() *PrometheusMetrics {
	registry := prometheus.NewRegistry()

	metrics := &PrometheusMetrics{
		registry: registry,
	}

	metrics.messagesProcessedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "messages_processed_total",
		Help: "Total number of messages processed successfully",
	})

	metrics.messagesFailedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "messages_failed_total",
		Help: "Total number of messages that failed processing",
	})

	metrics.retryAttemptsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "retry_attempts_total",
		Help: "Total number of retry attempts",
	})

	metrics.processingDurationSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "processing_duration_seconds",
		Help:    "Time spent processing messages",
		Buckets: prometheus.DefBuckets,
	})

	metrics.activeWorkers = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "active_workers",
		Help: "Number of currently active workers",
	})

	metrics.queueSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "queue_size",
		Help: "Current size of the job queue",
	})

	registry.MustRegister(
		metrics.messagesProcessedTotal,
		metrics.messagesFailedTotal,
		metrics.retryAttemptsTotal,
		metrics.processingDurationSeconds,
		metrics.activeWorkers,
		metrics.queueSize,
	)

	return metrics
}

// IncrementMessagesProcessed increments the processed messages counter
func (pm *PrometheusMetrics) IncrementMessagesProcessed() {
	pm.messagesProcessedTotal.Inc()
}

// IncrementMessagesFailed increments the failed messages counter
func (pm *PrometheusMetrics) IncrementMessagesFailed() {
	pm.messagesFailedTotal.Inc()
}

// IncrementRetryAttempts increments the retry attempts counter
func (pm *PrometheusMetrics) IncrementRetryAttempts() {
	pm.retryAttemptsTotal.Inc()
}

// ObserveProcessingDuration records the processing duration
func (pm *PrometheusMetrics) ObserveProcessingDuration(duration time.Duration) {
	pm.processingDurationSeconds.Observe(duration.Seconds())
}

// SetActiveWorkers sets the number of active workers
func (pm *PrometheusMetrics) SetActiveWorkers(count int) {
	pm.activeWorkers.Set(float64(count))
}

// SetQueueSize sets the current queue size
func (pm *PrometheusMetrics) SetQueueSize(size int) {
	pm.queueSize.Set(float64(size))
}

// GetHandler returns the HTTP handler for the /metrics endpoint
func (pm *PrometheusMetrics) GetHandler() http.Handler {
	return promhttp.HandlerFor(pm.registry, promhttp.HandlerOpts{})
}

// GetRegistry returns the Prometheus registry
func (pm *PrometheusMetrics) GetRegistry() *prometheus.Registry {
	return pm.registry
}
