package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"job-scheduler-fampay/models"
)

// MockExecutor tracks jobs submitted for execution
type MockExecutor struct {
	mu        sync.Mutex
	submitted []*models.Job
}

func (m *MockExecutor) Submit(job *models.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.submitted = append(m.submitted, job)
}

func (m *MockExecutor) GetSubmitted() []*models.Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]*models.Job{}, m.submitted...)
}

// MockRepository provides a test implementation of JobRepository
type MockRepository struct {
	mu              sync.Mutex
	jobs            []*models.Job
	nextRunUpdates  map[string]time.Time
	activeJobsError error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		jobs:           []*models.Job{},
		nextRunUpdates: make(map[string]time.Time),
	}
}

func (m *MockRepository) AddJob(job *models.Job) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, job)
}

func (m *MockRepository) GetActiveJobs(ctx context.Context) ([]*models.Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.activeJobsError != nil {
		return nil, m.activeJobsError
	}

	var active []*models.Job
	for _, job := range m.jobs {
		if job.Status == models.StatusActive {
			active = append(active, job)
		}
	}
	return active, nil
}

func (m *MockRepository) UpdateJobNextRun(ctx context.Context, jobID string, nextRun time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextRunUpdates[jobID] = nextRun

	// Update the job's NextRun in our mock storage
	for _, job := range m.jobs {
		if job.ID == jobID {
			job.NextRun = &nextRun
			break
		}
	}
	return nil
}

// Implement remaining interface methods (not used in tests but required)
func (m *MockRepository) CreateJob(ctx context.Context, job *models.Job) error { return nil }
func (m *MockRepository) GetJob(ctx context.Context, jobID string) (*models.Job, error) {
	return nil, nil
}
func (m *MockRepository) GetAllJobs(ctx context.Context, limit, offset int, status *models.JobStatus) ([]*models.Job, int, error) {
	return nil, 0, nil
}
func (m *MockRepository) UpdateJob(ctx context.Context, jobID string, updates map[string]interface{}) error {
	return nil
}
func (m *MockRepository) DeleteJob(ctx context.Context, jobID string) error { return nil }
func (m *MockRepository) CreateExecution(ctx context.Context, execution *models.JobExecution) error {
	return nil
}
func (m *MockRepository) GetJobExecutions(ctx context.Context, jobID string, limit, offset int) ([]*models.JobExecution, int, error) {
	return nil, 0, nil
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
func (m *MockRepository) UpdateJobLastExecuted(ctx context.Context, jobID string, lastExecuted time.Time) error {
	return nil
}

func TestSchedulerDispatchDueJobs(t *testing.T) {
	mockRepo := NewMockRepository()
	mockExecutor := &MockExecutor{}

	// Create scheduler with fast polling for testing
	cfg := Config{
		PollInterval: 50 * time.Millisecond,
	}
	scheduler := New(mockRepo, mockExecutor, nil, cfg)

	// Add a job that's already due
	now := time.Now()
	pastTime := now.Add(-1 * time.Second)
	dueJob := &models.Job{
		ID:       "job-1",
		Schedule: "* * * * * *", // Every second
		API:      "http://example.com/webhook",
		Status:   models.StatusActive,
		NextRun:  &pastTime,
	}
	mockRepo.AddJob(dueJob)

	// Add a job that's not due yet
	futureTime := now.Add(1 * time.Hour)
	futureJob := &models.Job{
		ID:       "job-2",
		Schedule: "0 0 * * * *", // Every hour
		API:      "http://example.com/webhook2",
		Status:   models.StatusActive,
		NextRun:  &futureTime,
	}
	mockRepo.AddJob(futureJob)

	// Add an inactive job that should not be dispatched
	inactiveJob := &models.Job{
		ID:       "job-3",
		Schedule: "* * * * * *",
		API:      "http://example.com/webhook3",
		Status:   models.StatusInactive,
		NextRun:  &pastTime,
	}
	mockRepo.AddJob(inactiveJob)

	// Start scheduler
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	scheduler.Start(ctx)

	// Wait for exactly one poll cycle
	time.Sleep(60 * time.Millisecond)

	scheduler.Stop()

	// Verify only the due active job was submitted
	submitted := mockExecutor.GetSubmitted()
	if len(submitted) != 1 {
		t.Fatalf("Expected 1 job to be submitted, got %d", len(submitted))
	}

	if submitted[0].ID != "job-1" {
		t.Errorf("Expected job-1 to be submitted, got %s", submitted[0].ID)
	}

	// Verify next run was updated for the dispatched job
	if nextRun, ok := mockRepo.nextRunUpdates["job-1"]; !ok {
		t.Error("Expected next run to be updated for job-1")
	} else if nextRun.Before(now) || nextRun.Equal(now) {
		t.Error("Expected next run to be in the future")
	}

	// Verify future job was not submitted
	for _, job := range submitted {
		if job.ID == "job-2" {
			t.Error("Future job should not have been submitted")
		}
	}

	// Verify inactive job was not submitted
	for _, job := range submitted {
		if job.ID == "job-3" {
			t.Error("Inactive job should not have been submitted")
		}
	}
}

func TestSchedulerMultipleDispatch(t *testing.T) {
	mockRepo := NewMockRepository()
	mockExecutor := &MockExecutor{}

	cfg := Config{
		PollInterval: 50 * time.Millisecond,
	}
	scheduler := New(mockRepo, mockExecutor, nil, cfg)

	// Add multiple due jobs
	now := time.Now()
	pastTime := now.Add(-1 * time.Second)

	for i := 0; i < 5; i++ {
		job := &models.Job{
			ID:       string(rune('a' + i)),
			Schedule: "* * * * * *",
			API:      "http://example.com/webhook",
			Status:   models.StatusActive,
			NextRun:  &pastTime,
		}
		mockRepo.AddJob(job)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	scheduler.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	scheduler.Stop()

	// All 5 jobs should be submitted
	submitted := mockExecutor.GetSubmitted()
	if len(submitted) != 5 {
		t.Fatalf("Expected 5 jobs to be submitted, got %d", len(submitted))
	}

	// All jobs should have their next run updated
	if len(mockRepo.nextRunUpdates) != 5 {
		t.Errorf("Expected 5 next run updates, got %d", len(mockRepo.nextRunUpdates))
	}
}

func TestSchedulerStartStop(t *testing.T) {
	mockRepo := NewMockRepository()
	mockExecutor := &MockExecutor{}

	cfg := Config{
		PollInterval: 100 * time.Millisecond,
	}
	scheduler := New(mockRepo, mockExecutor, nil, cfg)

	ctx := context.Background()

	// Test multiple starts (should be idempotent)
	scheduler.Start(ctx)
	scheduler.Start(ctx) // Should not panic

	// Test stop
	scheduler.Stop()

	// Test multiple stops (should be idempotent)
	scheduler.Stop() // Should not panic
}

func TestSchedulerRepositoryError(t *testing.T) {
	mockRepo := NewMockRepository()
	mockExecutor := &MockExecutor{}

	// Set repository to return error
	mockRepo.activeJobsError = context.DeadlineExceeded

	cfg := Config{
		PollInterval: 50 * time.Millisecond,
	}
	scheduler := New(mockRepo, mockExecutor, nil, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	scheduler.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	scheduler.Stop()

	// No jobs should be submitted when repository errors
	submitted := mockExecutor.GetSubmitted()
	if len(submitted) != 0 {
		t.Errorf("Expected no jobs to be submitted on repository error, got %d", len(submitted))
	}
}

func TestSchedulerNilNextRun(t *testing.T) {
	mockRepo := NewMockRepository()
	mockExecutor := &MockExecutor{}

	cfg := Config{
		PollInterval: 50 * time.Millisecond,
	}
	scheduler := New(mockRepo, mockExecutor, nil, cfg)

	// Add job with nil NextRun (newly created job)
	jobWithoutNextRun := &models.Job{
		ID:       "job-no-next",
		Schedule: "* * * * * *",
		API:      "http://example.com/webhook",
		Status:   models.StatusActive,
		NextRun:  nil, // No next run set yet
	}
	mockRepo.AddJob(jobWithoutNextRun)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	scheduler.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	scheduler.Stop()

	// Job with nil NextRun should not be submitted
	submitted := mockExecutor.GetSubmitted()
	if len(submitted) != 0 {
		t.Errorf("Expected no jobs to be submitted with nil NextRun, got %d", len(submitted))
	}
}

func TestSchedulerImmediateDispatch(t *testing.T) {
	mockRepo := NewMockRepository()
	mockExecutor := &MockExecutor{}

	// Add a due job before starting scheduler
	now := time.Now()
	pastTime := now.Add(-1 * time.Minute)
	dueJob := &models.Job{
		ID:       "immediate-job",
		Schedule: "* * * * * *",
		API:      "http://example.com/webhook",
		Status:   models.StatusActive,
		NextRun:  &pastTime,
	}
	mockRepo.AddJob(dueJob)

	cfg := Config{
		PollInterval: 1 * time.Second, // Long interval to test immediate dispatch
	}
	scheduler := New(mockRepo, mockExecutor, nil, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	scheduler.Start(ctx)

	// Should dispatch immediately, not wait for first tick
	time.Sleep(50 * time.Millisecond)

	scheduler.Stop()

	// Job should be submitted immediately
	submitted := mockExecutor.GetSubmitted()
	if len(submitted) != 1 {
		t.Fatalf("Expected job to be dispatched immediately, got %d submissions", len(submitted))
	}
}
