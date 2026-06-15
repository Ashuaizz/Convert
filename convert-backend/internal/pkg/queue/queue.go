package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"convert-backend/internal/pkg/config"

	"github.com/nats-io/nats.go"
)

const DefaultSubject = "jobs.created"

type JobMessage struct {
	JobID        string   `json:"job_id"`
	Type         string   `json:"type"`
	InputFileIDs []string `json:"input_file_ids"`
	CreatedAt    string   `json:"created_at,omitempty"`
}

type Publisher interface {
	PublishJob(ctx context.Context, message JobMessage) error
}

type JobHandler func(context.Context, JobMessage) error

type Consumer interface {
	ConsumeJobs(ctx context.Context, handler JobHandler) error
}

type NATSClient struct {
	conn     *nats.Conn
	js       nats.JetStreamContext
	stream   string
	subject  string
	consumer string
}

func NewNATSClient(ctx context.Context, cfg config.QueueConfig) (*NATSClient, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("nats url is required")
	}
	if cfg.Stream == "" {
		cfg.Stream = "convert_jobs"
	}
	if cfg.Subject == "" {
		cfg.Subject = DefaultSubject
	}
	if cfg.Consumer == "" {
		cfg.Consumer = "convert-worker"
	}

	conn, err := nats.Connect(cfg.URL)
	if err != nil {
		return nil, err
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, err
	}

	client := &NATSClient{
		conn:     conn,
		js:       js,
		stream:   cfg.Stream,
		subject:  cfg.Subject,
		consumer: cfg.Consumer,
	}
	if err := client.ensureStream(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return client, nil
}

func (c *NATSClient) Close() {
	if c.conn != nil {
		c.conn.Drain()
		c.conn.Close()
	}
}

func (c *NATSClient) PublishJob(ctx context.Context, message JobMessage) error {
	if message.JobID == "" {
		return fmt.Errorf("job_id is required")
	}
	if message.CreatedAt == "" {
		message.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = c.js.Publish(c.subject, payload, nats.Context(ctx))
	return err
}

func (c *NATSClient) ConsumeJobs(ctx context.Context, handler JobHandler) error {
	if handler == nil {
		return fmt.Errorf("job handler is required")
	}

	sub, err := c.js.PullSubscribe(
		c.subject,
		c.consumer,
		nats.BindStream(c.stream),
		nats.ManualAck(),
	)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		messages, err := sub.Fetch(1, nats.Context(ctx), nats.MaxWait(time.Second))
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		if errors.Is(err, nats.ErrTimeout) {
			continue
		}
		if err != nil {
			return err
		}

		for _, message := range messages {
			var job JobMessage
			if err := json.Unmarshal(message.Data, &job); err != nil {
				_ = message.Term()
				continue
			}
			if err := handler(ctx, job); err != nil {
				_ = message.Nak()
				continue
			}
			_ = message.Ack()
		}
	}
}

func (c *NATSClient) ensureStream(ctx context.Context) error {
	info, err := c.js.StreamInfo(c.stream, nats.Context(ctx))
	if err == nil && info != nil {
		return nil
	}
	if err != nil && !errors.Is(err, nats.ErrStreamNotFound) {
		return err
	}

	_, err = c.js.AddStream(&nats.StreamConfig{
		Name:     c.stream,
		Subjects: []string{c.subject},
	}, nats.Context(ctx))
	return err
}

type NoopPublisher struct{}

func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}

func (p *NoopPublisher) PublishJob(ctx context.Context, message JobMessage) error {
	return nil
}

type NoopConsumer struct{}

func NewNoopConsumer() *NoopConsumer {
	return &NoopConsumer{}
}

func (c *NoopConsumer) ConsumeJobs(ctx context.Context, handler JobHandler) error {
	<-ctx.Done()
	return nil
}
