package config

import "os"

type GatewayConfig struct {
	ServiceName string
	HTTPAddr    string
	Processors  map[string]string
}

type WorkerConfig struct {
	ServiceName string
}

type ProcessorConfig struct {
	ServiceName string
	HTTPAddr    string
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
