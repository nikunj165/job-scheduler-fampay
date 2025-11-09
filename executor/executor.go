package executor

import (
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"job-scheduler-fampay/models"
	"job-scheduler-fampay/repository"
)

// JobExecutorConfig holds configuration for the job execution workers.
type JobExecutorConfig struct {
	WorkerCount    int
	RequestTimeout time.Duration
	MaxQueueSize   int
}

// DefaultJobExecutorConfig provides sensible defaults for the executor.
func DefaultJobExecutorConfig() JobExecutorConfig {
	return JobExecutorConfig{
		WorkerCount:    4,
		RequestTimeout: 120 * time.Second, // Increased to handle APIs that take up to 90 seconds
		MaxQueueSize:   100,
	}
}

// JobExecutor is responsible for executing scheduled jobs.
type JobExecutor struct {
	config     JobExecutorConfig
	repo       repository.JobRepository
	httpClient *http.Client

	queue chan *models.Job

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewJobExecutor creates a new executor instance with the provided configuration.
func NewJobExecutor(repo repository.JobRepository, cfg JobExecutorConfig) *JobExecutor {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = DefaultJobExecutorConfig().WorkerCount
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = DefaultJobExecutorConfig().RequestTimeout
	}
	if cfg.MaxQueueSize <= 0 {
		cfg.MaxQueueSize = DefaultJobExecutorConfig().MaxQueueSize
	}

	return &JobExecutor{
		config: cfg,
		repo:   repo,
		httpClient: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
		queue: make(chan *models.Job, cfg.MaxQueueSize),
	}
}

// Start launches the worker pool. It is safe to call multiple times; subsequent calls are no-ops.
func (e *JobExecutor) Start(ctx context.Context) {
	e.startOnce.Do(func() {
		var workerCtx context.Context
		workerCtx, e.cancel = context.WithCancel(ctx)

		for i := 0; i < e.config.WorkerCount; i++ {
			e.wg.Add(1)
			go e.runWorker(workerCtx, i)
		}
	})
}

// Stop gracefully shuts down all workers and drains outstanding jobs.
func (e *JobExecutor) Stop() {
	e.stopOnce.Do(func() {
		if e.cancel != nil {
			e.cancel()
		}
		close(e.queue)
		e.wg.Wait()
	})
}

// Submit queues a job for execution. If the executor is not started,
// the job will block until Start is invoked or the context is canceled.
func (e *JobExecutor) Submit(job *models.Job) {
	if job == nil {
		return
	}
	e.queue <- job
}

func (e *JobExecutor) runWorker(ctx context.Context, workerID int) {
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

func (e *JobExecutor) executeJob(ctx context.Context, job *models.Job) {
	if job == nil {
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, e.config.RequestTimeout)
	defer cancel()

	start := time.Now()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, job.API, nil)
	if err != nil {
		log.Printf("executor: build request for job %s failed: %v", job.ID, err)
		e.recordExecutionFailure(ctx, job, time.Since(start), err.Error(), 0, "")
		return
	}

	resp, err := e.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("executor: job %s http failure: %v", job.ID, err)
		e.recordExecutionFailure(ctx, job, duration, err.Error(), 0, "")
		return
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap to 1MB
	if readErr != nil {
		log.Printf("executor: job %s read body error: %v", job.ID, readErr)
	}

	success := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	errorMsg := ""
	if !success {
		errorMsg = http.StatusText(resp.StatusCode)
	}

	execution := &models.JobExecution{
		JobID:        job.ID,
		HTTPStatus:   resp.StatusCode,
		ResponseTime: duration,
		Success:      success,
		Error:        errorMsg,
		ResponseBody: string(bodyBytes),
	}

	completedAt := time.Now()
	execution.CompletedAt = &completedAt

	if err := e.repo.CreateExecution(ctx, execution); err != nil {
		log.Printf("executor: failed to record execution for job %s: %v", job.ID, err)
		return
	}

	if err := e.repo.UpdateJobLastExecuted(ctx, job.ID, completedAt); err != nil {
		log.Printf("executor: failed to update last executed for job %s: %v", job.ID, err)
	}
}

func (e *JobExecutor) recordExecutionFailure(ctx context.Context, job *models.Job, duration time.Duration, errorMsg string, status int, body string) {
	finished := time.Now()
	execution := &models.JobExecution{
		JobID:        job.ID,
		HTTPStatus:   status,
		ResponseTime: duration,
		Success:      false,
		Error:        errorMsg,
		ResponseBody: body,
		CompletedAt:  &finished,
	}

	if err := e.repo.CreateExecution(ctx, execution); err != nil {
		log.Printf("executor: failed to record execution failure for job %s: %v", job.ID, err)
		return
	}

	if err := e.repo.UpdateJobLastExecuted(ctx, job.ID, finished); err != nil {
		log.Printf("executor: failed to update last executed for job %s: %v", job.ID, err)
	}
}
