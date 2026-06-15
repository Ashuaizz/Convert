package consumer

import (
	"context"

	"convert-backend/internal/worker/executor"
)

type NoopConsumer struct {
	executor *executor.NoopExecutor
}

func NewNoopConsumer(executor *executor.NoopExecutor) *NoopConsumer {
	return &NoopConsumer{executor: executor}
}

func (c *NoopConsumer) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
