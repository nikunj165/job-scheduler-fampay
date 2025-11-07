package repository

import (
	"context"
	"reflect"
	"testing"
	"time"

	"job-scheduler-fampay/models"
)

func TestMemoryRepositoryCreateJob(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{
		Schedule: "0 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
	}

	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	if job.ID == "" {
		t.Fatalf("CreateJob() expected generated ID, got empty string")
	}

	if job.Status != models.StatusActive {
		t.Fatalf("CreateJob() expected status %q, got %q", models.StatusActive, job.Status)
	}

	if job.CreatedAt.IsZero() || job.UpdatedAt.IsZero() {
		t.Fatalf("CreateJob() expected timestamps to be set")
	}

	stored, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}

	if stored.Schedule != job.Schedule || stored.API != job.API {
		t.Fatalf("stored job does not match input job")
	}

	if stored.CreatedAt.After(time.Now()) {
		t.Fatalf("stored CreatedAt should not be in the future")
	}
}

func TestMemoryRepositoryCreateJobDuplicate(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{
		ID:       "job_123",
		Schedule: "0 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
	}

	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	dup := &models.Job{
		ID:       "job_123",
		Schedule: "0 * * * * *",
		API:      "https://example.com/webhook",
		Type:     models.AtLeastOnce,
	}

	if err := repo.CreateJob(ctx, dup); err == nil {
		t.Fatalf("CreateJob() expected error for duplicate ID")
	}
}

func TestMemoryRepositoryGetJob(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/job",
		Type:     models.AtLeastOnce,
	}

	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	got, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}

	if got == job {
		t.Fatalf("GetJob() should return a copy, not the stored pointer")
	}

	if got.ID != job.ID || got.Schedule != job.Schedule || got.API != job.API || got.Type != job.Type {
		t.Fatalf("GetJob() returned job does not match stored job")
	}

	// Mutate the returned job and ensure repository data is unchanged.
	got.Schedule = "modified"
	got.Metadata = map[string]interface{}{"foo": "bar"}

	stored, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() unexpected error on re-fetch: %v", err)
	}

	if stored.Schedule != job.Schedule {
		t.Fatalf("Modifying returned job should not affect stored job")
	}

	if stored.Metadata != nil {
		t.Fatalf("Expected stored Metadata to remain nil, got %#v", stored.Metadata)
	}
}

func TestMemoryRepositoryGetJobNotFound(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	if _, err := repo.GetJob(ctx, "missing"); err == nil {
		t.Fatalf("GetJob() expected error for unknown job ID")
	}
}

func TestMemoryRepositoryGetAllJobs(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	jobs := []*models.Job{
		{Schedule: "0 * * * * *", API: "https://example.com/a", Type: models.AtLeastOnce},
		{Schedule: "1 * * * * *", API: "https://example.com/b", Type: models.AtLeastOnce},
		{Schedule: "2 * * * * *", API: "https://example.com/c", Type: models.AtLeastOnce},
	}

	expectedStatuses := make(map[string]models.JobStatus)

	for _, job := range jobs {
		if err := repo.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob() unexpected error: %v", err)
		}
		expectedStatuses[job.ID] = job.Status
	}

	// Update job statuses to cover all possible values.
	if err := repo.UpdateJob(ctx, jobs[1].ID, map[string]interface{}{"status": models.StatusInactive}); err != nil {
		t.Fatalf("UpdateJob() unexpected error: %v", err)
	}
	if err := repo.UpdateJob(ctx, jobs[2].ID, map[string]interface{}{"status": models.StatusDeleted}); err != nil {
		t.Fatalf("UpdateJob() unexpected error: %v", err)
	}

	expectedStatuses[jobs[1].ID] = models.StatusInactive
	expectedStatuses[jobs[2].ID] = models.StatusDeleted

	// No status filter with pagination (limit 2, offset 0).
	result, total, err := repo.GetAllJobs(ctx, 2, 0, nil)
	if err != nil {
		t.Fatalf("GetAllJobs() unexpected error: %v", err)
	}
	if total != len(jobs) {
		t.Fatalf("GetAllJobs() total = %d, want %d", total, len(jobs))
	}
	if len(result) != 2 {
		t.Fatalf("GetAllJobs() expected 2 results for first page, got %d", len(result))
	}
	for _, job := range result {
		if _, ok := expectedStatuses[job.ID]; !ok {
			t.Fatalf("GetAllJobs() returned unexpected job ID %q", job.ID)
		}
	}

	// Second page (limit 2, offset 2) should return the remaining job.
	result, total, err = repo.GetAllJobs(ctx, 2, 2, nil)
	if err != nil {
		t.Fatalf("GetAllJobs() unexpected error on second page: %v", err)
	}
	if total != len(jobs) {
		t.Fatalf("GetAllJobs() total = %d, want %d on second page", total, len(jobs))
	}
	if len(result) != 1 {
		t.Fatalf("GetAllJobs() expected 1 result on second page, got %d", len(result))
	}
	if _, ok := expectedStatuses[result[0].ID]; !ok {
		t.Fatalf("GetAllJobs() returned unexpected job ID %q on second page", result[0].ID)
	}

	// Status filter: inactive.
	inactive := models.StatusInactive
	filtered, total, err := repo.GetAllJobs(ctx, 10, 0, &inactive)
	if err != nil {
		t.Fatalf("GetAllJobs() unexpected error with status filter: %v", err)
	}
	if total != 1 {
		t.Fatalf("GetAllJobs() inactive total = %d, want 1", total)
	}
	if len(filtered) != 1 {
		t.Fatalf("GetAllJobs() expected 1 inactive job, got %d", len(filtered))
	}
	if filtered[0].Status != models.StatusInactive {
		t.Fatalf("GetAllJobs() expected inactive status, got %q", filtered[0].Status)
	}

	// Status filter: deleted.
	deleted := models.StatusDeleted
	filtered, total, err = repo.GetAllJobs(ctx, 10, 0, &deleted)
	if err != nil {
		t.Fatalf("GetAllJobs() unexpected error with deleted filter: %v", err)
	}
	if total != 1 || len(filtered) != 1 {
		t.Fatalf("GetAllJobs() expected 1 deleted job, got total=%d len=%d", total, len(filtered))
	}
	if filtered[0].Status != models.StatusDeleted {
		t.Fatalf("GetAllJobs() expected deleted status, got %q", filtered[0].Status)
	}
}

func TestMemoryRepositoryUpdateJob(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{
		Schedule: "0 * * * * *",
		API:      "https://example.com/original",
		Type:     models.AtLeastOnce,
	}

	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	originalUpdatedAt := job.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	expectedSchedule := "*/15 * * * * *"
	expectedAPI := "https://example.com/updated"
	expectedType := models.AtMostOnce
	expectedStatus := models.StatusInactive
	expectedMetadata := map[string]interface{}{"attempts": 3}

	updates := map[string]interface{}{
		"schedule":      expectedSchedule,
		"api":           expectedAPI,
		"type":          expectedType,
		"status":        expectedStatus,
		"metadata":      expectedMetadata,
		"unknown_field": "ignored",
	}

	if err := repo.UpdateJob(ctx, job.ID, updates); err != nil {
		t.Fatalf("UpdateJob() unexpected error: %v", err)
	}

	updatedJob, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() unexpected error after update: %v", err)
	}

	if updatedJob.Schedule != expectedSchedule {
		t.Fatalf("UpdateJob() schedule = %q, want %q", updatedJob.Schedule, expectedSchedule)
	}
	if updatedJob.API != expectedAPI {
		t.Fatalf("UpdateJob() api = %q, want %q", updatedJob.API, expectedAPI)
	}
	if updatedJob.Type != expectedType {
		t.Fatalf("UpdateJob() type = %q, want %q", updatedJob.Type, expectedType)
	}
	if updatedJob.Status != expectedStatus {
		t.Fatalf("UpdateJob() status = %q, want %q", updatedJob.Status, expectedStatus)
	}
	if !reflect.DeepEqual(updatedJob.Metadata, expectedMetadata) {
		t.Fatalf("UpdateJob() metadata = %#v, want %#v", updatedJob.Metadata, expectedMetadata)
	}
	if !job.UpdatedAt.After(originalUpdatedAt) {
		t.Fatalf("UpdateJob() expected UpdatedAt to be refreshed")
	}

	// Invalid field types should be ignored without panicking.
	if err := repo.UpdateJob(ctx, job.ID, map[string]interface{}{
		"api":      12345,
		"metadata": "invalid",
		"unknown":  true,
	}); err != nil {
		t.Fatalf("UpdateJob() unexpected error when applying invalid updates: %v", err)
	}

	postInvalid, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() unexpected error after invalid updates: %v", err)
	}
	if postInvalid.API != expectedAPI {
		t.Fatalf("UpdateJob() should ignore invalid api updates; got %q", postInvalid.API)
	}
	if !reflect.DeepEqual(postInvalid.Metadata, expectedMetadata) {
		t.Fatalf("UpdateJob() should ignore invalid metadata updates; got %#v", postInvalid.Metadata)
	}

	if err := repo.UpdateJob(ctx, "missing", updates); err == nil {
		t.Fatalf("UpdateJob() expected error for unknown job ID")
	}
}

func TestMemoryRepositoryDeleteJob(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{
		Schedule: "0 * * * * *",
		API:      "https://example.com/delete",
		Type:     models.AtLeastOnce,
	}

	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	originalUpdatedAt := job.UpdatedAt
	time.Sleep(10 * time.Millisecond)

	if err := repo.DeleteJob(ctx, job.ID); err != nil {
		t.Fatalf("DeleteJob() unexpected error: %v", err)
	}

	deletedJob, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() unexpected error after delete: %v", err)
	}

	if deletedJob.Status != models.StatusDeleted {
		t.Fatalf("DeleteJob() status = %q, want %q", deletedJob.Status, models.StatusDeleted)
	}

	if !job.UpdatedAt.After(originalUpdatedAt) {
		t.Fatalf("DeleteJob() expected UpdatedAt to be refreshed")
	}

	if err := repo.DeleteJob(ctx, "missing"); err == nil {
		t.Fatalf("DeleteJob() expected error for unknown job ID")
	}
}

func TestMemoryRepositoryCreateExecution(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/job",
		Type:     models.AtLeastOnce,
	}

	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	exec := &models.JobExecution{
		JobID: job.ID,
	}

	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() unexpected error: %v", err)
	}

	if exec.ID == "" {
		t.Fatalf("CreateExecution() expected generated ID, got empty string")
	}
	if exec.ExecutedAt.IsZero() {
		t.Fatalf("CreateExecution() expected ExecutedAt to be set")
	}

	// Subsequent retrieval should include the execution ID.
	execs, total, err := repo.GetJobExecutions(ctx, job.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetJobExecutions() unexpected error: %v", err)
	}
	if total != 1 || len(execs) != 1 {
		t.Fatalf("GetJobExecutions() expected 1 execution, got total=%d len=%d", total, len(execs))
	}
	if execs[0].ID != exec.ID {
		t.Fatalf("GetJobExecutions() returned execution ID %q, want %q", execs[0].ID, exec.ID)
	}

	// Creating an execution for an unknown job should error.
	badExec := &models.JobExecution{JobID: "missing"}
	if err := repo.CreateExecution(ctx, badExec); err == nil {
		t.Fatalf("CreateExecution() expected error for missing job ID")
	}
}

func TestMemoryRepositoryGetJobExecutions(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{
		Schedule: "*/10 * * * * *",
		API:      "https://example.com/job",
		Type:     models.AtLeastOnce,
	}

	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	// Seed three executions with deterministic timestamps.
	base := time.Now().Add(-time.Minute)
	for i := 0; i < 3; i++ {
		exec := &models.JobExecution{
			JobID: job.ID,
		}
		if err := repo.CreateExecution(ctx, exec); err != nil {
			t.Fatalf("CreateExecution() unexpected error: %v", err)
		}
		// Override execution time so we can reason about ordering.
		exec.ExecutedAt = base.Add(time.Duration(i) * time.Second)
		repo.executions[exec.ID] = exec
	}

	// Page size 2 should return the newest two executions.
	execs, total, err := repo.GetJobExecutions(ctx, job.ID, 2, 0)
	if err != nil {
		t.Fatalf("GetJobExecutions() unexpected error: %v", err)
	}
	if total != 3 {
		t.Fatalf("GetJobExecutions() total = %d, want 3", total)
	}
	if len(execs) != 2 {
		t.Fatalf("GetJobExecutions() expected 2 executions, got %d", len(execs))
	}
	if !execs[0].ExecutedAt.After(execs[1].ExecutedAt) {
		t.Fatalf("GetJobExecutions() expected results in reverse chronological order")
	}

	// Second page (offset 2) should return the oldest execution.
	execs, total, err = repo.GetJobExecutions(ctx, job.ID, 2, 2)
	if err != nil {
		t.Fatalf("GetJobExecutions() unexpected error on second page: %v", err)
	}
	if total != 3 {
		t.Fatalf("GetJobExecutions() total = %d on second page, want 3", total)
	}
	if len(execs) != 1 {
		t.Fatalf("GetJobExecutions() expected 1 execution on second page, got %d", len(execs))
	}

	// Offset beyond available data should return empty slice but keep total.
	execs, total, err = repo.GetJobExecutions(ctx, job.ID, 2, 10)
	if err != nil {
		t.Fatalf("GetJobExecutions() unexpected error with large offset: %v", err)
	}
	if total != 3 {
		t.Fatalf("GetJobExecutions() total = %d with large offset, want 3", total)
	}
	if len(execs) != 0 {
		t.Fatalf("GetJobExecutions() expected empty slice with large offset, got %d", len(execs))
	}

	if _, _, err := repo.GetJobExecutions(ctx, "missing", 10, 0); err == nil {
		t.Fatalf("GetJobExecutions() expected error for unknown job ID")
	}
}

func TestMemoryRepositoryUpdateExecution(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{
		Schedule: "0 * * * * *",
		API:      "https://example.com/job",
		Type:     models.AtLeastOnce,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	exec := &models.JobExecution{JobID: job.ID}
	if err := repo.CreateExecution(ctx, exec); err != nil {
		t.Fatalf("CreateExecution() unexpected error: %v", err)
	}

	completedAt := time.Now().Add(time.Second)
	responseTime := 150 * time.Millisecond
	updates := map[string]interface{}{
		"completed_at":  completedAt,
		"http_status":   200,
		"response_time": responseTime,
		"success":       true,
		"error":         "",
		"response_body": "{\"ok\":true}",
	}

	if err := repo.UpdateExecution(ctx, exec.ID, updates); err != nil {
		t.Fatalf("UpdateExecution() unexpected error: %v", err)
	}

	stored := repo.executions[exec.ID]
	if stored == nil {
		t.Fatalf("UpdateExecution() execution not found after update")
	}
	if stored.CompletedAt == nil || !stored.CompletedAt.Equal(completedAt) {
		t.Fatalf("UpdateExecution() completed_at mismatch, got %v want %v", stored.CompletedAt, completedAt)
	}
	if stored.HTTPStatus != 200 {
		t.Fatalf("UpdateExecution() http_status = %d, want 200", stored.HTTPStatus)
	}
	if stored.ResponseTime != responseTime {
		t.Fatalf("UpdateExecution() response_time = %v, want %v", stored.ResponseTime, responseTime)
	}
	if !stored.Success {
		t.Fatalf("UpdateExecution() success flag not updated")
	}
	if stored.ResponseBody != "{\"ok\":true}" {
		t.Fatalf("UpdateExecution() response_body = %q, want %q", stored.ResponseBody, "{\"ok\":true}")
	}

	// Invalid field types should be ignored.
	if err := repo.UpdateExecution(ctx, exec.ID, map[string]interface{}{
		"http_status":   "bad",
		"response_time": "slow",
		"success":       "true",
	}); err != nil {
		t.Fatalf("UpdateExecution() unexpected error on invalid updates: %v", err)
	}

	stored = repo.executions[exec.ID]
	if stored.HTTPStatus != 200 {
		t.Fatalf("UpdateExecution() should ignore invalid http_status updates")
	}
	if stored.ResponseTime != responseTime {
		t.Fatalf("UpdateExecution() should ignore invalid response_time updates")
	}
	if !stored.Success {
		t.Fatalf("UpdateExecution() should ignore invalid success updates")
	}

	if err := repo.UpdateExecution(ctx, "missing", updates); err == nil {
		t.Fatalf("UpdateExecution() expected error for unknown execution ID")
	}
}

func TestMemoryRepositoryGetJobStats(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{
		Schedule: "*/5 * * * * *",
		API:      "https://example.com/stats",
		Type:     models.AtLeastOnce,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	base := time.Now().Add(-time.Minute)
	var lastExec time.Time
	var totalResp time.Duration
	successCount := int64(0)
	failCount := int64(0)

	for i := 0; i < 3; i++ {
		exec := &models.JobExecution{JobID: job.ID}
		if err := repo.CreateExecution(ctx, exec); err != nil {
			t.Fatalf("CreateExecution() unexpected error: %v", err)
		}
		execTime := base.Add(time.Duration(i) * time.Second)
		exec.ExecutedAt = execTime
		repo.executions[exec.ID] = exec

		resp := time.Duration((i + 1) * 100)
		success := i%2 == 0
		if success {
			successCount++
		} else {
			failCount++
		}
		totalResp += resp * time.Millisecond

		completedAt := execTime.Add(500 * time.Millisecond)
		if err := repo.UpdateExecution(ctx, exec.ID, map[string]interface{}{
			"completed_at":  completedAt,
			"http_status":   200,
			"response_time": resp * time.Millisecond,
			"success":       success,
		}); err != nil {
			t.Fatalf("UpdateExecution() unexpected error: %v", err)
		}

		lastExec = execTime
	}

	nextRun := time.Now().Add(time.Minute)
	if err := repo.UpdateJobNextRun(ctx, job.ID, nextRun); err != nil {
		t.Fatalf("UpdateJobNextRun() unexpected error: %v", err)
	}
	if err := repo.UpdateJobLastExecuted(ctx, job.ID, lastExec); err != nil {
		t.Fatalf("UpdateJobLastExecuted() unexpected error: %v", err)
	}

	stats, err := repo.GetJobStats(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJobStats() unexpected error: %v", err)
	}
	if stats.TotalExecutions != 3 {
		t.Fatalf("GetJobStats() total = %d, want 3", stats.TotalExecutions)
	}
	if stats.SuccessfulExecutions != successCount {
		t.Fatalf("GetJobStats() success = %d, want %d", stats.SuccessfulExecutions, successCount)
	}
	if stats.FailedExecutions != failCount {
		t.Fatalf("GetJobStats() failed = %d, want %d", stats.FailedExecutions, failCount)
	}
	expectedAvg := totalResp / time.Duration(stats.TotalExecutions)
	if stats.AverageResponseTime != expectedAvg {
		t.Fatalf("GetJobStats() avg response = %v, want %v", stats.AverageResponseTime, expectedAvg)
	}
	expectedUptime := float64(successCount) / float64(stats.TotalExecutions) * 100
	if stats.UptimePercentage != expectedUptime {
		t.Fatalf("GetJobStats() uptime = %f, want %f", stats.UptimePercentage, expectedUptime)
	}
	if stats.LastExecutionTime == nil || !stats.LastExecutionTime.Equal(lastExec) {
		t.Fatalf("GetJobStats() last execution time mismatch, got %v want %v", stats.LastExecutionTime, lastExec)
	}
	if stats.NextScheduledTime == nil || !stats.NextScheduledTime.Equal(nextRun) {
		t.Fatalf("GetJobStats() next scheduled time mismatch, got %v want %v", stats.NextScheduledTime, nextRun)
	}

	if _, err := repo.GetJobStats(ctx, "missing"); err == nil {
		t.Fatalf("GetJobStats() expected error for unknown job ID")
	}
}

func TestMemoryRepositoryGetSchedulerStats(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	jobs := []*models.Job{
		{Schedule: "0 * * * * *", API: "https://example.com/a", Type: models.AtLeastOnce},
		{Schedule: "0 * * * * *", API: "https://example.com/b", Type: models.AtLeastOnce},
		{Schedule: "0 * * * * *", API: "https://example.com/c", Type: models.AtLeastOnce},
	}
	for _, job := range jobs {
		if err := repo.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob() unexpected error: %v", err)
		}
	}

	if err := repo.UpdateJob(ctx, jobs[1].ID, map[string]interface{}{"status": models.StatusInactive}); err != nil {
		t.Fatalf("UpdateJob() unexpected error: %v", err)
	}
	if err := repo.UpdateJob(ctx, jobs[2].ID, map[string]interface{}{"status": models.StatusDeleted}); err != nil {
		t.Fatalf("UpdateJob() unexpected error: %v", err)
	}

	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)

	makeExec := func(jobID string, executedAt time.Time, success bool, response time.Duration) {
		exec := &models.JobExecution{JobID: jobID}
		if err := repo.CreateExecution(ctx, exec); err != nil {
			t.Fatalf("CreateExecution() unexpected error: %v", err)
		}
		exec.ExecutedAt = executedAt
		repo.executions[exec.ID] = exec
		if err := repo.UpdateExecution(ctx, exec.ID, map[string]interface{}{
			"response_time": response,
			"success":       success,
		}); err != nil {
			t.Fatalf("UpdateExecution() unexpected error: %v", err)
		}
	}

	makeExec(jobs[0].ID, time.Now(), true, 100*time.Millisecond)
	makeExec(jobs[0].ID, time.Now().Add(-30*time.Minute), false, 200*time.Millisecond)
	makeExec(jobs[1].ID, time.Now().Add(-2*time.Hour), true, 300*time.Millisecond) // outside range

	stats, err := repo.GetSchedulerStats(ctx, from, to)
	if err != nil {
		t.Fatalf("GetSchedulerStats() unexpected error: %v", err)
	}

	if stats["total_jobs"].(int) != 3 {
		t.Fatalf("GetSchedulerStats() total_jobs = %v, want 3", stats["total_jobs"])
	}
	if stats["active_jobs"].(int) != 1 {
		t.Fatalf("GetSchedulerStats() active_jobs = %v, want 1", stats["active_jobs"])
	}
	if stats["inactive_jobs"].(int) != 1 {
		t.Fatalf("GetSchedulerStats() inactive_jobs = %v, want 1", stats["inactive_jobs"])
	}
	if stats["deleted_jobs"].(int) != 1 {
		t.Fatalf("GetSchedulerStats() deleted_jobs = %v, want 1", stats["deleted_jobs"])
	}
	if stats["total_executions"].(int) != 2 {
		t.Fatalf("GetSchedulerStats() total_executions = %v, want 2", stats["total_executions"])
	}
	if stats["successful_runs"].(int) != 1 {
		t.Fatalf("GetSchedulerStats() successful_runs = %v, want 1", stats["successful_runs"])
	}
	if stats["failed_runs"].(int) != 1 {
		t.Fatalf("GetSchedulerStats() failed_runs = %v, want 1", stats["failed_runs"])
	}

	expectedAvg := (100*time.Millisecond + 200*time.Millisecond) / 2
	if stats["avg_response_time"].(time.Duration) != expectedAvg {
		t.Fatalf("GetSchedulerStats() avg_response_time = %v, want %v", stats["avg_response_time"], expectedAvg)
	}
	expectedSuccessRate := float64(1) / float64(2) * 100
	if stats["success_rate"].(float64) != expectedSuccessRate {
		t.Fatalf("GetSchedulerStats() success_rate = %v, want %v", stats["success_rate"], expectedSuccessRate)
	}
}

func TestMemoryRepositoryGetActiveJobs(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	activeJob := &models.Job{Schedule: "* * * * * *", API: "https://example.com/active", Type: models.AtLeastOnce}
	inactiveJob := &models.Job{Schedule: "* * * * * *", API: "https://example.com/inactive", Type: models.AtLeastOnce}
	deletedJob := &models.Job{Schedule: "* * * * * *", API: "https://example.com/deleted", Type: models.AtLeastOnce}

	for _, job := range []*models.Job{activeJob, inactiveJob, deletedJob} {
		if err := repo.CreateJob(ctx, job); err != nil {
			t.Fatalf("CreateJob() unexpected error: %v", err)
		}
	}
	if err := repo.UpdateJob(ctx, inactiveJob.ID, map[string]interface{}{"status": models.StatusInactive}); err != nil {
		t.Fatalf("UpdateJob() unexpected error: %v", err)
	}
	if err := repo.UpdateJob(ctx, deletedJob.ID, map[string]interface{}{"status": models.StatusDeleted}); err != nil {
		t.Fatalf("UpdateJob() unexpected error: %v", err)
	}

	activeJobs, err := repo.GetActiveJobs(ctx)
	if err != nil {
		t.Fatalf("GetActiveJobs() unexpected error: %v", err)
	}
	if len(activeJobs) != 1 || activeJobs[0].ID != activeJob.ID {
		t.Fatalf("GetActiveJobs() expected only active job, got %#v", activeJobs)
	}

	// Modify returned job and ensure repository data is unchanged.
	activeJobs[0].Schedule = "modified"
	stored, err := repo.GetJob(ctx, activeJob.ID)
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}
	if stored.Schedule != activeJob.Schedule {
		t.Fatalf("GetActiveJobs() should return copies; stored schedule changed to %q", stored.Schedule)
	}
}

func TestMemoryRepositoryUpdateJobNextRun(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{Schedule: "* * * * * *", API: "https://example.com/job", Type: models.AtLeastOnce}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	originalUpdatedAt := job.UpdatedAt
	nextRun := time.Now().Add(time.Minute)
	time.Sleep(5 * time.Millisecond)

	if err := repo.UpdateJobNextRun(ctx, job.ID, nextRun); err != nil {
		t.Fatalf("UpdateJobNextRun() unexpected error: %v", err)
	}

	updated, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}
	if updated.NextRun == nil || !updated.NextRun.Equal(nextRun) {
		t.Fatalf("UpdateJobNextRun() next_run = %v, want %v", updated.NextRun, nextRun)
	}
	if !job.UpdatedAt.After(originalUpdatedAt) {
		t.Fatalf("UpdateJobNextRun() expected UpdatedAt to be refreshed")
	}

	if err := repo.UpdateJobNextRun(ctx, "missing", nextRun); err == nil {
		t.Fatalf("UpdateJobNextRun() expected error for unknown job ID")
	}
}

func TestMemoryRepositoryUpdateJobLastExecuted(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()

	job := &models.Job{Schedule: "* * * * * *", API: "https://example.com/job", Type: models.AtLeastOnce}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() unexpected error: %v", err)
	}

	originalUpdatedAt := job.UpdatedAt
	lastExecuted := time.Now()
	time.Sleep(5 * time.Millisecond)

	if err := repo.UpdateJobLastExecuted(ctx, job.ID, lastExecuted); err != nil {
		t.Fatalf("UpdateJobLastExecuted() unexpected error: %v", err)
	}

	updated, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() unexpected error: %v", err)
	}
	if updated.LastExecuted == nil || !updated.LastExecuted.Equal(lastExecuted) {
		t.Fatalf("UpdateJobLastExecuted() last_executed = %v, want %v", updated.LastExecuted, lastExecuted)
	}
	if !job.UpdatedAt.After(originalUpdatedAt) {
		t.Fatalf("UpdateJobLastExecuted() expected UpdatedAt to be refreshed")
	}

	if err := repo.UpdateJobLastExecuted(ctx, "missing", lastExecuted); err == nil {
		t.Fatalf("UpdateJobLastExecuted() expected error for unknown job ID")
	}
}
