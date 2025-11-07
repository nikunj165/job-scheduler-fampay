package repository

import (
	"context"
	"fmt"
	"sync"
	"time"

	"job-scheduler-fampay/models"
)

// MemoryRepository is an in-memory implementation of JobRepository
type MemoryRepository struct {
	mu         sync.RWMutex
	jobs       map[string]*models.Job
	executions map[string]*models.JobExecution
	jobExecMap map[string][]string // jobID -> executionIDs
	idCounter  int64
}

// NewMemoryRepository creates a new in-memory repository
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		jobs:       make(map[string]*models.Job),
		executions: make(map[string]*models.JobExecution),
		jobExecMap: make(map[string][]string),
		idCounter:  1,
	}
}

// generateID generates a unique ID. Caller must hold r.mu.
func (r *MemoryRepository) generateID(prefix string) string {
	id := fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), r.idCounter)
	r.idCounter++
	return id
}

// CreateJob creates a new job
func (r *MemoryRepository) CreateJob(ctx context.Context, job *models.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if job.ID == "" {
		job.ID = r.generateID("job")
	}

	if _, exists := r.jobs[job.ID]; exists {
		return fmt.Errorf("job with ID %s already exists", job.ID)
	}

	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	job.Status = models.StatusActive

	r.jobs[job.ID] = job
	r.jobExecMap[job.ID] = []string{}

	return nil
}

// GetJob retrieves a job by ID
func (r *MemoryRepository) GetJob(ctx context.Context, jobID string) (*models.Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// Return a copy to prevent external modifications
	jobCopy := *job
	return &jobCopy, nil
}

// GetAllJobs retrieves all jobs with pagination and optional status filter
func (r *MemoryRepository) GetAllJobs(ctx context.Context, limit, offset int, status *models.JobStatus) ([]*models.Job, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filtered []*models.Job
	for _, job := range r.jobs {
		if status == nil || job.Status == *status {
			jobCopy := *job
			filtered = append(filtered, &jobCopy)
		}
	}

	total := len(filtered)

	// Apply pagination
	start := offset
	if start > len(filtered) {
		start = len(filtered)
	}

	end := start + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[start:end], total, nil
}

// UpdateJob updates a job's fields
func (r *MemoryRepository) UpdateJob(ctx context.Context, jobID string, updates map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	// Apply updates
	for key, value := range updates {
		switch key {
		case "schedule":
			if v, ok := value.(string); ok {
				job.Schedule = v
			}
		case "api":
			if v, ok := value.(string); ok {
				job.API = v
			}
		case "type":
			if v, ok := value.(models.JobType); ok {
				job.Type = v
			}
		case "status":
			if v, ok := value.(models.JobStatus); ok {
				job.Status = v
			}
		case "metadata":
			if v, ok := value.(map[string]interface{}); ok {
				job.Metadata = v
			}
		}
	}

	job.UpdatedAt = time.Now()
	return nil
}

// DeleteJob soft deletes a job
func (r *MemoryRepository) DeleteJob(ctx context.Context, jobID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.Status = models.StatusDeleted
	job.UpdatedAt = time.Now()

	return nil
}

// CreateExecution creates a new job execution record
func (r *MemoryRepository) CreateExecution(ctx context.Context, execution *models.JobExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if execution.ID == "" {
		execution.ID = r.generateID("exec")
	}

	// Verify job exists
	if _, exists := r.jobs[execution.JobID]; !exists {
		return fmt.Errorf("job not found: %s", execution.JobID)
	}

	execution.ExecutedAt = time.Now()
	r.executions[execution.ID] = execution
	r.jobExecMap[execution.JobID] = append(r.jobExecMap[execution.JobID], execution.ID)

	return nil
}

// GetJobExecutions retrieves executions for a job
func (r *MemoryRepository) GetJobExecutions(ctx context.Context, jobID string, limit, offset int) ([]*models.JobExecution, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	execIDs, exists := r.jobExecMap[jobID]
	if !exists {
		return nil, 0, fmt.Errorf("job not found: %s", jobID)
	}

	total := len(execIDs)

	// Apply pagination
	start := offset
	if start > len(execIDs) {
		start = len(execIDs)
	}

	end := start + limit
	if end > len(execIDs) {
		end = len(execIDs)
	}

	// Get executions in reverse order (newest first)
	var result []*models.JobExecution
	for i := len(execIDs) - 1 - start; i >= len(execIDs)-end && i >= 0; i-- {
		if exec, ok := r.executions[execIDs[i]]; ok {
			execCopy := *exec
			result = append(result, &execCopy)
		}
	}

	return result, total, nil
}

// UpdateExecution updates an execution record
func (r *MemoryRepository) UpdateExecution(ctx context.Context, executionID string, updates map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	exec, exists := r.executions[executionID]
	if !exists {
		return fmt.Errorf("execution not found: %s", executionID)
	}

	// Apply updates
	for key, value := range updates {
		switch key {
		case "completed_at":
			if v, ok := value.(time.Time); ok {
				exec.CompletedAt = &v
			}
		case "http_status":
			if v, ok := value.(int); ok {
				exec.HTTPStatus = v
			}
		case "response_time":
			if v, ok := value.(time.Duration); ok {
				exec.ResponseTime = v
			}
		case "success":
			if v, ok := value.(bool); ok {
				exec.Success = v
			}
		case "error":
			if v, ok := value.(string); ok {
				exec.Error = v
			}
		case "response_body":
			if v, ok := value.(string); ok {
				exec.ResponseBody = v
			}
		}
	}

	return nil
}

// GetJobStats calculates statistics for a job
func (r *MemoryRepository) GetJobStats(ctx context.Context, jobID string) (*models.JobStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	execIDs := r.jobExecMap[jobID]
	stats := &models.JobStats{
		JobID:             jobID,
		TotalExecutions:   int64(len(execIDs)),
		NextScheduledTime: job.NextRun,
	}

	var totalResponseTime time.Duration
	for _, execID := range execIDs {
		if exec, ok := r.executions[execID]; ok {
			if exec.Success {
				stats.SuccessfulExecutions++
			} else {
				stats.FailedExecutions++
			}
			totalResponseTime += exec.ResponseTime

			// Track last execution time
			if stats.LastExecutionTime == nil || exec.ExecutedAt.After(*stats.LastExecutionTime) {
				stats.LastExecutionTime = &exec.ExecutedAt
			}
		}
	}

	if stats.TotalExecutions > 0 {
		stats.AverageResponseTime = totalResponseTime / time.Duration(stats.TotalExecutions)
		stats.UptimePercentage = float64(stats.SuccessfulExecutions) / float64(stats.TotalExecutions) * 100
	}

	return stats, nil
}

// GetSchedulerStats calculates overall scheduler statistics
func (r *MemoryRepository) GetSchedulerStats(ctx context.Context, from, to time.Time) (map[string]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := map[string]interface{}{
		"total_jobs":        0,
		"active_jobs":       0,
		"inactive_jobs":     0,
		"deleted_jobs":      0,
		"total_executions":  0,
		"successful_runs":   0,
		"failed_runs":       0,
		"avg_response_time": time.Duration(0),
		"period_start":      from,
		"period_end":        to,
	}

	activeJobs := 0
	inactiveJobs := 0
	deletedJobs := 0

	for _, job := range r.jobs {
		switch job.Status {
		case models.StatusActive:
			activeJobs++
		case models.StatusInactive:
			inactiveJobs++
		case models.StatusDeleted:
			deletedJobs++
		}
	}

	stats["total_jobs"] = len(r.jobs)
	stats["active_jobs"] = activeJobs
	stats["inactive_jobs"] = inactiveJobs
	stats["deleted_jobs"] = deletedJobs

	// Calculate execution stats within time range
	totalExecs := 0
	successfulExecs := 0
	failedExecs := 0
	var totalResponseTime time.Duration

	for _, exec := range r.executions {
		if exec.ExecutedAt.After(from) && exec.ExecutedAt.Before(to) {
			totalExecs++
			if exec.Success {
				successfulExecs++
			} else {
				failedExecs++
			}
			totalResponseTime += exec.ResponseTime
		}
	}

	stats["total_executions"] = totalExecs
	stats["successful_runs"] = successfulExecs
	stats["failed_runs"] = failedExecs

	if totalExecs > 0 {
		stats["avg_response_time"] = totalResponseTime / time.Duration(totalExecs)
		stats["success_rate"] = float64(successfulExecs) / float64(totalExecs) * 100
	}

	return stats, nil
}

// GetActiveJobs retrieves all active jobs for scheduling
func (r *MemoryRepository) GetActiveJobs(ctx context.Context) ([]*models.Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var activeJobs []*models.Job
	for _, job := range r.jobs {
		if job.Status == models.StatusActive {
			jobCopy := *job
			activeJobs = append(activeJobs, &jobCopy)
		}
	}

	return activeJobs, nil
}

// UpdateJobNextRun updates the next run time for a job
func (r *MemoryRepository) UpdateJobNextRun(ctx context.Context, jobID string, nextRun time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.NextRun = &nextRun
	job.UpdatedAt = time.Now()

	return nil
}

// UpdateJobLastExecuted updates the last execution time for a job
func (r *MemoryRepository) UpdateJobLastExecuted(ctx context.Context, jobID string, lastExecuted time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	job, exists := r.jobs[jobID]
	if !exists {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job.LastExecuted = &lastExecuted
	job.UpdatedAt = time.Now()

	return nil
}
