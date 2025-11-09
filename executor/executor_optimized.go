package executor

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"job-scheduler-fampay/models"
	"job-scheduler-fampay/repository"
)

// OptimizedJobExecutorConfig holds configuration for high-throughput job execution.
type OptimizedJobExecutorConfig struct {
	WorkerCount     int
	RequestTimeout  time.Duration
	MaxQueueSize    int
	MaxConnsPerHost int
	MaxIdleConns    int
	IdleConnTimeout time.Duration
}

// DefaultOptimizedConfig provides configuration for 1000+ jobs/second.
func DefaultOptimizedConfig() OptimizedJobExecutorConfig {
	return OptimizedJobExecutorConfig{
		WorkerCount:     1000, // Match target throughput
		RequestTimeout:  120 * time.Second,
		MaxQueueSize:    10000, // Buffer for burst traffic
		MaxConnsPerHost: 100,   // Allow many concurrent connections per host
		MaxIdleConns:    1000,  // Keep connections alive for reuse
		IdleConnTimeout: 90 * time.Second,
	}
}

// OptimizedJobExecutor is optimized for high-throughput job execution.
type OptimizedJobExecutor struct {
	config     OptimizedJobExecutorConfig
	repo       repository.JobRepository
	httpClient *http.Client

	queue chan *models.Job

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Metrics for monitoring
	processed uint64
	failed    uint64
	mu        sync.RWMutex
}

// NewOptimizedJobExecutor creates an executor optimized for high throughput.
func NewOptimizedJobExecutor(repo repository.JobRepository, cfg OptimizedJobExecutorConfig) *OptimizedJobExecutor {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = DefaultOptimizedConfig().WorkerCount
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = DefaultOptimizedConfig().RequestTimeout
	}
	if cfg.MaxQueueSize <= 0 {
		cfg.MaxQueueSize = DefaultOptimizedConfig().MaxQueueSize
	}
	if cfg.MaxConnsPerHost <= 0 {
		cfg.MaxConnsPerHost = DefaultOptimizedConfig().MaxConnsPerHost
	}
	if cfg.MaxIdleConns <= 0 {
		cfg.MaxIdleConns = DefaultOptimizedConfig().MaxIdleConns
	}
	if cfg.IdleConnTimeout <= 0 {
		cfg.IdleConnTimeout = DefaultOptimizedConfig().IdleConnTimeout
	}

	// Optimized HTTP transport for high concurrency
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          cfg.MaxIdleConns,
		MaxIdleConnsPerHost:   cfg.MaxConnsPerHost,
		MaxConnsPerHost:       cfg.MaxConnsPerHost,
		IdleConnTimeout:       cfg.IdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true, // Reduce CPU overhead
		ForceAttemptHTTP2:     true, // Use HTTP/2 when available
	}

	return &OptimizedJobExecutor{
		config: cfg,
		repo:   repo,
		httpClient: &http.Client{
			Timeout:   cfg.RequestTimeout,
			Transport: transport,
		},
		queue: make(chan *models.Job, cfg.MaxQueueSize),
	}
}

// Start launches the worker pool.
func (e *OptimizedJobExecutor) Start(ctx context.Context) {
	e.startOnce.Do(func() {
		var workerCtx context.Context
		workerCtx, e.cancel = context.WithCancel(ctx)

		log.Printf("Starting optimized executor with %d workers, queue size %d",
			e.config.WorkerCount, e.config.MaxQueueSize)

		for i := 0; i < e.config.WorkerCount; i++ {
			e.wg.Add(1)
			go e.runWorker(workerCtx, i)
		}

		// Metrics reporter
		e.wg.Add(1)
		go e.reportMetrics(workerCtx)
	})
}

// Stop gracefully shuts down all workers.
func (e *OptimizedJobExecutor) Stop() {
	e.stopOnce.Do(func() {
		if e.cancel != nil {
			e.cancel()
		}
		close(e.queue)
		e.wg.Wait()

		e.mu.RLock()
		log.Printf("Executor stopped. Processed: %d, Failed: %d", e.processed, e.failed)
		e.mu.RUnlock()
	})
}

// Submit queues a job for execution (non-blocking with buffer).
func (e *OptimizedJobExecutor) Submit(job *models.Job) {
	if job == nil {
		return
	}

	select {
	case e.queue <- job:
		// Job queued successfully
	default:
		// Queue is full, log and drop (or implement backpressure)
		log.Printf("WARNING: Job queue full, dropping job %s", job.ID)
	}
}

func (e *OptimizedJobExecutor) runWorker(ctx context.Context, workerID int) {
	defer e.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-e.queue:
			if !ok {
				return
			}
			e.executeJob(ctx, job)
		}
	}
}

func (e *OptimizedJobExecutor) executeJob(ctx context.Context, job *models.Job) {
	if job == nil {
		return
	}

	start := time.Now()

	// Create request with timeout
	reqCtx, cancel := context.WithTimeout(ctx, e.config.RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, job.API, nil)
	if err != nil {
		e.recordFailure(ctx, job, time.Since(start), err.Error(), 0)
		return
	}

	// Execute HTTP request
	resp, err := e.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		e.recordFailure(ctx, job, duration, err.Error(), 0)
		return
	}
	defer resp.Body.Close()

	// Read response (limited to prevent memory issues)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit

	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	// Record execution asynchronously to avoid blocking
	go e.recordExecution(context.Background(), job, duration, resp.StatusCode, string(bodyBytes), success)

	// Update metrics
	e.mu.Lock()
	e.processed++
	if !success {
		e.failed++
	}
	e.mu.Unlock()
}

func (e *OptimizedJobExecutor) recordExecution(ctx context.Context, job *models.Job, duration time.Duration, status int, body string, success bool) {
	execution := &models.JobExecution{
		JobID:        job.ID,
		HTTPStatus:   status,
		ResponseTime: duration,
		Success:      success,
		ResponseBody: body,
	}

	if !success {
		execution.Error = http.StatusText(status)
	}

	completedAt := time.Now()
	execution.CompletedAt = &completedAt

	// Best effort recording - don't block on repository operations
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := e.repo.CreateExecution(ctx, execution); err != nil {
		log.Printf("Failed to record execution for job %s: %v", job.ID, err)
	}

	if err := e.repo.UpdateJobLastExecuted(ctx, job.ID, completedAt); err != nil {
		log.Printf("Failed to update last executed for job %s: %v", job.ID, err)
	}
}

func (e *OptimizedJobExecutor) recordFailure(ctx context.Context, job *models.Job, duration time.Duration, errorMsg string, status int) {
	go e.recordExecution(context.Background(), job, duration, status, "", false)
}

func (e *OptimizedJobExecutor) reportMetrics(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	var lastProcessed uint64

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.mu.RLock()
			current := e.processed
			failed := e.failed
			e.mu.RUnlock()

			rate := float64(current-lastProcessed) / 10.0
			log.Printf("Executor metrics: Rate: %.1f jobs/sec, Total: %d, Failed: %d, Queue: %d/%d",
				rate, current, failed, len(e.queue), e.config.MaxQueueSize)
			lastProcessed = current
		}
	}
}

// GetMetrics returns current executor metrics.
func (e *OptimizedJobExecutor) GetMetrics() (processed, failed uint64, queueDepth int) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.processed, e.failed, len(e.queue)
}
