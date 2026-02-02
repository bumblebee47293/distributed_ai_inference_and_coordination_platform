package config

import (
	"os"
	"strings"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
)

// Config holds application configuration
type Config struct {
	// Server
	ServiceName string
	Port        string
	LogLevel    string

	// Authentication
	JWTSecret string

	// Dependencies
	RedisHost         string
	RouterServiceURL  string
	MetadataServiceURL string
	KafkaBrokers      []string
	KafkaTopic        string

	// Observability
	JaegerEndpoint string
}

// Load reads configuration from environment variables
func Load() *Config {
	return &Config{
		ServiceName:        getEnv("SERVICE_NAME", "api-gateway"),
		Port:               getEnv("PORT", "8080"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		JWTSecret:          getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		RedisHost:          getEnv("REDIS_HOST", "localhost:6379"),
		RouterServiceURL:   getEnv("ROUTER_SERVICE_URL", "http://localhost:8081"),
		MetadataServiceURL: getEnv("METADATA_SERVICE_URL", "http://localhost:8083"),
		KafkaBrokers:       strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ","),
		KafkaTopic:         getEnv("KAFKA_TOPIC", "inference-jobs"),
		JaegerEndpoint:     getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// NewRedisClient creates a new Redis client
func NewRedisClient(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

// NewKafkaProducer creates a new Kafka producer
func NewKafkaProducer(brokers []string) (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Retry.Max = 5
	config.Producer.Return.Successes = true

	return sarama.NewSyncProducer(brokers, config)
}
