package config

import (
	"os"
	"strconv"
	"time"
)

type GatewayConfig struct {
	ServiceName string
	HTTPAddr    string
	Processors  map[string]string
	Storage     StorageConfig
}

type WorkerConfig struct {
	ServiceName string
}

type ProcessorConfig struct {
	ServiceName string
	HTTPAddr    string
}

type StorageConfig struct {
	Provider        string
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
	UploadExpiry    time.Duration
	DownloadExpiry  time.Duration
}

func LoadGateway() GatewayConfig {
	return GatewayConfig{
		ServiceName: env("CONVERT_SERVICE_NAME", "convert-gateway"),
		HTTPAddr:    env("CONVERT_HTTP_ADDR", ":8080"),
		Processors: map[string]string{
			"pdf":   env("CONVERT_PDF_ENDPOINT", "pdf-service:9001"),
			"image": env("CONVERT_IMAGE_ENDPOINT", "image-service:9002"),
			"media": env("CONVERT_MEDIA_ENDPOINT", "media-service:9003"),
		},
		Storage: StorageConfig{
			Provider:        env("CONVERT_STORAGE_PROVIDER", "s3"),
			Endpoint:        env("CONVERT_STORAGE_ENDPOINT", "http://localhost:9000"),
			Bucket:          env("CONVERT_STORAGE_BUCKET", "convert"),
			Region:          env("CONVERT_STORAGE_REGION", "us-east-1"),
			AccessKeyID:     env("CONVERT_STORAGE_ACCESS_KEY_ID", "convert"),
			SecretAccessKey: env("CONVERT_STORAGE_SECRET_ACCESS_KEY", "convert-secret"),
			ForcePathStyle:  envBool("CONVERT_STORAGE_FORCE_PATH_STYLE", true),
			UploadExpiry:    envDuration("CONVERT_STORAGE_UPLOAD_EXPIRY", 15*time.Minute),
			DownloadExpiry:  envDuration("CONVERT_STORAGE_DOWNLOAD_EXPIRY", 15*time.Minute),
		},
	}
}

func LoadWorker() WorkerConfig {
	return WorkerConfig{
		ServiceName: env("CONVERT_SERVICE_NAME", "convert-worker"),
	}
}

func LoadProcessor(defaultName, defaultAddr string) ProcessorConfig {
	return ProcessorConfig{
		ServiceName: env("CONVERT_SERVICE_NAME", defaultName),
		HTTPAddr:    env("CONVERT_HTTP_ADDR", defaultAddr),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
