package executor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"job-scheduler-fampay/models"
)

// MockRepository implements the repository interface for testing
type MockRepository struct {
	mu         sync.Mutex
	executions []*models.JobExecution
	jobs       map[string]*models.Job

	createExecutionCalled    bool
	updateLastExecutedCalled bool
	lastExecutedJobID        string
	lastExecutedTime         time.Time
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		jobs: make(map[string]*models.Job),
	}
}

func (m *MockRepository) CreateJob(ctx context.Context, job *models.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs[job.ID] = job
	return nil
}

func (m *MockRepository) GetJob(ctx context.Context, jobID string) (*models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobs[jobID], nil
}

func (m *MockRepository) GetAllJobs(ctx context.Context, limit, offset int, status *models.JobStatus) ([]*models.Job, int, error) {
	return nil, 0, nil
}

func (m *MockRepository) UpdateJob(ctx context.Context, jobID string, updates map[string]interface{}) error {
	return nil
}

func (m *MockRepository) DeleteJob(ctx context.Context, jobID string) error {
	return nil
}

func (m *MockRepository) CreateExecution(ctx context.Context, execution *models.JobExecution) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createExecutionCalled = true
	m.executions = append(m.executions, execution)
	return nil
}

func (m *MockRepository) GetJobExecutions(ctx context.Context, jobID string, limit, offset int) ([]*models.JobExecution, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*models.JobExecution
	for _, exec := range m.executions {
		if exec.JobID == jobID {
			result = append(result, exec)
		}
	}
	return result, len(result), nil
}

func (m *MockRepository) UpdateExecution(ctx context.Context, executionID string, updates map[string]interface{}) error {
	return nil
}

func (m *MockRepository) GetJobStats(ctx context.Context, jobID string) (*models.JobStats, error) {
	return nil, nil
}

func (m *MockRepository) GetSchedulerStats(ctx context.Context, from, to time.Time) (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockRepository) GetActiveJobs(ctx context.Context) ([]*models.Job, error) {
	return nil, nil
}

func (m *MockRepository) UpdateJobNextRun(ctx context.Context, jobID string, nextRun time.Time) error {
	return nil
}

func (m *MockRepository) UpdateJobLastExecuted(ctx context.Context, jobID string, lastExecuted time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateLastExecutedCalled = true
	m.lastExecutedJobID = jobID
	m.lastExecutedTime = lastExecuted
	return nil
}

func TestJobExecutorExecuteJob_Success(t *testing.T) {
	// Setup a test HTTP server with thread-safe variables
	var mu sync.Mutex
	called := false
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		called = true
		receivedMethod = r.Method
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	// Create mock repository
	mockRepo := NewMockRepository()

	// Create executor with short timeout for testing
	cfg := JobExecutorConfig{
		WorkerCount:    2,
		RequestTimeout: 5 * time.Second,
		MaxQueueSize:   10,
	}
	executor := NewJobExecutor(mockRepo, cfg)

	ctx := context.Background()
	executor.Start(ctx)
	defer executor.Stop()

	// Create a test job
	job := &models.Job{
		ID:       "test-job-1",
		Schedule: "* * * * * *",
		API:      server.URL,
		Type:     models.AtLeastOnce,
		Status:   models.StatusActive,
	}

	// Submit job for execution
	executor.Submit(job)

	// Wait for execution to complete
	time.Sleep(100 * time.Millisecond)

	// Verify HTTP endpoint was called (with lock)
	mu.Lock()
	if !called {
		t.Error("Expected HTTP endpoint to be called")
	}
	if receivedMethod != "POST" {
		t.Errorf("Expected POST request, got %s", receivedMethod)
	}
	mu.Unlock()

	// Verify execution was recorded (with lock)
	mockRepo.mu.Lock()
	if !mockRepo.createExecutionCalled {
		t.Error("Expected CreateExecution to be called")
	}
	mockRepo.mu.Unlock()

	executions, _, _ := mockRepo.GetJobExecutions(ctx, job.ID, 10, 0)
	if len(executions) != 1 {
		t.Fatalf("Expected 1 execution, got %d", len(executions))
	}

	exec := executions[0]
	if exec.JobID != job.ID {
		t.Errorf("Expected JobID %s, got %s", job.ID, exec.JobID)
	}
	if exec.HTTPStatus != http.StatusOK {
		t.Errorf("Expected HTTP status 200, got %d", exec.HTTPStatus)
	}
	if !exec.Success {
		t.Error("Expected execution to be successful")
	}
	if exec.ResponseBody != `{"success":true}` {
		t.Errorf("Expected response body to be captured, got %s", exec.ResponseBody)
	}

	// Verify last executed was updated (with lock)
	mockRepo.mu.Lock()
	if !mockRepo.updateLastExecutedCalled {
		t.Error("Expected UpdateJobLastExecuted to be called")
	}
	if mockRepo.lastExecutedJobID != job.ID {
		t.Errorf("Expected last executed job ID %s, got %s", job.ID, mockRepo.lastExecutedJobID)
	}
	mockRepo.mu.Unlock()
}

func TestJobExecutorExecuteJob_Failure(t *testing.T) {
	// Setup a test HTTP server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	// Create mock repository
	mockRepo := NewMockRepository()

	// Create executor
	cfg := JobExecutorConfig{
		WorkerCount:    1,
		RequestTimeout: 5 * time.Second,
		MaxQueueSize:   10,
	}
	executor := NewJobExecutor(mockRepo, cfg)

	ctx := context.Background()
	executor.Start(ctx)
	defer executor.Stop()

	// Create a test job
	job := &models.Job{
		ID:       "test-job-2",
		Schedule: "* * * * * *",
		API:      server.URL,
		Type:     models.AtMostOnce,
		Status:   models.StatusActive,
	}

	// Submit job for execution
	executor.Submit(job)

	// Wait for execution to complete
	time.Sleep(100 * time.Millisecond)

	// Verify execution was recorded as failure
	executions, _, _ := mockRepo.GetJobExecutions(ctx, job.ID, 10, 0)
	if len(executions) != 1 {
		t.Fatalf("Expected 1 execution, got %d", len(executions))
	}

	exec := executions[0]
	if exec.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("Expected HTTP status 500, got %d", exec.HTTPStatus)
	}
	if exec.Success {
		t.Error("Expected execution to be failed")
	}
	if exec.Error == "" {
		t.Error("Expected error message to be set")
	}
}

func TestJobExecutorExecuteJob_Timeout(t *testing.T) {
	// Setup a slow HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create mock repository
	mockRepo := NewMockRepository()

	// Create executor with very short timeout
	cfg := JobExecutorConfig{
		WorkerCount:    1,
		RequestTimeout: 100 * time.Millisecond, // Very short timeout
		MaxQueueSize:   10,
	}
	executor := NewJobExecutor(mockRepo, cfg)

	ctx := context.Background()
	executor.Start(ctx)
	defer executor.Stop()

	// Create a test job
	job := &models.Job{
		ID:       "test-job-3",
		Schedule: "* * * * * *",
		API:      server.URL,
		Type:     models.AtLeastOnce,
		Status:   models.StatusActive,
	}

	// Submit job for execution
	executor.Submit(job)

	// Wait for timeout to occur
	time.Sleep(500 * time.Millisecond)

	// Verify execution was recorded as failure due to timeout
	executions, _, _ := mockRepo.GetJobExecutions(ctx, job.ID, 10, 0)
	if len(executions) != 1 {
		t.Fatalf("Expected 1 execution, got %d", len(executions))
	}

	exec := executions[0]
	if exec.Success {
		t.Error("Expected execution to fail due to timeout")
	}
	if exec.Error == "" {
		t.Error("Expected timeout error message")
	}
}

func TestJobExecutorWorkerPool(t *testing.T) {
	// Test that multiple workers can process jobs concurrently
	jobCount := 10
	var processedCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Increment counter atomically
		atomic.AddInt32(&processedCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mockRepo := NewMockRepository()

	cfg := JobExecutorConfig{
		WorkerCount:    5, // 5 workers
		RequestTimeout: 5 * time.Second,
		MaxQueueSize:   20,
	}
	executor := NewJobExecutor(mockRepo, cfg)

	ctx := context.Background()
	executor.Start(ctx)
	defer executor.Stop()

	// Submit multiple jobs
	for i := 0; i < jobCount; i++ {
		job := &models.Job{
			ID:       string(rune('a' + i)),
			Schedule: "* * * * * *",
			API:      server.URL,
			Type:     models.AtLeastOnce,
			Status:   models.StatusActive,
		}
		executor.Submit(job)
	}

	// Wait for all jobs to be processed with polling
	maxWait := time.Now().Add(2 * time.Second)
	for time.Now().Before(maxWait) {
		count := atomic.LoadInt32(&processedCount)
		if count >= int32(jobCount) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify all jobs were executed
	finalCount := atomic.LoadInt32(&processedCount)
	if finalCount != int32(jobCount) {
		t.Errorf("Expected %d jobs to be processed, got %d", jobCount, finalCount)
	}
}

func TestJobExecutorStartStop(t *testing.T) {
	mockRepo := NewMockRepository()
	cfg := DefaultJobExecutorConfig()
	executor := NewJobExecutor(mockRepo, cfg)

	ctx := context.Background()

	// Test multiple starts (should be idempotent)
	executor.Start(ctx)
	executor.Start(ctx) // Should not panic

	// Test stop
	executor.Stop()

	// Test multiple stops (should be idempotent)
	executor.Stop() // Should not panic
}

func TestJobExecutorSubmitNilJob(t *testing.T) {
	mockRepo := NewMockRepository()
	cfg := DefaultJobExecutorConfig()
	executor := NewJobExecutor(mockRepo, cfg)

	ctx := context.Background()
	executor.Start(ctx)
	defer executor.Stop()

	// Should not panic when submitting nil job
	executor.Submit(nil)

	// Wait a bit to ensure no crash
	time.Sleep(50 * time.Millisecond)
}
