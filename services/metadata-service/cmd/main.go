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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yourusername/ai-platform/metadata-service/internal/cache"
	"github.com/yourusername/ai-platform/metadata-service/internal/config"
	"github.com/yourusername/ai-platform/metadata-service/internal/handlers"
	"github.com/yourusername/ai-platform/metadata-service/internal/repository"
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
		zap.String("port", cfg.Port),
	)

	// Initialize PostgreSQL repository
	repo, err := repository.NewModelRepository(cfg.PostgresURL, logger)
	if err != nil {
		logger.Fatal("failed to initialize repository", zap.Error(err))
	}
	defer repo.Close()
	logger.Info("connected to PostgreSQL")

	// Initialize Redis cache
	redisClient := config.NewRedisClient(cfg.RedisHost)
	defer redisClient.Close()

	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("failed to connect to redis", zap.Error(err))
	}
	logger.Info("connected to Redis")

	modelCache := cache.NewModelCache(redisClient, logger)

	// Initialize handlers
	modelHandler := handlers.NewModelHandler(repo, modelCache, logger)

	// Setup router
	if cfg.LogLevel == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Health check
	router.GET("/health", modelHandler.HealthCheck)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API v1 routes
	v1 := router.Group("/v1")
	{
		// Model routes
		models := v1.Group("/models")
		{
			models.POST("", modelHandler.CreateModel)
			models.GET("", modelHandler.ListModels)
			models.GET("/:id", modelHandler.GetModel)
			models.PUT("/:id", modelHandler.UpdateModel)
			models.DELETE("/:id", modelHandler.DeleteModel)
			models.GET("/by-name/:name/:version", modelHandler.GetModelByNameVersion)
		}
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
		logger.Info("starting metadata service", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}
