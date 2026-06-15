package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"convert-backend/internal/pkg/config"
)

func TestSafeFilename(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		`..\..\demo.jpg`:         "demo.jpg",
		` nested /汉字 demo!.png `: "demo_.png",
		`.hidden`:                "hidden",
		"":                       "",
	}

	for input, want := range tests {
		if got := SafeFilename(input); got != want {
			t.Fatalf("SafeFilename(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStorageKeys(t *testing.T) {
	t.Parallel()

	key, err := UploadKey("dev-user", "file_123", "../demo image.jpg")
	if err != nil {
		t.Fatalf("UploadKey() error = %v", err)
	}
	if key != "uploads/dev-user/file_123/demo_image.jpg" {
		t.Fatalf("UploadKey() = %q", key)
	}

	key, err = ResultKey("dev-user", "job_123", "result.webp")
	if err != nil {
		t.Fatalf("ResultKey() error = %v", err)
	}
	if key != "results/dev-user/job_123/result.webp" {
		t.Fatalf("ResultKey() = %q", key)
	}
}

func TestKeyFromURI(t *testing.T) {
	t.Parallel()

	key, err := KeyFromURI("s3://convert/uploads/dev-user/file_123/demo.jpg")
	if err != nil {
		t.Fatalf("KeyFromURI() error = %v", err)
	}
	if key != "uploads/dev-user/file_123/demo.jpg" {
		t.Fatalf("KeyFromURI() = %q", key)
	}
}

func TestS3ClientPresign(t *testing.T) {
	t.Parallel()

	client, err := NewS3Client(context.Background(), config.StorageConfig{
		Endpoint:        "http://localhost:9000",
		Bucket:          "convert",
		Region:          "us-east-1",
		AccessKeyID:     "convert",
		SecretAccessKey: "convert-secret",
		ForcePathStyle:  true,
		UploadExpiry:    15 * time.Minute,
		DownloadExpiry:  15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewS3Client() error = %v", err)
	}

	uploadURL, err := client.PresignUpload(context.Background(), "uploads/dev-user/file_123/demo.jpg", "image/jpeg")
	if err != nil {
		t.Fatalf("PresignUpload() error = %v", err)
	}
	if !strings.Contains(uploadURL, "uploads/dev-user/file_123/demo.jpg") {
		t.Fatalf("PresignUpload() URL = %q", uploadURL)
	}

	downloadURL, err := client.PresignDownload(context.Background(), "uploads/dev-user/file_123/demo.jpg")
	if err != nil {
		t.Fatalf("PresignDownload() error = %v", err)
	}
	if !strings.Contains(downloadURL, "uploads/dev-user/file_123/demo.jpg") {
		t.Fatalf("PresignDownload() URL = %q", downloadURL)
	}
}
