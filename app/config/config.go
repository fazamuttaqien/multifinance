package config

import (
	"time"
)

type Config struct {
	SERVICE_NAME                string
	SERVICE_VERSION             string
	ENVIRONMENT                 string
	OTEL_EXPORTER_OTLP_ENDPOINT string
	OTEL_RESOURCE_ATTRIBUTES    string
	LOG_LEVEL                   string
	METRIC_INTERVAL             time.Duration
	RUNTIME_METRICS             bool
	REQUESTS_METRIC             bool
	DEVELOPMENT_MODE            bool
	SERVER_PORT                 string
	CLOUDINARY_CLOUD            string
	CLOUDINARY_API_KEY          string
	CLOUDINARY_API_SECRET       string
	SHUTDOWN_TIMEOUT            time.Duration
}

func LoadConfig() (*Config, error) {
	// file, err := os.Open(configPath)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to open configuration file: %w", err)
	// }
	// defer file.Close()

	config := &Config{
		SERVICE_NAME:                "multifinance",
		SERVICE_VERSION:             "1.0.0",
		ENVIRONMENT:                 "production",
		OTEL_EXPORTER_OTLP_ENDPOINT: "",
		OTEL_RESOURCE_ATTRIBUTES:    "service.name=multifinance,service.namespace=multifinance-group,deployment.environment=production",
		LOG_LEVEL:                   "info",
		METRIC_INTERVAL:             15 * time.Second,
		RUNTIME_METRICS:             true,
		REQUESTS_METRIC:             true,
		DEVELOPMENT_MODE:            false,
		SERVER_PORT:                 "3001",
		CLOUDINARY_CLOUD:            "",
		CLOUDINARY_API_KEY:          "",
		CLOUDINARY_API_SECRET:       "",
		SHUTDOWN_TIMEOUT:            15 * time.Second,
	}

	return config, nil
}
