package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"job-scheduler-fampay/models"
	"job-scheduler-fampay/repository"
)

// Set Gin to test mode
func init() {
	gin.SetMode(gin.TestMode)
}

// MockRepository for testing
type MockRepository struct {
	jobs       map[string]*models.Job
	executions map[string][]*models.JobExecution
	err        error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		jobs:       make(map[string]*models.Job),
		executions: make(map[string][]*models.JobExecution),
	}
}

func (m *MockRepository) CreateJob(ctx context.Context, job *models.Job) error {
	if m.err != nil {
		return m.err
	}
	if job.ID == "" {
		job.ID = "test-job-id"
	}
	job.CreatedAt = time.Now()
	job.UpdatedAt = time.Now()
	job.Status = models.StatusActive
	m.jobs[job.ID] = job
	return nil
}

func (m *MockRepository) GetJob(ctx context.Context, jobID string) (*models.Job, error) {
	if m.err != nil {
		return nil, m.err
	}
	job, exists := m.jobs[jobID]
	if !exists {
		return nil, repository.ErrJobNotFound
	}
	return job, nil
}

func (m *MockRepository) GetAllJobs(ctx context.Context, limit, offset int, status *models.JobStatus) ([]*models.Job, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}

	var result []*models.Job
	for _, job := range m.jobs {
		if status == nil || job.Status == *status {
			result = append(result, job)
		}
	}

	total := len(result)

	// Apply pagination
	start := offset
	if start > len(result) {
		start = len(result)
	}
	end := start + limit
	if end > len(result) {
		end = len(result)
	}

	return result[start:end], total, nil
}

func (m *MockRepository) UpdateJob(ctx context.Context, jobID string, updates map[string]interface{}) error {
	if m.err != nil {
		return m.err
	}
	job, exists := m.jobs[jobID]
	if !exists {
		return repository.ErrJobNotFound
	}

	// Apply updates
	if schedule, ok := updates["schedule"].(string); ok {
		job.Schedule = schedule
	}
	if api, ok := updates["api"].(string); ok {
		job.API = api
	}
	if status, ok := updates["status"].(models.JobStatus); ok {
		job.Status = status
	}

	job.UpdatedAt = time.Now()
	return nil
}

func (m *MockRepository) DeleteJob(ctx context.Context, jobID string) error {
	if m.err != nil {
		return m.err
	}
	job, exists := m.jobs[jobID]
	if !exists {
		return repository.ErrJobNotFound
	}
	job.Status = models.StatusDeleted
	job.UpdatedAt = time.Now()
	return nil
}

func (m *MockRepository) GetJobExecutions(ctx context.Context, jobID string, limit, offset int) ([]*models.JobExecution, int, error) {
	if m.err != nil {
		return nil, 0, m.err
	}

	if _, exists := m.jobs[jobID]; !exists {
		return nil, 0, repository.ErrJobNotFound
	}

	execs := m.executions[jobID]
	total := len(execs)

	// Apply pagination
	start := offset
	if start > len(execs) {
		start = len(execs)
	}
	end := start + limit
	if end > len(execs) {
		end = len(execs)
	}

	return execs[start:end], total, nil
}

func (m *MockRepository) GetJobStats(ctx context.Context, jobID string) (*models.JobStats, error) {
	if m.err != nil {
		return nil, m.err
	}

	if _, exists := m.jobs[jobID]; !exists {
		return nil, repository.ErrJobNotFound
	}

	execs := m.executions[jobID]
	stats := &models.JobStats{
		JobID:           jobID,
		TotalExecutions: int64(len(execs)),
	}

	for _, exec := range execs {
		if exec.Success {
			stats.SuccessfulExecutions++
		} else {
			stats.FailedExecutions++
		}
	}

	if stats.TotalExecutions > 0 {
		stats.UptimePercentage = float64(stats.SuccessfulExecutions) / float64(stats.TotalExecutions) * 100
	}

	return stats, nil
}

func (m *MockRepository) GetSchedulerStats(ctx context.Context, from, to time.Time) (map[string]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}

	activeJobs := 0
	for _, job := range m.jobs {
		if job.Status == models.StatusActive {
			activeJobs++
		}
	}

	return map[string]interface{}{
		"total_jobs":       len(m.jobs),
		"active_jobs":      activeJobs,
		"total_executions": 0,
		"period_start":     from,
		"period_end":       to,
	}, nil
}

// Implement remaining interface methods
func (m *MockRepository) CreateExecution(ctx context.Context, execution *models.JobExecution) error {
	return nil
}
func (m *MockRepository) UpdateExecution(ctx context.Context, executionID string, updates map[string]interface{}) error {
	return nil
}
func (m *MockRepository) GetActiveJobs(ctx context.Context) ([]*models.Job, error) { return nil, nil }
func (m *MockRepository) UpdateJobNextRun(ctx context.Context, jobID string, nextRun time.Time) error {
	return nil
}
func (m *MockRepository) UpdateJobLastExecuted(ctx context.Context, jobID string, lastExecuted time.Time) error {
	return nil
}

// Test helpers
func setupTestRouter(repo repository.JobRepository) *gin.Engine {
	handler := NewHandler(nil, repo)
	router := gin.New()
	router.GET("/healthz", handler.Health)
	router.POST("/api/v1/jobs", handler.CreateJob)
	router.GET("/api/v1/jobs", handler.GetAllJobs)
	router.GET("/api/v1/jobs/:id", handler.GetJob)
	router.PATCH("/api/v1/jobs/:id", handler.UpdateJob)
	router.DELETE("/api/v1/jobs/:id", handler.DeleteJob)
	router.GET("/api/v1/jobs/:id/executions", handler.GetJobExecutions)
	router.GET("/api/v1/jobs/:id/stats", handler.GetJobStats)
	router.GET("/api/v1/stats", handler.GetSchedulerStats)
	return router
}

// Tests

func TestHealth(t *testing.T) {
	router := setupTestRouter(NewMockRepository())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/healthz", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}

	if _, ok := response["timestamp"]; !ok {
		t.Error("Expected timestamp in response")
	}
}

func TestCreateJob_Success(t *testing.T) {
	repo := NewMockRepository()
	router := setupTestRouter(repo)

	payload := models.JobRequest{
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
	}

	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/jobs", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	var response models.JobResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	if response.JobID == "" {
		t.Error("Expected job ID in response")
	}

	if response.Message != "Job created successfully" {
		t.Errorf("Unexpected message: %s", response.Message)
	}
}

func TestCreateJob_InvalidCron(t *testing.T) {
	repo := NewMockRepository()
	router := setupTestRouter(repo)

	payload := models.JobRequest{
		Schedule: "invalid-cron",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
	}

	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/jobs", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["error"] != "Invalid CRON expression" {
		t.Errorf("Expected CRON error, got %v", response["error"])
	}
}

func TestCreateJob_MissingFields(t *testing.T) {
	repo := NewMockRepository()
	router := setupTestRouter(repo)

	// Missing API field
	payload := map[string]interface{}{
		"schedule": "*/5 * * * * *",
		"type":     "ATLEAST_ONCE",
	}

	body, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/jobs", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestGetJob_Success(t *testing.T) {
	repo := NewMockRepository()

	// Add a test job
	job := &models.Job{
		ID:       "test-job-1",
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
		Status:   models.StatusActive,
	}
	repo.jobs[job.ID] = job

	router := setupTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jobs/test-job-1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response models.Job
	json.Unmarshal(w.Body.Bytes(), &response)

	if response.ID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, response.ID)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	repo := NewMockRepository()
	router := setupTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jobs/non-existent", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestGetAllJobs(t *testing.T) {
	repo := NewMockRepository()

	// Add test jobs
	for i := 0; i < 3; i++ {
		job := &models.Job{
			ID:       string(rune('a' + i)),
			Schedule: "*/5 * * * * *",
			API:      "https://example.com/webhook",
			Type:     models.AtLeastOnce,
			Status:   models.StatusActive,
		}
		repo.jobs[job.ID] = job
	}

	router := setupTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jobs", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if int(response["total"].(float64)) != 3 {
		t.Errorf("Expected 3 jobs, got %v", response["total"])
	}
}

func TestUpdateJob_Success(t *testing.T) {
	repo := NewMockRepository()

	// Add a test job
	job := &models.Job{
		ID:       "test-job-1",
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
		Status:   models.StatusActive,
	}
	repo.jobs[job.ID] = job

	router := setupTestRouter(repo)

	updates := map[string]interface{}{
		"schedule": "*/10 * * * * *",
	}

	body, _ := json.Marshal(updates)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/jobs/test-job-1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify job was updated
	if repo.jobs[job.ID].Schedule != "*/10 * * * * *" {
		t.Error("Job schedule was not updated")
	}
}

func TestDeleteJob_Success(t *testing.T) {
	repo := NewMockRepository()

	// Add a test job
	job := &models.Job{
		ID:       "test-job-1",
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
		Status:   models.StatusActive,
	}
	repo.jobs[job.ID] = job

	router := setupTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/jobs/test-job-1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify job was soft deleted
	if repo.jobs[job.ID].Status != models.StatusDeleted {
		t.Error("Job was not marked as deleted")
	}
}

func TestGetJobExecutions(t *testing.T) {
	repo := NewMockRepository()

	// Add a test job
	job := &models.Job{
		ID:       "test-job-1",
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
		Status:   models.StatusActive,
	}
	repo.jobs[job.ID] = job

	// Add executions
	now := time.Now()
	repo.executions[job.ID] = []*models.JobExecution{
		{
			JobID:        job.ID,
			ExecutedAt:   now,
			HTTPStatus:   200,
			ResponseTime: 150 * time.Millisecond,
			Success:      true,
		},
	}

	router := setupTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jobs/test-job-1/executions", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if response["job_id"] != job.ID {
		t.Errorf("Expected job_id %s, got %v", job.ID, response["job_id"])
	}

	if int(response["count"].(float64)) != 1 {
		t.Errorf("Expected 1 execution, got %v", response["count"])
	}
}

func TestGetJobStats(t *testing.T) {
	repo := NewMockRepository()

	// Add a test job
	job := &models.Job{
		ID:       "test-job-1",
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
		Status:   models.StatusActive,
	}
	repo.jobs[job.ID] = job

	// Add executions
	repo.executions[job.ID] = []*models.JobExecution{
		{JobID: job.ID, Success: true},
		{JobID: job.ID, Success: false},
	}

	router := setupTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/jobs/test-job-1/stats", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var stats models.JobStats
	json.Unmarshal(w.Body.Bytes(), &stats)

	if stats.TotalExecutions != 2 {
		t.Errorf("Expected 2 total executions, got %d", stats.TotalExecutions)
	}

	if stats.SuccessfulExecutions != 1 {
		t.Errorf("Expected 1 successful execution, got %d", stats.SuccessfulExecutions)
	}
}

func TestGetSchedulerStats(t *testing.T) {
	repo := NewMockRepository()

	// Add test jobs
	job := &models.Job{
		ID:       "test-job-1",
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
		Status:   models.StatusActive,
	}
	repo.jobs[job.ID] = job

	router := setupTestRouter(repo)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stats", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)

	if int(response["total_jobs"].(float64)) != 1 {
		t.Errorf("Expected 1 total job, got %v", response["total_jobs"])
	}

	if int(response["active_jobs"].(float64)) != 1 {
		t.Errorf("Expected 1 active job, got %v", response["active_jobs"])
	}
}
