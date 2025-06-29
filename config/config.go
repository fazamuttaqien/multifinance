package config

import (
	"os"
	"strconv"
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
	MYSQL_HOST                  string
	MYSQL_PORT                  string
	MYSQL_USER                  string
	MYSQL_PASSWORD              string
	MYSQL_DBNAME                string
	REDIS_ADDRESS               string
	REDIS_PASSWORD              string
	JWT_SECRET_KEY              string
	SHUTDOWN_TIMEOUT            time.Duration
}

func LoadConfig() (*Config, error) {
	// Helper function to get environment variable with default value
	Env := func(key, defaultValue string) string {
		if value := os.Getenv(key); value != "" {
			return value
		}
		return defaultValue
	}

	// Helper function to parse Duration from environment variable
	Duration := func(key string, defaultValue time.Duration) time.Duration {
		if value := os.Getenv(key); value != "" {
			if duration, err := time.ParseDuration(value); err == nil {
				return duration
			}
		}
		return defaultValue
	}

	// Helper function to parse boolean from environment variable
	Bool := func(key string, defaultValue bool) bool {
		if value := os.Getenv(key); value != "" {
			if boolValue, err := strconv.ParseBool(value); err == nil {
				return boolValue
			}
		}
		return defaultValue
	}

	config := &Config{
		SERVICE_NAME:                Env("SERVICE_NAME", "multifinance"),
		SERVICE_VERSION:             Env("SERVICE_VERSION", "1.0.0"),
		ENVIRONMENT:                 Env("ENVIRONMENT", "production"),
		OTEL_EXPORTER_OTLP_ENDPOINT: Env("OTEL_EXPORTER_OTLP_ENDPOINT", "0.0.0.0:4317"),
		OTEL_RESOURCE_ATTRIBUTES:    Env("OTEL_RESOURCE_ATTRIBUTES", "service.name=multifinance,service.namespace=multifinance-group,deployment.environment=production"),
		LOG_LEVEL:                   Env("LOG_LEVEL", "info"),
		METRIC_INTERVAL:             Duration("METRIC_INTERVAL", 15*time.Second),
		RUNTIME_METRICS:             Bool("RUNTIME_METRICS", true),
		REQUESTS_METRIC:             Bool("REQUESTS_METRIC", true),
		DEVELOPMENT_MODE:            Bool("DEVELOPMENT_MODE", false),
		SERVER_PORT:                 Env("SERVER_PORT", "3001"),
		CLOUDINARY_CLOUD:            Env("CLOUDINARY_CLOUD", ""),
		CLOUDINARY_API_KEY:          Env("CLOUDINARY_API_KEY", ""),
		CLOUDINARY_API_SECRET:       Env("CLOUDINARY_API_SECRET", ""),
		MYSQL_HOST:                  Env("MYSQL_HOST", "127.0.0.1"),
		MYSQL_PORT:                  Env("MYSQL_PORT", "3306"),
		MYSQL_USER:                  Env("MYSQL_USER", "root"),
		MYSQL_PASSWORD:              Env("MYSQL_PASSWORD", ""),
		MYSQL_DBNAME:                Env("MYSQL_DBNAME", "loan_system"),
		REDIS_ADDRESS:               Env("REDIS_ADDRESS", "localhost:6379"),
		REDIS_PASSWORD:              Env("REDIS_PASSWORD", ""),
		JWT_SECRET_KEY:              Env("JWT_SECRET_KEY", ""),
		SHUTDOWN_TIMEOUT:            Duration("SHUTDOWN_TIMEOUT", 15*time.Second),
	}

	return config, nil
}
