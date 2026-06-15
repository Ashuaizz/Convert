package consumer

import (
	"context"
	"log/slog"

	"convert-backend/internal/pkg/queue"
	"convert-backend/internal/worker/executor"
)

type Consumer struct {
	queue    queue.Consumer
	executor executor.Executor
	logger   *slog.Logger
}

func New(queue queue.Consumer, executor executor.Executor, logger *slog.Logger) *Consumer {
	return &Consumer{
		queue:    queue,
		executor: executor,
		logger:   logger,
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	return c.queue.ConsumeJobs(ctx, func(ctx context.Context, message queue.JobMessage) error {
		c.logger.Info("job received", "job_id", message.JobID, "type", message.Type)
		return c.executor.Execute(ctx, executor.Job{
			ID:           message.JobID,
			Type:         message.Type,
			InputFileIDs: message.InputFileIDs,
		})
	})
}
