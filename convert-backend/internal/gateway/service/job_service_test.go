package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"convert-backend/internal/gateway/repository"
	"convert-backend/internal/gateway/rpcclient"
	"convert-backend/internal/pkg/queue"
)

type fakeStorage struct{}

func (s fakeStorage) URI(key string) string {
	return "s3://convert/" + strings.TrimPrefix(key, "/")
}

func (s fakeStorage) UploadExpiry() time.Duration {
	return 15 * time.Minute
}

func (s fakeStorage) DownloadExpiry() time.Duration {
	return 15 * time.Minute
}

func (s fakeStorage) PresignUpload(ctx context.Context, key string, contentType string) (string, error) {
	return "https://storage.local/upload/" + strings.TrimPrefix(key, "/"), nil
}

func (s fakeStorage) PresignDownload(ctx context.Context, key string) (string, error) {
	return "https://storage.local/download/" + strings.TrimPrefix(key, "/"), nil
}

func newTestJobService(maxUploadSizeBytes int64) (*JobService, repository.Repository) {
	repo := repository.NewMemoryRepository()
	service := NewJobService(
		repo,
		fakeStorage{},
		queue.NewNoopPublisher(),
		rpcclient.NewRegistry(nil),
		WithMaxUploadSizeBytes(maxUploadSizeBytes),
	)
	return service, repo
}

func TestPresignUploadAndCompleteLifecycle(t *testing.T) {
	t.Parallel()

	service, repo := newTestJobService(10 << 20)
	ctx := context.Background()

	presign, err := service.PresignUpload(ctx, PresignUploadRequest{
		UserID:      "dev-user",
		Filename:    "../demo image.jpg",
		ContentType: "image/jpeg",
		Size:        1024,
	})
	if err != nil {
		t.Fatalf("PresignUpload() error = %v", err)
	}
	if presign.FileID == "" || !strings.Contains(presign.UploadURL, "/uploads/dev-user/") {
		t.Fatalf("PresignUpload() = %+v", presign)
	}

	file, err := repo.GetFile(ctx, presign.FileID)
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}
	if file.Status != "pending_upload" {
		t.Fatalf("file status = %q, want pending_upload", file.Status)
	}

	completed, err := service.CompleteUpload(ctx, presign.FileID)
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}
	if completed.Status != "uploaded" {
		t.Fatalf("completed status = %q, want uploaded", completed.Status)
	}
}

func TestPresignUploadRejectsOversizedFiles(t *testing.T) {
	t.Parallel()

	service, _ := newTestJobService(100)
	_, err := service.PresignUpload(context.Background(), PresignUploadRequest{
		UserID:      "dev-user",
		Filename:    "demo.jpg",
		ContentType: "image/jpeg",
		Size:        101,
	})
	if err == nil {
		t.Fatal("PresignUpload() error = nil, want oversized error")
	}
}

func TestPresignUploadRejectsUnsupportedContentType(t *testing.T) {
	t.Parallel()

	service, _ := newTestJobService(10 << 20)
	_, err := service.PresignUpload(context.Background(), PresignUploadRequest{
		UserID:      "dev-user",
		Filename:    "demo.exe",
		ContentType: "application/x-msdownload",
		Size:        100,
	})
	if err == nil {
		t.Fatal("PresignUpload() error = nil, want unsupported media type error")
	}
}
