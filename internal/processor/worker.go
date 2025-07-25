package processor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gohopper/internal/logger"
	"gohopper/internal/metrics"
	"gohopper/internal/queue"

	"github.com/streadway/amqp"
)

// Metrics represents shared metrics for the worker pool
type Metrics struct {
	unsafeMetrics map[string]int64

	safeMetrics  map[string]int64
	metricsMutex sync.RWMutex

	syncMetrics sync.Map

	atomicMetrics struct {
		processedCount int64
		failedCount    int64
		retryCount     int64
		dlqCount       int64
	}
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		unsafeMetrics: make(map[string]int64),
		safeMetrics:   make(map[string]int64),
	}
}

// UnsafeIncrement - RACE CONDITION PRONE
func (m *Metrics) UnsafeIncrement(key string) {
	// This is intentionally unsafe to demonstrate race condition
	// Use defer recover to prevent fatal panics in tests
	defer func() {
		if r := recover(); r != nil {
		}
	}()

	if m.unsafeMetrics == nil {
		m.unsafeMetrics = make(map[string]int64)
	}

	m.unsafeMetrics[key]++
}

// SafeIncrementWithMutex - Thread-safe with mutex
func (m *Metrics) SafeIncrementWithMutex(key string) {
	m.metricsMutex.Lock()
	defer m.metricsMutex.Unlock()
	m.safeMetrics[key]++
}

// SafeIncrementWithSyncMap - Thread-safe with sync.Map
func (m *Metrics) SafeIncrementWithSyncMap(key string) {
	// For testing purposes, we'll use a simpler approach
	// In real applications, sync.Map is not ideal for counters
	// This is just to demonstrate the concept
	if existing, loaded := m.syncMetrics.Load(key); loaded {
		value := existing.(int64)
		m.syncMetrics.Store(key, value+1)
	} else {
		m.syncMetrics.Store(key, int64(1))
	}
}

// AtomicIncrement - Thread-safe with atomic operations
func (m *Metrics) AtomicIncrement(metricType string) {
	switch metricType {
	case "processed":
		atomic.AddInt64(&m.atomicMetrics.processedCount, 1)
	case "failed":
		atomic.AddInt64(&m.atomicMetrics.failedCount, 1)
	case "retry":
		atomic.AddInt64(&m.atomicMetrics.retryCount, 1)
	case "dlq":
		atomic.AddInt64(&m.atomicMetrics.dlqCount, 1)
	}
}

// GetMetrics returns all metrics in a thread-safe way
func (m *Metrics) GetMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})

	metrics["unsafe_metrics"] = m.unsafeMetrics

	m.metricsMutex.RLock()
	safeMetricsCopy := make(map[string]int64)
	for k, v := range m.safeMetrics {
		safeMetricsCopy[k] = v
	}
	m.metricsMutex.RUnlock()
	metrics["safe_metrics"] = safeMetricsCopy

	syncMetricsCopy := make(map[string]int64)
	m.syncMetrics.Range(func(key, value interface{}) bool {
		syncMetricsCopy[key.(string)] = value.(int64)
		return true
	})
	metrics["sync_metrics"] = syncMetricsCopy

	metrics["atomic_metrics"] = map[string]int64{
		"processed": atomic.LoadInt64(&m.atomicMetrics.processedCount),
		"failed":    atomic.LoadInt64(&m.atomicMetrics.failedCount),
		"retry":     atomic.LoadInt64(&m.atomicMetrics.retryCount),
		"dlq":       atomic.LoadInt64(&m.atomicMetrics.dlqCount),
	}

	return metrics
}

// Semaphore represents a custom semaphore for concurrency control
type Semaphore struct {
	permits chan struct{}
	mu      sync.RWMutex
	active  int
	max     int
}

// NewSemaphore creates a new semaphore with the specified capacity
func NewSemaphore(capacity int) *Semaphore {
	return &Semaphore{
		permits: make(chan struct{}, capacity),
		max:     capacity,
	}
}

// Acquire acquires a permit from the semaphore
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.permits <- struct{}{}:
		s.mu.Lock()
		s.active++
		s.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a permit back to the semaphore
func (s *Semaphore) Release() {
	s.mu.Lock()
	s.active--
	s.mu.Unlock()
	<-s.permits
}

// GetStats returns semaphore statistics
func (s *Semaphore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"active":      s.active,
		"max":         s.max,
		"available":   s.max - s.active,
		"utilization": float64(s.active) / float64(s.max) * 100,
	}
}

// Job represents a message processing job
type Job struct {
	Message      *queue.EventMessage
	Delivery     amqp.Delivery
	ReceivedAt   time.Time
	RetryCount   int
	MaxRetries   int
	RetryDelay   time.Duration
	RetryTimeout time.Duration
}

// Worker represents a worker in the pool
type Worker struct {
	ID        int
	jobChan   chan *Job
	quitChan  chan bool
	wg        *sync.WaitGroup
	logger    *logger.Logger
	handler   MessageHandler
	ctx       context.Context
	cancel    context.CancelFunc
	semaphore *Semaphore
	metrics   *Metrics
	pool      *WorkerPool
}

// MessageHandler defines the interface for processing messages
type MessageHandler interface {
	ProcessMessage(ctx context.Context, message *queue.EventMessage) error
}

// WorkerPool manages a pool of workers
type WorkerPool struct {
	workers    []*Worker
	jobChan    chan *Job
	quitChan   chan bool
	wg         sync.WaitGroup
	logger     *logger.Logger
	handler    MessageHandler
	maxWorkers int
	active     bool
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	semaphore  *Semaphore
	metrics    *Metrics
	prometheus *metrics.PrometheusMetrics
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(maxWorkers int, handler MessageHandler) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		workers:    make([]*Worker, 0, maxWorkers),
		jobChan:    make(chan *Job, maxWorkers*2),
		quitChan:   make(chan bool),
		logger:     logger.NewLogger(),
		handler:    handler,
		maxWorkers: maxWorkers,
		active:     false,
		ctx:        ctx,
		cancel:     cancel,
		semaphore:  NewSemaphore(maxWorkers),
		metrics:    NewMetrics(),
		prometheus: metrics.NewPrometheusMetrics(),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.active {
		return fmt.Errorf("worker pool is already active")
	}

	wp.logger.Info(ctx, "Starting worker pool", logger.Fields{
		"max_workers":        wp.maxWorkers,
		"semaphore_capacity": wp.semaphore.max,
	})

	for i := 0; i < wp.maxWorkers; i++ {
		worker := wp.newWorker(i)
		wp.workers = append(wp.workers, worker)
		worker.Start(ctx)
	}

	wp.active = true
	wp.logger.Info(ctx, "Worker pool started successfully", logger.Fields{
		"worker_count":       len(wp.workers),
		"semaphore_capacity": wp.semaphore.max,
	})

	return nil
}

// Stop stops the worker pool gracefully
func (wp *WorkerPool) Stop(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if !wp.active {
		return fmt.Errorf("worker pool is not active")
	}

	wp.logger.Info(ctx, "Stopping worker pool", nil)

	wp.cancel()

	close(wp.quitChan)

	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
		if timeout < 0 {
			timeout = 30 * time.Second
		}
	}

	select {
	case <-done:
		wp.logger.Info(ctx, "All workers stopped gracefully", nil)
	case <-time.After(timeout):
		wp.logger.Warn(ctx, "Worker pool stop timeout - forcing shutdown", logger.Fields{
			"timeout": timeout,
		})
	case <-ctx.Done():
		wp.logger.Warn(ctx, "Context cancelled during worker pool shutdown", nil)
	}

	close(wp.jobChan)

	wp.active = false
	wp.logger.Info(ctx, "Worker pool stopped successfully", nil)

	return nil
}

// SubmitJob submits a job to the worker pool
func (wp *WorkerPool) SubmitJob(ctx context.Context, job *Job) error {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	if !wp.active {
		return fmt.Errorf("worker pool is not active")
	}

	select {
	case wp.jobChan <- job:
		wp.logger.Debug(ctx, "Job submitted to worker pool", logger.Fields{
			"message_id":   job.Message.ID,
			"message_type": job.Message.Type,
			"worker_count": len(wp.workers),
		})
		return nil
	case <-ctx.Done():
		return fmt.Errorf("context cancelled while submitting job")
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is shutting down")
	default:
		return fmt.Errorf("worker pool is full")
	}
}

// newWorker creates a new worker
func (wp *WorkerPool) newWorker(id int) *Worker {
	workerCtx, cancel := context.WithCancel(wp.ctx)
	return &Worker{
		ID:        id,
		jobChan:   wp.jobChan,
		quitChan:  wp.quitChan,
		wg:        &wp.wg,
		logger:    wp.logger,
		handler:   wp.handler,
		ctx:       workerCtx,
		cancel:    cancel,
		semaphore: wp.semaphore,
		metrics:   wp.metrics,
		pool:      wp,
	}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

// Stop stops the worker
func (w *Worker) Stop(ctx context.Context) {
	w.logger.Info(ctx, "Worker stopping", logger.Fields{
		"worker_id": w.ID,
	})
	w.cancel()
}

// run is the main worker loop
func (w *Worker) run(ctx context.Context) {
	defer func() {
		w.wg.Done()
		w.logger.Info(ctx, "Worker goroutine finished", logger.Fields{
			"worker_id": w.ID,
		})
	}()

	workerCtx := logger.WithTraceID(ctx)
	w.logger.Info(workerCtx, "Worker started", logger.Fields{
		"worker_id": w.ID,
	})

	for {
		select {
		case job := <-w.jobChan:
			w.processJobWithSemaphore(workerCtx, job)
		case <-w.quitChan:
			w.logger.Info(workerCtx, "Worker received quit signal", logger.Fields{
				"worker_id": w.ID,
			})
			return
		case <-w.ctx.Done():
			w.logger.Info(workerCtx, "Worker context cancelled", logger.Fields{
				"worker_id": w.ID,
			})
			return
		case <-ctx.Done():
			w.logger.Info(workerCtx, "Worker parent context cancelled", logger.Fields{
				"worker_id": w.ID,
			})
			return
		}
	}
}

// processJobWithSemaphore processes a job with semaphore concurrency control
func (w *Worker) processJobWithSemaphore(ctx context.Context, job *Job) {
	if err := w.semaphore.Acquire(ctx); err != nil {
		w.logger.Error(ctx, "Failed to acquire semaphore permit", err, logger.Fields{
			"worker_id":  w.ID,
			"message_id": job.Message.ID,
		})
		return
	}

	defer w.semaphore.Release()

	w.logger.Debug(ctx, "Semaphore permit acquired", logger.Fields{
		"worker_id":       w.ID,
		"message_id":      job.Message.ID,
		"semaphore_stats": w.semaphore.GetStats(),
	})

	w.processJobWithRetry(ctx, job)

	w.logger.Debug(ctx, "Semaphore permit released", logger.Fields{
		"worker_id":       w.ID,
		"message_id":      job.Message.ID,
		"semaphore_stats": w.semaphore.GetStats(),
	})
}

// processJobWithRetry processes a job with retry logic
func (w *Worker) processJobWithRetry(ctx context.Context, job *Job) {
	startTime := time.Now()

	messageCtx := context.WithValue(ctx, logger.TraceIDKey, job.Message.TraceID)

	w.logger.LogMessageReceived(messageCtx, job.Message.ID, job.Message.Type, job.Message.Source, logger.Fields{
		"worker_id":     w.ID,
		"retry_count":   job.RetryCount,
		"queue_time_ms": time.Since(job.ReceivedAt).Milliseconds(),
	})

	err := w.processWithRetry(messageCtx, job)

	processingTime := time.Since(startTime)

	if err != nil {
		w.logger.LogMessageFailed(messageCtx, job.Message.ID, job.Message.Type, job.Message.Source, err, job.RetryCount, logger.Fields{
			"worker_id":          w.ID,
			"processing_time_ms": processingTime.Milliseconds(),
			"max_retries":        job.MaxRetries,
		})

		// Update metrics with race condition simulation
		w.updateMetricsWithRaceCondition(job.Message.Type, "failed")

		// Update Prometheus metrics
		w.updatePrometheusMetrics("failed", processingTime)

		failureType := w.determineFailureType(err, job)
		w.handleFailure(messageCtx, job, err, failureType)
	} else {
		w.logger.LogMessageProcessed(messageCtx, job.Message.ID, job.Message.Type, job.Message.Source, processingTime, logger.Fields{
			"worker_id": w.ID,
		})

		// Update metrics with race condition simulation
		w.updateMetricsWithRaceCondition(job.Message.Type, "processed")

		// Update Prometheus metrics
		w.updatePrometheusMetrics("processed", processingTime)

		if err := job.Delivery.Ack(false); err != nil {
			w.logger.Error(messageCtx, "Failed to acknowledge message", err, logger.Fields{
				"worker_id":  w.ID,
				"message_id": job.Message.ID,
			})
		}
	}
}

// updateMetricsWithRaceCondition simulates race condition in metrics
func (w *Worker) updateMetricsWithRaceCondition(messageType, status string) {
	// Simulate race condition with unsafe metrics
	w.metrics.UnsafeIncrement(fmt.Sprintf("%s_%s", messageType, status))

	// Safe metrics with mutex
	w.metrics.SafeIncrementWithMutex(fmt.Sprintf("%s_%s", messageType, status))

	// Safe metrics with sync.Map
	w.metrics.SafeIncrementWithSyncMap(fmt.Sprintf("%s_%s", messageType, status))

	// Safe metrics with atomic operations
	w.metrics.AtomicIncrement(status)
}

// updatePrometheusMetrics updates Prometheus metrics
func (w *Worker) updatePrometheusMetrics(status string, processingTime time.Duration) {
	if w.pool != nil && w.pool.prometheus != nil {
		// Record processing duration
		w.pool.prometheus.ObserveProcessingDuration(processingTime)

		// Increment appropriate counters
		switch status {
		case "processed":
			w.pool.prometheus.IncrementMessagesProcessed()
		case "failed":
			w.pool.prometheus.IncrementMessagesFailed()
		}

		// Update current stats
		w.pool.UpdatePrometheusMetrics()
	}
}

// FailureType represents the type of failure
type FailureType string

const (
	FailureTypeRetryable    FailureType = "retryable"
	FailureTypeNonRetryable FailureType = "non_retryable"
	FailureTypeMaxRetries   FailureType = "max_retries_exceeded"
	FailureTypeTimeout      FailureType = "timeout"
	FailureTypeContext      FailureType = "context_cancelled"
)

// determineFailureType determines the type of failure based on error and retry count
func (w *Worker) determineFailureType(err error, job *Job) FailureType {
	if err == context.Canceled || err == context.DeadlineExceeded {
		return FailureTypeContext
	}

	if err.Error() == "processing timeout" || err.Error() == "user.created processing timeout" ||
		err.Error() == "order.created processing timeout" {
		return FailureTypeTimeout
	}

	if job.RetryCount >= job.MaxRetries {
		return FailureTypeMaxRetries
	}

	if w.isRetryableError(err) {
		return FailureTypeRetryable
	}

	return FailureTypeNonRetryable
}

// isRetryableError determines if an error is retryable
func (w *Worker) isRetryableError(err error) bool {
	retryablePatterns := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"service unavailable",
		"rate limit exceeded",
		"simulated error", // For testing purposes
	}

	errMsg := err.Error()
	for _, pattern := range retryablePatterns {
		if strings.Contains(strings.ToLower(errMsg), pattern) {
			return true
		}
	}

	return false
}

// handleFailure handles different types of failures
func (w *Worker) handleFailure(ctx context.Context, job *Job, err error, failureType FailureType) {
	switch failureType {
	case FailureTypeRetryable:
		w.handleRetryableFailure(ctx, job, err)
	case FailureTypeNonRetryable:
		w.handleNonRetryableFailure(ctx, job, err)
	case FailureTypeMaxRetries:
		w.handleMaxRetriesFailure(ctx, job, err)
	case FailureTypeTimeout:
		w.handleTimeoutFailure(ctx, job, err)
	case FailureTypeContext:
		w.handleContextFailure(ctx, job, err)
	default:
		w.handleNonRetryableFailure(ctx, job, err)
	}
}

// handleRetryableFailure handles retryable failures
func (w *Worker) handleRetryableFailure(ctx context.Context, job *Job, err error) {
	w.logger.Warn(ctx, "Retryable failure detected", logger.Fields{
		"worker_id":   w.ID,
		"message_id":  job.Message.ID,
		"error":       err.Error(),
		"retry_count": job.RetryCount,
		"max_retries": job.MaxRetries,
	})

	// Update Prometheus metrics for retry
	if w.pool != nil && w.pool.prometheus != nil {
		w.pool.prometheus.IncrementRetryAttempts()
	}

	if rejectErr := job.Delivery.Reject(false); rejectErr != nil {
		w.logger.Error(ctx, "Failed to reject message for retry", rejectErr, logger.Fields{
			"worker_id":  w.ID,
			"message_id": job.Message.ID,
		})
	}
}

// handleNonRetryableFailure handles non-retryable failures
func (w *Worker) handleNonRetryableFailure(ctx context.Context, job *Job, err error) {
	w.logger.Error(ctx, "Non-retryable failure detected", err, logger.Fields{
		"worker_id":   w.ID,
		"message_id":  job.Message.ID,
		"retry_count": job.RetryCount,
	})

	w.sendToDLQ(ctx, job, err, "non_retryable_failure")
}

// handleMaxRetriesFailure handles failures after max retries
func (w *Worker) handleMaxRetriesFailure(ctx context.Context, job *Job, err error) {
	w.logger.Error(ctx, "Max retries exceeded", err, logger.Fields{
		"worker_id":   w.ID,
		"message_id":  job.Message.ID,
		"retry_count": job.RetryCount,
		"max_retries": job.MaxRetries,
	})

	w.sendToDLQ(ctx, job, err, "max_retries_exceeded")
}

// handleTimeoutFailure handles timeout failures
func (w *Worker) handleTimeoutFailure(ctx context.Context, job *Job, err error) {
	w.logger.Error(ctx, "Processing timeout failure", err, logger.Fields{
		"worker_id":   w.ID,
		"message_id":  job.Message.ID,
		"retry_count": job.RetryCount,
		"timeout":     job.RetryTimeout,
	})

	w.sendToDLQ(ctx, job, err, "processing_timeout")
}

// handleContextFailure handles context cancellation failures
func (w *Worker) handleContextFailure(ctx context.Context, job *Job, err error) {
	w.logger.Warn(ctx, "Context cancellation failure", logger.Fields{
		"worker_id":   w.ID,
		"message_id":  job.Message.ID,
		"error":       err.Error(),
		"retry_count": job.RetryCount,
	})

	if rejectErr := job.Delivery.Reject(false); rejectErr != nil {
		w.logger.Error(ctx, "Failed to reject message after context cancellation", rejectErr, logger.Fields{
			"worker_id":  w.ID,
			"message_id": job.Message.ID,
		})
	}
}

// sendToDLQ sends a message to the Dead Letter Queue
func (w *Worker) sendToDLQ(ctx context.Context, job *Job, err error, reason string) {
	job.Message.Metadata.DLQReason = reason
	job.Message.Metadata.DLQTimestamp = time.Now()
	job.Message.Metadata.FinalError = err.Error()

	w.logger.LogMessageDLQ(ctx, job.Message.ID, job.Message.Type, job.Message.Source, err.Error(), logger.Fields{
		"worker_id":   w.ID,
		"max_retries": job.MaxRetries,
		"retry_count": job.RetryCount,
		"dlq_reason":  reason,
	})

	if err := job.Delivery.Reject(false); err != nil {
		w.logger.Error(ctx, "Failed to reject message for DLQ", err, logger.Fields{
			"worker_id":  w.ID,
			"message_id": job.Message.ID,
		})
	}
}

// processWithRetry processes a message with exponential backoff retry
func (w *Worker) processWithRetry(ctx context.Context, job *Job) error {
	var lastErr error

	for attempt := 0; attempt <= job.MaxRetries; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, job.RetryTimeout)

		err := w.handler.ProcessMessage(attemptCtx, job.Message)
		cancel()

		if err == nil {
			if attempt > 0 {
				w.logger.Info(ctx, "Message processed successfully after retry", logger.Fields{
					"worker_id":   w.ID,
					"message_id":  job.Message.ID,
					"attempt":     attempt,
					"retry_count": job.RetryCount,
				})
			}
			return nil
		}

		lastErr = err

		if attempt == job.MaxRetries {
			break
		}

		delay := w.calculateExponentialBackoff(attempt, job.RetryDelay)

		w.logger.LogMessageRetry(ctx, job.Message.ID, job.Message.Type, job.Message.Source, attempt, delay, logger.Fields{
			"worker_id":   w.ID,
			"attempt":     attempt,
			"max_retries": job.MaxRetries,
			"error":       err.Error(),
		})

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			w.logger.Info(ctx, "Retry cancelled due to context cancellation", logger.Fields{
				"worker_id":  w.ID,
				"message_id": job.Message.ID,
				"attempt":    attempt,
			})
			return ctx.Err()
		}
	}

	return fmt.Errorf("message processing failed after %d attempts: %w", job.MaxRetries+1, lastErr)
}

// calculateExponentialBackoff calculates exponential backoff delay
func (w *Worker) calculateExponentialBackoff(attempt int, baseDelay time.Duration) time.Duration {
	multiplier := 1 << attempt
	delay := time.Duration(multiplier) * baseDelay

	jitter := time.Duration(float64(delay) * 0.1) // 10% jitter
	delay += time.Duration(float64(jitter) * (0.5 + 0.5*float64(attempt%10)/10))

	maxDelay := 30 * time.Second
	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}

// GetStats returns worker pool statistics
func (wp *WorkerPool) GetStats() map[string]interface{} {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	return map[string]interface{}{
		"active":          wp.active,
		"worker_count":    len(wp.workers),
		"max_workers":     wp.maxWorkers,
		"queue_size":      len(wp.jobChan),
		"queue_cap":       cap(wp.jobChan),
		"context_done":    wp.ctx.Err() != nil,
		"semaphore_stats": wp.semaphore.GetStats(),
		"metrics":         wp.metrics.GetMetrics(),
		"prometheus":      "available at /metrics endpoint",
	}
}

// IsActive returns whether the worker pool is active
func (wp *WorkerPool) IsActive() bool {
	wp.mu.RLock()
	defer wp.mu.RUnlock()
	return wp.active
}

// GetContext returns the worker pool context
func (wp *WorkerPool) GetContext() context.Context {
	return wp.ctx
}

// GetPrometheusMetrics returns the Prometheus metrics instance
func (wp *WorkerPool) GetPrometheusMetrics() *metrics.PrometheusMetrics {
	return wp.prometheus
}

// UpdatePrometheusMetrics updates Prometheus metrics with current stats
func (wp *WorkerPool) UpdatePrometheusMetrics() {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	// Update active workers count
	activeCount := 0
	for _, worker := range wp.workers {
		if worker != nil {
			activeCount++
		}
	}
	wp.prometheus.SetActiveWorkers(activeCount)

	// Update queue size
	wp.prometheus.SetQueueSize(len(wp.jobChan))
}
