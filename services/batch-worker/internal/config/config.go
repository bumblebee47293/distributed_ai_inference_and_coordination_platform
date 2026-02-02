package config

import (
	"fmt"
	"os"
)

// Config holds the batch worker configuration
type Config struct {
	ServiceName     string
	KafkaBrokers    []string
	KafkaTopic      string
	ConsumerGroup   string
	PostgresURL     string
	MinIOEndpoint   string
	MinIOAccessKey  string
	MinIOSecretKey  string
	MinioBucket     string
	WorkerPoolSize  int
	JaegerEndpoint  string
	LogLevel        string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		ServiceName:    getEnv("SERVICE_NAME", "batch-worker"),
		KafkaBrokers:   []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
		KafkaTopic:     getEnv("KAFKA_TOPIC", "batch-inference"),
		ConsumerGroup:  getEnv("CONSUMER_GROUP", "batch-worker-group"),
		PostgresURL:    getEnv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/ai_platform?sslmode=disable"),
		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:    getEnv("MINIO_BUCKET", "inference-results"),
		WorkerPoolSize: getEnvInt("WORKER_POOL_SIZE", 10),
		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
