package api

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"job-scheduler-fampay/cron"
	"job-scheduler-fampay/models"
	"job-scheduler-fampay/repository"
)

// Handler exposes HTTP endpoints for the API service.
type Handler struct {
	logger     *log.Logger
	repo       repository.JobRepository
	cronParser *cron.Parser
}

// NewHandler constructs a new Handler instance.
func NewHandler(logger *log.Logger, repo repository.JobRepository) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	if repo == nil {
		repo = repository.NewMemoryRepository()
	}
	return &Handler{
		logger:     logger,
		repo:       repo,
		cronParser: cron.NewParser(),
	}
}

// Health returns the current health status of the service.
func (h *Handler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
	})
}

// CreateJob creates a new scheduled job
func (h *Handler) CreateJob(c *gin.Context) {
	var req models.JobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Printf("Invalid request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Validate CRON expression
	if err := h.cronParser.Validate(req.Schedule); err != nil {
		h.logger.Printf("Invalid CRON expression: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid CRON expression",
			"details": err.Error(),
		})
		return
	}

	// Calculate next run time
	nextRun, err := h.cronParser.GetNextRun(req.Schedule, time.Now())
	if err != nil {
		h.logger.Printf("Failed to calculate next run: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to calculate next run time",
		})
		return
	}

	// Create job
	job := &models.Job{
		Schedule: req.Schedule,
		API:      req.API,
		Type:     req.Type,
		Metadata: req.Metadata,
		NextRun:  &nextRun,
	}

	if err := h.repo.CreateJob(c.Request.Context(), job); err != nil {
		h.logger.Printf("Failed to create job: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create job",
		})
		return
	}

	h.logger.Printf("Job created: %s", job.ID)
	c.JSON(http.StatusCreated, models.JobResponse{
		JobID:     job.ID,
		Message:   "Job created successfully",
		CreatedAt: job.CreatedAt,
	})
}

// GetAllJobs retrieves all jobs with pagination
func (h *Handler) GetAllJobs(c *gin.Context) {
	// Parse pagination parameters
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid 'limit' parameter",
		})
		return
	}

	offsetStr := c.DefaultQuery("offset", "0")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid 'offset' parameter",
		})
		return
	}

	// Parse optional status filter
	var statusFilter *models.JobStatus
	if status := c.Query("status"); status != "" {
		s := models.JobStatus(status)
		statusFilter = &s
	}

	jobs, total, err := h.repo.GetAllJobs(c.Request.Context(), limit, offset, statusFilter)
	if err != nil {
		h.logger.Printf("Failed to get jobs: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve jobs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":   jobs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetJob retrieves a specific job by ID
func (h *Handler) GetJob(c *gin.Context) {
	jobID := c.Param("id")

	job, err := h.repo.GetJob(c.Request.Context(), jobID)
	if err != nil {
		h.logger.Printf("Failed to get job %s: %v", jobID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	c.JSON(http.StatusOK, job)
}

// UpdateJob updates an existing job
func (h *Handler) UpdateJob(c *gin.Context) {
	jobID := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Validate CRON expression if provided
	if schedule, ok := updates["schedule"].(string); ok {
		if err := h.cronParser.Validate(schedule); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid CRON expression",
				"details": err.Error(),
			})
			return
		}

		// Update next run time
		nextRun, _ := h.cronParser.GetNextRun(schedule, time.Now())
		h.repo.UpdateJobNextRun(c.Request.Context(), jobID, nextRun)
	}

	if err := h.repo.UpdateJob(c.Request.Context(), jobID, updates); err != nil {
		h.logger.Printf("Failed to update job %s: %v", jobID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Job updated successfully",
		"job_id":  jobID,
	})
}

// DeleteJob deletes a job
func (h *Handler) DeleteJob(c *gin.Context) {
	jobID := c.Param("id")

	if err := h.repo.DeleteJob(c.Request.Context(), jobID); err != nil {
		h.logger.Printf("Failed to delete job %s: %v", jobID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	h.logger.Printf("Job deleted: %s", jobID)
	c.JSON(http.StatusOK, gin.H{
		"message": "Job deleted successfully",
		"job_id":  jobID,
	})
}

// GetJobExecutions retrieves execution history for a job
func (h *Handler) GetJobExecutions(c *gin.Context) {
	jobID := c.Param("id")

	// Parse pagination parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	executions, total, err := h.repo.GetJobExecutions(c.Request.Context(), jobID, limit, offset)
	if err != nil {
		h.logger.Printf("Failed to get executions for job %s: %v", jobID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	// Convert []*JobExecution to []JobExecution
	executionList := make([]models.JobExecution, len(executions))
	for i, exec := range executions {
		if exec != nil {
			executionList[i] = *exec
		}
	}

	page := offset/limit + 1
	c.JSON(http.StatusOK, models.ExecutionListResponse{
		JobID:      jobID,
		Executions: executionList,
		Total:      total,
		Page:       page,
		PageSize:   limit,
	})
}

// GetJobStats retrieves statistics for a specific job
func (h *Handler) GetJobStats(c *gin.Context) {
	jobID := c.Param("id")

	stats, err := h.repo.GetJobStats(c.Request.Context(), jobID)
	if err != nil {
		h.logger.Printf("Failed to get stats for job %s: %v", jobID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Job not found",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetSchedulerStats retrieves overall scheduler statistics
func (h *Handler) GetSchedulerStats(c *gin.Context) {
	// Parse time range
	fromStr := c.DefaultQuery("from", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid 'from' time format",
		})
		return
	}

	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid 'to' time format",
		})
		return
	}

	stats, err := h.repo.GetSchedulerStats(c.Request.Context(), from, to)
	if err != nil {
		h.logger.Printf("Failed to get scheduler stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve scheduler statistics",
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}
