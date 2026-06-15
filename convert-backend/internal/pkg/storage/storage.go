package storage

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
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

var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func SafeFilename(filename string) string {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	if filename == "" {
		return ""
	}
	filename = path.Base(filename)
	filename = unsafeFilenameChars.ReplaceAllString(filename, "_")
	filename = strings.Trim(filename, "._-")
	if filename == "" {
		return "file"
	}
	return filename
}

func UploadKey(userID string, fileID string, filename string) (string, error) {
	userID = strings.TrimSpace(userID)
	fileID = strings.TrimSpace(fileID)
	filename = SafeFilename(filename)
	if userID == "" {
		return "", fmt.Errorf("user id is required")
	}
	if fileID == "" {
		return "", fmt.Errorf("file id is required")
	}
	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}
	return strings.Join([]string{"uploads", userID, fileID, filename}, "/"), nil
}

func ResultKey(userID string, jobID string, filename string) (string, error) {
	userID = strings.TrimSpace(userID)
	jobID = strings.TrimSpace(jobID)
	filename = SafeFilename(filename)
	if userID == "" {
		return "", fmt.Errorf("user id is required")
	}
	if jobID == "" {
		return "", fmt.Errorf("job id is required")
	}
	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}
	return strings.Join([]string{"results", userID, jobID, filename}, "/"), nil
}

func TempKey(service string, jobID string, filename string) (string, error) {
	service = strings.TrimSpace(service)
	jobID = strings.TrimSpace(jobID)
	filename = SafeFilename(filename)
	if service == "" {
		return "", fmt.Errorf("service is required")
	}
	if jobID == "" {
		return "", fmt.Errorf("job id is required")
	}
	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}
	return strings.Join([]string{"temp", service, jobID, filename}, "/"), nil
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
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return "", fmt.Errorf("storage key is required")
	}
	input := &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
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
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return "", fmt.Errorf("storage key is required")
	}
	request, err := c.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(c.downloadExpiry))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}

func URI(bucket string, key string) string {
	return "s3://" + strings.Trim(bucket, "/") + "/" + strings.TrimPrefix(key, "/")
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

func FilenameWithSuffix(filename string, suffix string) string {
	filename = SafeFilename(filename)
	if suffix == "" {
		return filename
	}

	ext := path.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	if base == "" {
		base = "file"
	}
	return base + "-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10) + suffix + ext
}
