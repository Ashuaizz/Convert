package storage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"convert-backend/internal/pkg/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type FileRef struct {
	ID          string `json:"id"`
	URI         string `json:"uri"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type Client interface {
	URI(key string) string
	UploadExpiry() time.Duration
	DownloadExpiry() time.Duration
	PresignUpload(ctx context.Context, key string, contentType string) (string, error)
	PresignDownload(ctx context.Context, key string) (string, error)
}

type S3Client struct {
	bucket         string
	uploadExpiry   time.Duration
	downloadExpiry time.Duration
	presigner      *s3.PresignClient
}

func NewS3Client(ctx context.Context, cfg config.StorageConfig) (*S3Client, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("storage bucket is required")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.UploadExpiry <= 0 {
		cfg.UploadExpiry = 15 * time.Minute
	}
	if cfg.DownloadExpiry <= 0 {
		cfg.DownloadExpiry = 15 * time.Minute
	}

	options := []func(*awscfg.LoadOptions) error{
		awscfg.WithRegion(cfg.Region),
	}
	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" {
		options = append(options, awscfg.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsConfig, err := awscfg.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = cfg.ForcePathStyle
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	return &S3Client{
		bucket:         cfg.Bucket,
		uploadExpiry:   cfg.UploadExpiry,
		downloadExpiry: cfg.DownloadExpiry,
		presigner:      s3.NewPresignClient(client),
	}, nil
}

func (c *S3Client) URI(key string) string {
	return fmt.Sprintf("s3://%s/%s", c.bucket, strings.TrimPrefix(key, "/"))
}

func (c *S3Client) UploadExpiry() time.Duration {
	return c.uploadExpiry
}

func (c *S3Client) DownloadExpiry() time.Duration {
	return c.downloadExpiry
}

func (c *S3Client) PresignUpload(ctx context.Context, key string, contentType string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(strings.TrimPrefix(key, "/")),
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	request, err := c.presigner.PresignPutObject(ctx, input, s3.WithPresignExpires(c.uploadExpiry))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

func (c *S3Client) PresignDownload(ctx context.Context, key string) (string, error) {
	request, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(strings.TrimPrefix(key, "/")),
	}, s3.WithPresignExpires(c.downloadExpiry))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

func KeyFromURI(uri string) (string, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "s3" {
		return "", fmt.Errorf("unsupported storage uri scheme %q", parsed.Scheme)
	}
	key := strings.TrimPrefix(parsed.Path, "/")
	if parsed.Host == "" || key == "" {
		return "", fmt.Errorf("invalid storage uri")
	}
	return key, nil
}
