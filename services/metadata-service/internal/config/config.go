package config

import (
	"fmt"
	"os"

	"github.com/redis/go-redis/v9"
)

// Config holds the metadata service configuration
type Config struct {
	ServiceName    string
	Port           string
	PostgresURL    string
	RedisHost      string
	JaegerEndpoint string
	LogLevel       string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		ServiceName:    getEnv("SERVICE_NAME", "metadata-service"),
		Port:           getEnv("PORT", "8083"),
		PostgresURL:    getEnv("POSTGRES_URL", "postgres://postgres:postgres@localhost:5432/ai_platform?sslmode=disable"),
		RedisHost:      getEnv("REDIS_HOST", "localhost:6379"),
		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
	}
}

// NewRedisClient creates a new Redis client
func NewRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
