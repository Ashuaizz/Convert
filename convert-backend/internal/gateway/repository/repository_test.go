package repository

import (
	"context"
	"testing"
	"time"
)

func TestMemoryRepositoryFileLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMemoryRepository()
	now := time.Now().UTC()
	file := File{
		ID:          "file_test",
		UserID:      "dev-user",
		Filename:    "demo.jpg",
		ContentType: "image/jpeg",
		Size:        123,
		StorageURI:  "s3://convert/uploads/dev-user/file_test/demo.jpg",
		Status:      "pending_upload",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := repo.CreateFile(ctx, file); err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}

	got, err := repo.GetFile(ctx, file.ID)
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}
	if got.ID != file.ID || got.Status != "pending_upload" {
		t.Fatalf("GetFile() = %+v, want id %q with pending_upload", got, file.ID)
	}

	updatedAt := now.Add(time.Minute)
	updated, err := repo.UpdateFileStatus(ctx, file.ID, "uploaded", updatedAt)
	if err != nil {
		t.Fatalf("UpdateFileStatus() error = %v", err)
	}
	if updated.Status != "uploaded" || !updated.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdateFileStatus() = %+v, want uploaded at %s", updated, updatedAt)
	}
}

func TestMemoryRepositoryJobLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMemoryRepository()
	now := time.Now().UTC()
	file := File{
		ID:          "file_test",
		UserID:      "dev-user",
		Filename:    "demo.jpg",
		ContentType: "image/jpeg",
		Size:        123,
		StorageURI:  "s3://convert/uploads/dev-user/file_test/demo.jpg",
		Status:      "uploaded",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	job := Job{
		ID:        "job_test",
		UserID:    "dev-user",
		Type:      "image.resize",
		Status:    "queued",
		Progress:  0,
		Options:   map[string]any{"width": 800},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := repo.CreateFile(ctx, file); err != nil {
		t.Fatalf("CreateFile() error = %v", err)
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if err := repo.AttachJobFile(ctx, job.ID, file.ID, "input"); err != nil {
		t.Fatalf("AttachJobFile() error = %v", err)
	}

	got, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.ID != job.ID || got.UserID != job.UserID || got.Type != job.Type {
		t.Fatalf("GetJob() = %+v, want %+v", got, job)
	}
}

func TestMemoryRepositoryJobStateTransitions(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMemoryRepository()
	now := time.Now().UTC()
	job := Job{
		ID:         "job_test",
		UserID:     "dev-user",
		Type:       "image.resize",
		Status:     "queued",
		Progress:   0,
		Options:    map[string]any{"width": 800},
		MaxRetries: 2,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	running, claimed, err := repo.ClaimJob(ctx, job.ID, now.Add(time.Second))
	if err != nil {
		t.Fatalf("ClaimJob() error = %v", err)
	}
	if !claimed || running.Status != "running" || running.StartedAt == nil {
		t.Fatalf("ClaimJob() = %+v, %v; want running claimed job", running, claimed)
	}

	again, claimed, err := repo.ClaimJob(ctx, job.ID, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("second ClaimJob() error = %v", err)
	}
	if claimed || again.Status != "running" {
		t.Fatalf("second ClaimJob() = %+v, %v; want skipped running job", again, claimed)
	}

	done, err := repo.SucceedJob(ctx, job.ID, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("SucceedJob() error = %v", err)
	}
	if done.Status != "succeeded" || done.Progress != 100 || done.FinishedAt == nil {
		t.Fatalf("SucceedJob() = %+v, want succeeded with progress 100", done)
	}
}

func TestMemoryRepositoryFailJobIncrementsRetryCount(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := NewMemoryRepository()
	now := time.Now().UTC()
	job := Job{
		ID:        "job_test",
		UserID:    "dev-user",
		Type:      "image.resize",
		Status:    "queued",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	if _, claimed, err := repo.ClaimJob(ctx, job.ID, now.Add(time.Second)); err != nil || !claimed {
		t.Fatalf("ClaimJob() claimed = %v, error = %v", claimed, err)
	}

	failed, err := repo.FailJob(ctx, job.ID, "JOB_FAILED", "boom", now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("FailJob() error = %v", err)
	}
	if failed.Status != "failed" || failed.RetryCount != 1 || failed.ErrorCode != "JOB_FAILED" || failed.ErrorMessage != "boom" {
		t.Fatalf("FailJob() = %+v, want failed with retry count and error", failed)
	}
}
