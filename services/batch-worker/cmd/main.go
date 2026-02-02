package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/ai-platform/batch-worker/internal/config"
	"github.com/yourusername/ai-platform/batch-worker/internal/consumer"
	"github.com/yourusername/ai-platform/batch-worker/internal/storage"
	"github.com/yourusername/ai-platform/batch-worker/internal/worker"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	// Load configuration
	cfg := config.Load()
	logger.Info("configuration loaded",
		zap.String("service", cfg.ServiceName),
		zap.Strings("kafka_brokers", cfg.KafkaBrokers),
		zap.String("topic", cfg.KafkaTopic),
		zap.Int("worker_pool_size", cfg.WorkerPoolSize),
	)

	// Initialize PostgreSQL store
	pgStore, err := storage.NewPostgresStore(cfg.PostgresURL, logger)
	if err != nil {
		logger.Fatal("failed to initialize postgres store", zap.Error(err))
	}
	defer pgStore.Close()
	logger.Info("connected to PostgreSQL")

	// Initialize MinIO store
	minioStore, err := storage.NewMinIOStore(
		cfg.MinIOEndpoint,
		cfg.MinIOAccessKey,
		cfg.MinIOSecretKey,
		cfg.MinioBucket,
		logger,
	)
	if err != nil {
		logger.Fatal("failed to initialize minio store", zap.Error(err))
	}
	logger.Info("connected to MinIO")

	// Create worker pool
	orchestratorURL := getEnv("ORCHESTRATOR_URL", "http://localhost:8082")
	pool := worker.NewPool(cfg.WorkerPoolSize, orchestratorURL, pgStore, minioStore, logger)
	logger.Info("worker pool created", zap.Int("size", cfg.WorkerPoolSize))

	// Create Kafka consumer
	kafkaConsumer, err := consumer.NewKafkaConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaTopic,
		cfg.ConsumerGroup,
		pool,
		pgStore,
		logger,
	)
	if err != nil {
		logger.Fatal("failed to create kafka consumer", zap.Error(err))
	}
	logger.Info("kafka consumer created")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consumer in goroutine
	go func() {
		if err := kafkaConsumer.Start(ctx); err != nil {
			logger.Error("kafka consumer error", zap.Error(err))
		}
	}()

	logger.Info("batch worker started successfully")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down batch worker...")
	cancel()

	logger.Info("batch worker exited")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
