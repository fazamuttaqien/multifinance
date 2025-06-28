// Use vault agent

package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	SERVICE_NAME                 string
	SERVICE_VERSION              string
	ENVIRONMENT                  string
	OTEL_EXPORTER_OTLP_ENDPOINT  string
	OTEL_RESOURCE_ATTRIBUTES     string
	LOG_LEVEL                    string
	METRIC_INTERVAL              time.Duration
	RUNTIME_METRICS              bool
	REQUESTS_METRIC              bool
	DEV_MODE                     bool
	SERVER_PORT                  string
	CLOUDINARY_CLOUD             string
	CLOUDINARY_API_KEY           string
	CLOUDINARY_API_SECRET        string
	SHUTDOWN_TIMEOUT             time.Duration
}

func LoadConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka file konfigurasi: %w", err)
	}
	defer file.Close()

	config := &Config{
		SERVICE_NAME:                 "media",
		SERVICE_VERSION:              "1.0.0",
		ENVIRONMENT:                  "production",
		OTEL_EXPORTER_OTLP_ENDPOINT:  "grafana-k8s-monitoring-alloy.grafana.svc.cluster.local:4317",
		OTEL_RESOURCE_ATTRIBUTES:     "service.name=media,service.namespace=canva-group,deployment.environment=production",
		LOG_LEVEL:                    "info",
		METRIC_INTERVAL:              15 * time.Second,
		RUNTIME_METRICS:              true,
		REQUESTS_METRIC:              true,
		DEV_MODE:                     false,
		SERVER_PORT:                  "3001",
		CLOUDINARY_CLOUD:             "drnswvvpg",
		CLOUDINARY_API_KEY:           "",
		CLOUDINARY_API_SECRET:        "",
		SHUTDOWN_TIMEOUT:             15 * time.Second,
	}

	secretsMap := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = strings.Trim(value, "\"")
			}
			secretsMap[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error saat memindai file konfigurasi: %w", err)
	}

	if cloudinaryKey, ok := secretsMap["CLOUDINARY_API_KEY"]; ok {
		config.CLOUDINARY_API_KEY = cloudinaryKey
	}
	if cloudinarySecret, ok := secretsMap["CLOUDINARY_API_SECRET"]; ok {
		config.CLOUDINARY_API_SECRET = cloudinarySecret
	}

	return config, nil
}
