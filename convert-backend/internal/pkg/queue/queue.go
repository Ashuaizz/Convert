package queue

import "context"

type JobMessage struct {
	JobID        string   `json:"job_id"`
	Type         string   `json:"type"`
	InputFileIDs []string `json:"input_file_ids"`
}

type Publisher interface {
	PublishJob(ctx context.Context, message JobMessage) error
}

type NoopPublisher struct{}

func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}

func (p *NoopPublisher) PublishJob(ctx context.Context, message JobMessage) error {
	return nil
}
