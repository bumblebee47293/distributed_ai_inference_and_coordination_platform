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

	"github.com/yourusername/ai-platform/inference-orchestrator/internal/config"
	"github.com/yourusername/ai-platform/inference-orchestrator/internal/handlers"
	"github.com/yourusername/ai-platform/inference-orchestrator/internal/triton"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer logger.Sync()

	cfg := config.Load()
	logger.Info("configuration loaded", zap.String("port", cfg.Port))

	// Initialize Triton client
	tritonClient := triton.NewClient(logger, cfg.TritonURL)

	// Setup router
	if cfg.LogLevel == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	inferHandler := handlers.NewInferenceHandler(logger, tritonClient)
	v1 := r.Group("/v1")
	{
		v1.POST("/infer", inferHandler.Infer)
	}

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		logger.Info("starting inference orchestrator", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

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
