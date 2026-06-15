package executor

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"convert-backend/internal/gateway/repository"
)

type countingExecutor struct {
	count int
	err   error
}

func (e *countingExecutor) Execute(ctx context.Context, job Job) error {
	e.count++
	return e.err
}

func TestStateTransitionExecutorSucceedsClaimedJob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	inner := &countingExecutor{}
	exec := NewStateTransitionExecutor(repo, inner, slog.Default())
	now := time.Now().UTC()
	job := repository.Job{
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

	if err := exec.Execute(ctx, Job{ID: job.ID, Type: job.Type}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	got, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if inner.count != 1 || got.Status != "succeeded" || got.Progress != 100 {
		t.Fatalf("count = %d, job = %+v; want one execution and succeeded job", inner.count, got)
	}
}

func TestStateTransitionExecutorSkipsAlreadyRunningJob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	inner := &countingExecutor{}
	exec := NewStateTransitionExecutor(repo, inner, slog.Default())
	now := time.Now().UTC()
	job := repository.Job{
		ID:        "job_test",
		UserID:    "dev-user",
		Type:      "image.resize",
		Status:    "running",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	if err := exec.Execute(ctx, Job{ID: job.ID, Type: job.Type}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if inner.count != 0 {
		t.Fatalf("inner executions = %d, want 0", inner.count)
	}
}

func TestStateTransitionExecutorFailsJob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	inner := &countingExecutor{err: errors.New("boom")}
	exec := NewStateTransitionExecutor(repo, inner, slog.Default())
	now := time.Now().UTC()
	job := repository.Job{
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

	if err := exec.Execute(ctx, Job{ID: job.ID, Type: job.Type}); err == nil {
		t.Fatal("Execute() error = nil, want inner error")
	}
	got, err := repo.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.Status != "failed" || got.RetryCount != 1 || got.ErrorMessage != "boom" {
		t.Fatalf("job = %+v, want failed with retry count and error", got)
	}
}
