package executor

import "context"

type Job struct {
	ID   string
	Type string
}

type NoopExecutor struct{}

func NewNoopExecutor() *NoopExecutor {
	return &NoopExecutor{}
}

func (e *NoopExecutor) Execute(ctx context.Context, job Job) error {
	return nil
}
