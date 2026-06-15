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
