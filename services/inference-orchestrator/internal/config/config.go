package config

import "os"

type Config struct {
	ServiceName    string
	Port           string
	LogLevel       string
	TritonURL      string
	JaegerEndpoint string
}

func Load() *Config {
	return &Config{
		ServiceName:    getEnv("SERVICE_NAME", "inference-orchestrator"),
		Port:           getEnv("PORT", "8082"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		TritonURL:      getEnv("TRITON_URL", "localhost:8001"),
		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
