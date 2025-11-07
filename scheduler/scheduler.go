package scheduler

import (
	"context"
	"log"
	"sync"
	"time"

	"job-scheduler-fampay/cron"
	"job-scheduler-fampay/models"
	"job-scheduler-fampay/repository"
)

// Executor defines the minimal interface required from a job executor.
type Executor interface {
	Submit(job *models.Job)
}

// Config contains tunable parameters for the scheduler loop.
type Config struct {
	PollInterval time.Duration
}

// DefaultConfig returns sane defaults for scheduler operation.
func DefaultConfig() Config {
	return Config{
		PollInterval: 5 * time.Second,
	}
}

// Scheduler orchestrates polling for due jobs and handing them to an executor.
type Scheduler struct {
	config   Config
	repo     repository.JobRepository
	parser   *cron.Parser
	executor Executor
	logger   *log.Logger

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// New creates a new scheduler instance.
func New(repo repository.JobRepository, executor Executor, logger *log.Logger, cfg Config) *Scheduler {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultConfig().PollInterval
	}
	if logger == nil {
		logger = log.Default()
	}

	return &Scheduler{
		config:   cfg,
		repo:     repo,
		parser:   cron.NewParser(),
		executor: executor,
		logger:   logger,
	}
}

// Start launches the scheduler loop. Subsequent calls are no-ops.
func (s *Scheduler) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		var runCtx context.Context
		runCtx, s.cancel = context.WithCancel(ctx)
		s.wg.Add(1)
		go s.run(runCtx)
	})
}

// Stop requests shutdown of the scheduler loop and waits for completion.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.wg.Wait()
	})
}

func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	// Initial sweep immediately.
	s.dispatchDueJobs(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.dispatchDueJobs(ctx)
		}
	}
}

func (s *Scheduler) dispatchDueJobs(ctx context.Context) {
	jobs, err := s.repo.GetActiveJobs(ctx)
	if err != nil {
		s.logger.Printf("scheduler: failed to load active jobs: %v", err)
		return
	}

	now := time.Now()
	for _, job := range jobs {
		if job == nil {
			continue
		}

		if job.NextRun == nil || now.Before(*job.NextRun) {
			continue
		}

		s.executor.Submit(job)

		nextRun, err := s.parser.GetNextRun(job.Schedule, now)
		if err != nil {
			s.logger.Printf("scheduler: failed to compute next run for job %s: %v", job.ID, err)
			continue
		}

		if err := s.repo.UpdateJobNextRun(ctx, job.ID, nextRun); err != nil {
			s.logger.Printf("scheduler: failed to update next run for job %s: %v", job.ID, err)
		}
	}
}
