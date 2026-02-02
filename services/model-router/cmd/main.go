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

	"github.com/yourusername/ai-platform/model-router/internal/config"
	"github.com/yourusername/ai-platform/model-router/internal/handlers"
	"github.com/yourusername/ai-platform/model-router/internal/router"
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
	logger.Info("configuration loaded", zap.String("port", cfg.Port))

	// Initialize model router
	modelRouter := router.NewModelRouter(logger, cfg.OrchestratorURL)

	// Register models (in production, this would come from metadata service)
	modelRouter.RegisterBackend("resnet18", "v1", cfg.OrchestratorURL)
	modelRouter.RegisterBackend("resnet18", "v2", cfg.OrchestratorURL)

	// Setup HTTP router
	if cfg.LogLevel == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Routing endpoints
	routeHandler := handlers.NewRouteHandler(logger, modelRouter)
	v1 := r.Group("/v1")
	{
		v1.POST("/route", routeHandler.RouteInference)
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Start server
	go func() {
		logger.Info("starting model router", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("server forced to shutdown", zap.Error(err))
	}

	logger.Info("server exited")
}
