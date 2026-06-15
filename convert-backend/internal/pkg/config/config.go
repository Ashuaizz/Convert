package config

import (
	"errors"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type GatewayConfig struct {
	ServiceName string
	Env         string
	HTTPAddr    string
	Database    DatabaseConfig
	Processors  map[string]string
	Storage     StorageConfig
	Queue       QueueConfig
	Limits      LimitsConfig
}

type WorkerConfig struct {
	ServiceName string
	Env         string
}

type ProcessorConfig struct {
	ServiceName string
	Env         string
	HTTPAddr    string
}

type DatabaseConfig struct {
	DSN string
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

type QueueConfig struct {
	Provider string
	URL      string
	Stream   string
}

type LimitsConfig struct {
	MaxUploadSizeMB int
	SyncTimeout     time.Duration
	AsyncJobTimeout time.Duration
}

func LoadGateway() GatewayConfig {
	cfg := GatewayConfig{
		ServiceName: "convert-gateway",
		Env:         "dev",
		HTTPAddr:    ":8080",
		Database: DatabaseConfig{
			DSN: "postgres://convert:convert@localhost:5432/convert?sslmode=disable",
		},
		Processors: map[string]string{
			"pdf":   "pdf-service:9001",
			"image": "image-service:9002",
			"media": "media-service:9003",
		},
		Storage: StorageConfig{
			Provider:        "s3",
			Endpoint:        "http://localhost:9000",
			Bucket:          "convert",
			Region:          "us-east-1",
			AccessKeyID:     "convert",
			SecretAccessKey: "convert-secret",
			ForcePathStyle:  true,
			UploadExpiry:    15 * time.Minute,
			DownloadExpiry:  15 * time.Minute,
		},
		Queue: QueueConfig{
			Provider: "nats",
			URL:      "nats://localhost:4222",
			Stream:   "convert_jobs",
		},
		Limits: LimitsConfig{
			MaxUploadSizeMB: 500,
			SyncTimeout:     20 * time.Second,
			AsyncJobTimeout: 30 * time.Minute,
		},
	}

	loadGatewayYAML(&cfg, env("CONVERT_CONFIG", "configs/gateway.dev.yaml"))
	applyGatewayEnv(&cfg)
	return cfg
}

func LoadWorker() WorkerConfig {
	cfg := WorkerConfig{
		ServiceName: "convert-worker",
		Env:         "dev",
	}
	cfg.ServiceName = env("CONVERT_SERVICE_NAME", cfg.ServiceName)
	cfg.Env = env("CONVERT_ENV", cfg.Env)
	return cfg
}

func LoadProcessor(defaultName, defaultAddr string) ProcessorConfig {
	cfg := ProcessorConfig{
		ServiceName: defaultName,
		Env:         "dev",
		HTTPAddr:    defaultAddr,
	}
	cfg.ServiceName = env("CONVERT_SERVICE_NAME", cfg.ServiceName)
	cfg.Env = env("CONVERT_ENV", cfg.Env)
	cfg.HTTPAddr = env("CONVERT_HTTP_ADDR", cfg.HTTPAddr)
	return cfg
}

type gatewayYAML struct {
	Server struct {
		Name     string `yaml:"name"`
		Env      string `yaml:"env"`
		HTTPAddr string `yaml:"http_addr"`
	} `yaml:"server"`
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
	Storage struct {
		Provider        string `yaml:"provider"`
		Endpoint        string `yaml:"endpoint"`
		Bucket          string `yaml:"bucket"`
		Region          string `yaml:"region"`
		AccessKeyID     string `yaml:"access_key_id"`
		SecretAccessKey string `yaml:"secret_access_key"`
		ForcePathStyle  *bool  `yaml:"force_path_style"`
		UploadExpiry    string `yaml:"upload_expiry"`
		DownloadExpiry  string `yaml:"download_expiry"`
	} `yaml:"storage"`
	Queue struct {
		Provider string `yaml:"provider"`
		URL      string `yaml:"url"`
		Stream   string `yaml:"stream"`
	} `yaml:"queue"`
	Processors map[string]struct {
		Endpoint string `yaml:"endpoint"`
	} `yaml:"processors"`
	Limits struct {
		MaxUploadSizeMB int    `yaml:"max_upload_size_mb"`
		SyncTimeout     string `yaml:"sync_timeout"`
		AsyncJobTimeout string `yaml:"async_job_timeout"`
	} `yaml:"limits"`
}

func loadGatewayYAML(cfg *GatewayConfig, path string) {
	if path == "" {
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		panic(err)
	}

	var raw gatewayYAML
	if err := yaml.Unmarshal(content, &raw); err != nil {
		panic(err)
	}

	if raw.Server.Name != "" {
		cfg.ServiceName = raw.Server.Name
	}
	if raw.Server.Env != "" {
		cfg.Env = raw.Server.Env
	}
	if raw.Server.HTTPAddr != "" {
		cfg.HTTPAddr = raw.Server.HTTPAddr
	}
	if raw.Database.DSN != "" {
		cfg.Database.DSN = raw.Database.DSN
	}

	if raw.Storage.Provider != "" {
		cfg.Storage.Provider = raw.Storage.Provider
	}
	if raw.Storage.Endpoint != "" {
		cfg.Storage.Endpoint = raw.Storage.Endpoint
	}
	if raw.Storage.Bucket != "" {
		cfg.Storage.Bucket = raw.Storage.Bucket
	}
	if raw.Storage.Region != "" {
		cfg.Storage.Region = raw.Storage.Region
	}
	if raw.Storage.AccessKeyID != "" {
		cfg.Storage.AccessKeyID = raw.Storage.AccessKeyID
	}
	if raw.Storage.SecretAccessKey != "" {
		cfg.Storage.SecretAccessKey = raw.Storage.SecretAccessKey
	}
	if raw.Storage.ForcePathStyle != nil {
		cfg.Storage.ForcePathStyle = *raw.Storage.ForcePathStyle
	}
	cfg.Storage.UploadExpiry = parseDuration(raw.Storage.UploadExpiry, cfg.Storage.UploadExpiry)
	cfg.Storage.DownloadExpiry = parseDuration(raw.Storage.DownloadExpiry, cfg.Storage.DownloadExpiry)

	if raw.Queue.Provider != "" {
		cfg.Queue.Provider = raw.Queue.Provider
	}
	if raw.Queue.URL != "" {
		cfg.Queue.URL = raw.Queue.URL
	}
	if raw.Queue.Stream != "" {
		cfg.Queue.Stream = raw.Queue.Stream
	}

	for name, processor := range raw.Processors {
		if processor.Endpoint != "" {
			cfg.Processors[name] = processor.Endpoint
		}
	}

	if raw.Limits.MaxUploadSizeMB > 0 {
		cfg.Limits.MaxUploadSizeMB = raw.Limits.MaxUploadSizeMB
	}
	cfg.Limits.SyncTimeout = parseDuration(raw.Limits.SyncTimeout, cfg.Limits.SyncTimeout)
	cfg.Limits.AsyncJobTimeout = parseDuration(raw.Limits.AsyncJobTimeout, cfg.Limits.AsyncJobTimeout)
}

func applyGatewayEnv(cfg *GatewayConfig) {
	cfg.ServiceName = env("CONVERT_SERVICE_NAME", cfg.ServiceName)
	cfg.Env = env("CONVERT_ENV", cfg.Env)
	cfg.HTTPAddr = env("CONVERT_HTTP_ADDR", cfg.HTTPAddr)
	cfg.Database.DSN = env("CONVERT_DATABASE_DSN", cfg.Database.DSN)

	cfg.Storage.Provider = env("CONVERT_STORAGE_PROVIDER", cfg.Storage.Provider)
	cfg.Storage.Endpoint = env("CONVERT_STORAGE_ENDPOINT", cfg.Storage.Endpoint)
	cfg.Storage.Bucket = env("CONVERT_STORAGE_BUCKET", cfg.Storage.Bucket)
	cfg.Storage.Region = env("CONVERT_STORAGE_REGION", cfg.Storage.Region)
	cfg.Storage.AccessKeyID = env("CONVERT_STORAGE_ACCESS_KEY_ID", cfg.Storage.AccessKeyID)
	cfg.Storage.SecretAccessKey = env("CONVERT_STORAGE_SECRET_ACCESS_KEY", cfg.Storage.SecretAccessKey)
	cfg.Storage.ForcePathStyle = envBool("CONVERT_STORAGE_FORCE_PATH_STYLE", cfg.Storage.ForcePathStyle)
	cfg.Storage.UploadExpiry = envDuration("CONVERT_STORAGE_UPLOAD_EXPIRY", cfg.Storage.UploadExpiry)
	cfg.Storage.DownloadExpiry = envDuration("CONVERT_STORAGE_DOWNLOAD_EXPIRY", cfg.Storage.DownloadExpiry)

	cfg.Queue.Provider = env("CONVERT_QUEUE_PROVIDER", cfg.Queue.Provider)
	cfg.Queue.URL = env("CONVERT_QUEUE_URL", cfg.Queue.URL)
	cfg.Queue.Stream = env("CONVERT_QUEUE_STREAM", cfg.Queue.Stream)

	cfg.Processors["pdf"] = env("CONVERT_PDF_ENDPOINT", cfg.Processors["pdf"])
	cfg.Processors["image"] = env("CONVERT_IMAGE_ENDPOINT", cfg.Processors["image"])
	cfg.Processors["media"] = env("CONVERT_MEDIA_ENDPOINT", cfg.Processors["media"])

	cfg.Limits.MaxUploadSizeMB = envInt("CONVERT_MAX_UPLOAD_SIZE_MB", cfg.Limits.MaxUploadSizeMB)
	cfg.Limits.SyncTimeout = envDuration("CONVERT_SYNC_TIMEOUT", cfg.Limits.SyncTimeout)
	cfg.Limits.AsyncJobTimeout = envDuration("CONVERT_ASYNC_JOB_TIMEOUT", cfg.Limits.AsyncJobTimeout)
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
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

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
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
