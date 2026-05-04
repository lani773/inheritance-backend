// Package workers implements a high-performance goroutine worker pool
// for background task processing with zero external dependencies.
package workers

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// Task is a unit of work submitted to the pool.
type Task struct {
	Name    string
	Payload interface{}
	Handler func(ctx context.Context, payload interface{}) error
	Retries int
}

// Pool manages a fixed-size goroutine pool with a bounded task queue.
// Design goals:
//   - Fixed memory footprint (no unbounded goroutine spawning)
//   - Backpressure via bounded channel
//   - Graceful shutdown with drain
//   - Per-task retry with exponential backoff
//   - Prometheus-style counters (no external dep)
type Pool struct {
	workers   int
	queue     chan Task
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
	log       *zap.Logger

	// Metrics (atomic counters — safe for concurrent reads)
	processed  atomic.Int64
	failed     atomic.Int64
	retried    atomic.Int64
	queueDepth atomic.Int64
}

// New creates a worker pool with the given concurrency and queue size.
func New(workers, queueSize int, log *zap.Logger) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		workers: workers,
		queue:   make(chan Task, queueSize),
		ctx:     ctx,
		cancel:  cancel,
		log:     log,
	}
	p.start()
	return p
}

// start launches all worker goroutines.
func (p *Pool) start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	p.log.Info("Worker pool started",
		zap.Int("workers", p.workers),
		zap.Int("queueSize", cap(p.queue)),
	)
}

// Submit enqueues a task. Returns false if the queue is full (backpressure).
func (p *Pool) Submit(task Task) bool {
	select {
	case p.queue <- task:
		p.queueDepth.Add(1)
		return true
	default:
		p.log.Warn("Worker pool queue full — task dropped", zap.String("task", task.Name))
		return false
	}
}

// SubmitBlocking enqueues a task, blocking until space is available or ctx is cancelled.
func (p *Pool) SubmitBlocking(ctx context.Context, task Task) error {
	select {
	case p.queue <- task:
		p.queueDepth.Add(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown gracefully drains and stops the pool.
func (p *Pool) Shutdown(timeout time.Duration) {
	p.log.Info("Worker pool shutting down…")
	close(p.queue)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.log.Info("Worker pool stopped cleanly")
	case <-time.After(timeout):
		p.cancel()
		p.log.Warn("Worker pool forced stop after timeout")
	}
}

// Stats returns current pool metrics.
func (p *Pool) Stats() map[string]int64 {
	return map[string]int64{
		"processed":   p.processed.Load(),
		"failed":      p.failed.Load(),
		"retried":     p.retried.Load(),
		"queueDepth":  p.queueDepth.Load(),
		"workerCount": int64(p.workers),
	}
}

// worker processes tasks from the queue with retry logic.
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for task := range p.queue {
		p.queueDepth.Add(-1)
		p.processWithRetry(task)
	}
}

func (p *Pool) processWithRetry(task Task) {
	maxRetries := task.Retries
	if maxRetries < 0 { maxRetries = 0 }

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s, …
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			if backoff > 30*time.Second { backoff = 30 * time.Second }
			p.log.Info("Retrying task",
				zap.String("name", task.Name),
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
			)
			time.Sleep(backoff)
			p.retried.Add(1)
		}

		ctx, cancel := context.WithTimeout(p.ctx, 2*time.Minute)
		err := task.Handler(ctx, task.Payload)
		cancel()

		if err == nil {
			p.processed.Add(1)
			return
		}

		p.log.Error("Task failed",
			zap.String("name", task.Name),
			zap.Int("attempt", attempt+1),
			zap.Int("maxRetries", maxRetries),
			zap.Error(err),
		)
	}
	p.failed.Add(1)
}
