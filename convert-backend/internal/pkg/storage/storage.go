package storage

import "context"

type FileRef struct {
	ID          string `json:"id"`
	URI         string `json:"uri"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type Client interface {
	PresignUpload(ctx context.Context, key string) (string, error)
	PresignDownload(ctx context.Context, key string) (string, error)
}

type NoopClient struct{}

func NewNoopClient() *NoopClient {
	return &NoopClient{}
}

func (c *NoopClient) PresignUpload(ctx context.Context, key string) (string, error) {
	return "noop://upload/" + key, nil
}

func (c *NoopClient) PresignDownload(ctx context.Context, key string) (string, error) {
	return "noop://download/" + key, nil
}
