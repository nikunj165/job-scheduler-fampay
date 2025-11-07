package repository

import (
	"context"
	"time"

	"job-scheduler-fampay/models"
)

// JobRepository defines the interface for job storage operations
type JobRepository interface {
	// Job CRUD operations
	CreateJob(ctx context.Context, job *models.Job) error
	GetJob(ctx context.Context, jobID string) (*models.Job, error)
	GetAllJobs(ctx context.Context, limit, offset int, status *models.JobStatus) ([]*models.Job, int, error)
	UpdateJob(ctx context.Context, jobID string, updates map[string]interface{}) error
	DeleteJob(ctx context.Context, jobID string) error

	// Execution tracking
	CreateExecution(ctx context.Context, execution *models.JobExecution) error
	GetJobExecutions(ctx context.Context, jobID string, limit, offset int) ([]*models.JobExecution, int, error)
	UpdateExecution(ctx context.Context, executionID string, updates map[string]interface{}) error

	// Statistics
	GetJobStats(ctx context.Context, jobID string) (*models.JobStats, error)
	GetSchedulerStats(ctx context.Context, from, to time.Time) (map[string]interface{}, error)

	// Scheduler operations
	GetActiveJobs(ctx context.Context) ([]*models.Job, error)
	UpdateJobNextRun(ctx context.Context, jobID string, nextRun time.Time) error
	UpdateJobLastExecuted(ctx context.Context, jobID string, lastExecuted time.Time) error
}
