package processor

import (
	"log"
	"sync"
	"time"
)

type Worker struct {
	id       int
	jobChan  chan Job
	quitChan chan bool
	wg       *sync.WaitGroup
}

type Job struct {
	ID      string
	Payload []byte
	Retries int
}

type Pool struct {
	workers    []*Worker
	jobChan    chan Job
	quitChan   chan bool
	wg         sync.WaitGroup
	maxRetries int
	retryDelay time.Duration
}

// NewPool creates a new worker pool
func NewPool() *Pool {
	poolSize := getEnvAsInt("WORKER_POOL_SIZE", 5)
	maxRetries := getEnvAsInt("MAX_RETRIES", 3)
	retryDelay := getEnvAsDuration("RETRY_DELAY", 1000*time.Millisecond)

	return &Pool{
		workers:    make([]*Worker, poolSize),
		jobChan:    make(chan Job, poolSize*2),
		quitChan:   make(chan bool),
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

// Start starts the worker pool
func (p *Pool) Start() {
	log.Printf("Iniciando pool com %d workers...", len(p.workers))

	for i := 0; i < len(p.workers); i++ {
		worker := &Worker{
			id:       i + 1,
			jobChan:  p.jobChan,
			quitChan: p.quitChan,
			wg:       &p.wg,
		}
		p.workers[i] = worker
		p.wg.Add(1)
		go worker.start()
	}
}

// Stop stops the worker pool
func (p *Pool) Stop() {
	log.Println("Parando pool de workers...")
	close(p.quitChan)
	p.wg.Wait()
}

// SubmitJob submits a job for processing
func (p *Pool) SubmitJob(job Job) {
	p.jobChan <- job
}

// start starts the worker
func (w *Worker) start() {
	defer w.wg.Done()

	log.Printf("Worker %d iniciado", w.id)

	for {
		select {
		case job := <-w.jobChan:
			w.processJob(job)
		case <-w.quitChan:
			log.Printf("Worker %d parando", w.id)
			return
		}
	}
}

// processJob processes a job with retry and DLQ
func (w *Worker) processJob(job Job) {
	log.Printf("Worker %d processando job %s", w.id, job.ID)

	// TODO: Implement retry and DLQ

	// Simulação de processamento
	time.Sleep(100 * time.Millisecond)
	log.Printf("Job %s processado com sucesso", job.ID)
}

// getEnvAsInt gets the environment variable as an int
func getEnvAsInt(key string, defaultValue int) int {
	// TODO: Implement conversion from string to int
	return defaultValue
}

// getEnvAsDuration gets the environment variable as a Duration
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	// TODO: Implement conversion from string to Duration
	return defaultValue
}
