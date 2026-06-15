package executor

import "context"

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
