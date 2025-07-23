package processor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gohopper/internal/logger"
	"gohopper/internal/queue"

	"github.com/streadway/amqp"
)

// Job represents a message processing job
type Job struct {
	Message    *queue.EventMessage
	Delivery   amqp.Delivery
	ReceivedAt time.Time
	RetryCount int
	MaxRetries int
	RetryDelay time.Duration
}

// Worker represents a worker in the pool
type Worker struct {
	ID       int
	jobChan  chan *Job
	quitChan chan bool
	wg       *sync.WaitGroup
	logger   *logger.Logger
	handler  MessageHandler
	ctx      context.Context
	cancel   context.CancelFunc
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
		"max_workers": wp.maxWorkers,
	})

	for i := 0; i < wp.maxWorkers; i++ {
		worker := wp.newWorker(i)
		wp.workers = append(wp.workers, worker)
		worker.Start(ctx)
	}

	wp.active = true
	wp.logger.Info(ctx, "Worker pool started successfully", logger.Fields{
		"worker_count": len(wp.workers),
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

	// Use default timeout of 30 seconds if not provided in context
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
		ID:       id,
		jobChan:  wp.jobChan,
		quitChan: wp.quitChan,
		wg:       &wp.wg,
		logger:   wp.logger,
		handler:  wp.handler,
		ctx:      workerCtx,
		cancel:   cancel,
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
			w.processJob(workerCtx, job)
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

// processJob processes a single job
func (w *Worker) processJob(ctx context.Context, job *Job) {
	startTime := time.Now()

	messageCtx := context.WithValue(ctx, "trace_id", job.Message.TraceID)

	w.logger.LogMessageReceived(messageCtx, job.Message.ID, job.Message.Type, job.Message.Source, logger.Fields{
		"worker_id":     w.ID,
		"retry_count":   job.RetryCount,
		"queue_time_ms": time.Since(job.ReceivedAt).Milliseconds(),
	})

	err := w.handler.ProcessMessage(messageCtx, job.Message)

	processingTime := time.Since(startTime)

	if err != nil {
		w.logger.LogMessageFailed(messageCtx, job.Message.ID, job.Message.Type, job.Message.Source, err, job.RetryCount, logger.Fields{
			"worker_id":          w.ID,
			"processing_time_ms": processingTime.Milliseconds(),
		})

		if job.RetryCount < job.MaxRetries {
			w.handleRetry(messageCtx, job)
		} else {
			w.handleDLQ(messageCtx, job, err)
		}
	} else {
		w.logger.LogMessageProcessed(messageCtx, job.Message.ID, job.Message.Type, job.Message.Source, processingTime, logger.Fields{
			"worker_id": w.ID,
		})

		if err := job.Delivery.Ack(false); err != nil {
			w.logger.Error(messageCtx, "Failed to acknowledge message", err, logger.Fields{
				"worker_id":  w.ID,
				"message_id": job.Message.ID,
			})
		}
	}
}

// handleRetry handles message retry logic
func (w *Worker) handleRetry(ctx context.Context, job *Job) {
	delay := w.calculateBackoffDelay(job.RetryCount, job.RetryDelay)

	w.logger.LogMessageRetry(ctx, job.Message.ID, job.Message.Type, job.Message.Source, job.RetryCount, delay, logger.Fields{
		"worker_id": w.ID,
	})

	if err := job.Delivery.Reject(false); err != nil {
		w.logger.Error(ctx, "Failed to reject message for retry", err, logger.Fields{
			"worker_id":  w.ID,
			"message_id": job.Message.ID,
		})
	}
}

// handleDLQ handles sending message to Dead Letter Queue
func (w *Worker) handleDLQ(ctx context.Context, job *Job, err error) {
	w.logger.LogMessageDLQ(ctx, job.Message.ID, job.Message.Type, job.Message.Source, err.Error(), logger.Fields{
		"worker_id":   w.ID,
		"max_retries": job.MaxRetries,
	})

	if err := job.Delivery.Reject(false); err != nil {
		w.logger.Error(ctx, "Failed to reject message for DLQ", err, logger.Fields{
			"worker_id":  w.ID,
			"message_id": job.Message.ID,
		})
	}
}

// calculateBackoffDelay calculates exponential backoff delay
func (w *Worker) calculateBackoffDelay(retryCount int, baseDelay time.Duration) time.Duration {
	multiplier := 1 << retryCount
	delay := time.Duration(multiplier) * baseDelay

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
		"active":       wp.active,
		"worker_count": len(wp.workers),
		"max_workers":  wp.maxWorkers,
		"queue_size":   len(wp.jobChan),
		"queue_cap":    cap(wp.jobChan),
		"context_done": wp.ctx.Err() != nil,
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
