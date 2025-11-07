package models

import (
	"time"
)

// JobType represents the execution guarantee type
type JobType string

const (
	AtLeastOnce JobType = "ATLEAST_ONCE"
	AtMostOnce  JobType = "ATMOST_ONCE"
)

// JobStatus represents the current status of a job
type JobStatus string

const (
	StatusActive   JobStatus = "ACTIVE"
	StatusInactive JobStatus = "INACTIVE"
	StatusDeleted  JobStatus = "DELETED"
)

// Job represents a scheduled job
type Job struct {
	ID           string                 `json:"id"`
	Schedule     string                 `json:"schedule"` // Extended CRON expression with seconds
	API          string                 `json:"api"`      // Target API endpoint
	Type         JobType                `json:"type"`     // Execution guarantee type
	Status       JobStatus              `json:"status"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	LastExecuted *time.Time             `json:"last_executed,omitempty"`
	NextRun      *time.Time             `json:"next_run,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"` // Additional job metadata
}

// JobExecution represents a single execution of a job
type JobExecution struct {
	ID           string        `json:"id"`
	JobID        string        `json:"job_id"`
	ExecutedAt   time.Time     `json:"executed_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	HTTPStatus   int           `json:"http_status"`
	ResponseTime time.Duration `json:"response_time"` // in milliseconds
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	RetryCount   int           `json:"retry_count"`
	RequestBody  string        `json:"request_body,omitempty"`
	ResponseBody string        `json:"response_body,omitempty"`
}

// JobRequest represents the request to create a new job
type JobRequest struct {
	Schedule string                 `json:"schedule" binding:"required"`
	API      string                 `json:"api" binding:"required,url"`
	Type     JobType                `json:"type" binding:"required,oneof=ATLEAST_ONCE ATMOST_ONCE"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// JobResponse represents the response for job creation
type JobResponse struct {
	JobID     string    `json:"job_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// ExecutionListResponse represents the response for execution history
type ExecutionListResponse struct {
	JobID      string         `json:"job_id"`
	Executions []JobExecution `json:"executions"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
}

// JobStats represents job statistics for observability
type JobStats struct {
	JobID                string        `json:"job_id"`
	TotalExecutions      int64         `json:"total_executions"`
	SuccessfulExecutions int64         `json:"successful_executions"`
	FailedExecutions     int64         `json:"failed_executions"`
	AverageResponseTime  time.Duration `json:"average_response_time"`
	LastExecutionTime    *time.Time    `json:"last_execution_time,omitempty"`
	NextScheduledTime    *time.Time    `json:"next_scheduled_time,omitempty"`
	UptimePercentage     float64       `json:"uptime_percentage"`
}
