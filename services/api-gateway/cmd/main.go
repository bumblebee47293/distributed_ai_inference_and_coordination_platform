package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/yourusername/ai-platform/api-gateway/internal/config"
	"github.com/yourusername/ai-platform/api-gateway/internal/handlers"
	"github.com/yourusername/ai-platform/api-gateway/internal/middleware"
	"github.com/yourusername/ai-platform/api-gateway/internal/observability"
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
		zap.String("port", cfg.Port),
		zap.String("log_level", cfg.LogLevel),
	)

	// Initialize observability
	shutdown, err := observability.InitTracing(cfg.ServiceName, cfg.JaegerEndpoint)
	if err != nil {
		logger.Fatal("failed to initialize tracing", zap.Error(err))
	}
	defer shutdown(context.Background())

	observability.InitMetrics()

	// Initialize dependencies
	redisClient := config.NewRedisClient(cfg.RedisHost)
	defer redisClient.Close()

	kafkaProducer, err := config.NewKafkaProducer(cfg.KafkaBrokers)
	if err != nil {
		logger.Fatal("failed to initialize kafka producer", zap.Error(err))
	}
	defer kafkaProducer.Close()

	// Setup router
	if cfg.LogLevel == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()

	// Global middleware
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recovery(logger))
	router.Use(middleware.Tracing())
	router.Use(middleware.Metrics())
	router.Use(middleware.CORS())

	// Health check endpoint (no auth required)
	router.GET("/health", handlers.HealthCheck())
	router.GET("/metrics", handlers.MetricsHandler())

	// API v1 routes
	v1 := router.Group("/v1")
	{
		// Apply authentication and rate limiting
		v1.Use(middleware.Auth(cfg.JWTSecret))
		v1.Use(middleware.RateLimit(redisClient, 100, time.Minute))

		// Inference endpoints
		inferenceHandler := handlers.NewInferenceHandler(
			logger,
			cfg.RouterServiceURL,
			kafkaProducer,
			cfg.KafkaTopic,
		)
		v1.POST("/infer", inferenceHandler.RealTimeInference)
		v1.POST("/batch", inferenceHandler.BatchInference)
		v1.GET("/jobs/:id", inferenceHandler.GetJobStatus)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("starting api gateway", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}
