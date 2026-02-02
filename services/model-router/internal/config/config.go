package config

import "os"

type Config struct {
	ServiceName     string
	Port            string
	LogLevel        string
	OrchestratorURL string
	MetadataURL     string
	JaegerEndpoint  string
}

func Load() *Config {
	return &Config{
		ServiceName:     getEnv("SERVICE_NAME", "model-router"),
		Port:            getEnv("PORT", "8081"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		OrchestratorURL: getEnv("ORCHESTRATOR_SERVICE_URL", "http://localhost:8082"),
		MetadataURL:     getEnv("METADATA_SERVICE_URL", "http://localhost:8083"),
		JaegerEndpoint:  getEnv("JAEGER_ENDPOINT", "http://localhost:14268/api/traces"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
