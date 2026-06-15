package executor

import (
	"context"
	"log/slog"
	"time"

	"convert-backend/internal/gateway/repository"
)

type Job struct {
	ID           string
	Type         string
	InputFileIDs []string
}

type Executor interface {
	Execute(ctx context.Context, job Job) error
}

type NoopExecutor struct{}

func NewNoopExecutor() *NoopExecutor {
	return &NoopExecutor{}
}

func (e *NoopExecutor) Execute(ctx context.Context, job Job) error {
	return nil
}

type StateTransitionExecutor struct {
	repo   repository.Repository
	inner  Executor
	logger *slog.Logger
}

func NewStateTransitionExecutor(repo repository.Repository, inner Executor, logger *slog.Logger) *StateTransitionExecutor {
	return &StateTransitionExecutor{
		repo:   repo,
		inner:  inner,
		logger: logger,
	}
}

func (e *StateTransitionExecutor) Execute(ctx context.Context, job Job) error {
	now := time.Now().UTC()
	claimed, ok, err := e.repo.ClaimJob(ctx, job.ID, now)
	if err != nil {
		return err
	}
	if !ok {
		e.logger.Info("job skipped", "job_id", job.ID, "status", claimed.Status)
		return nil
	}

	if err := e.inner.Execute(ctx, job); err != nil {
		_, failErr := e.repo.FailJob(ctx, job.ID, "JOB_FAILED", err.Error(), time.Now().UTC())
		if failErr != nil {
			return failErr
		}
		return err
	}

	_, err = e.repo.SucceedJob(ctx, job.ID, time.Now().UTC())
	return err
}
