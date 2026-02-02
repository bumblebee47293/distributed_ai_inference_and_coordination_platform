package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yourusername/ai-platform/metadata-service/internal/cache"
	"github.com/yourusername/ai-platform/metadata-service/internal/models"
	"github.com/yourusername/ai-platform/metadata-service/internal/repository"
	"go.uber.org/zap"
)

// ModelHandler handles model metadata HTTP requests
type ModelHandler struct {
	repo   *repository.ModelRepository
	cache  *cache.ModelCache
	logger *zap.Logger
}

// NewModelHandler creates a new model handler
func NewModelHandler(repo *repository.ModelRepository, cache *cache.ModelCache, logger *zap.Logger) *ModelHandler {
	return &ModelHandler{
		repo:   repo,
		cache:  cache,
		logger: logger,
	}
}

// CreateModel creates a new model
func (h *ModelHandler) CreateModel(c *gin.Context) {
	var req models.CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	model, err := h.repo.Create(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("failed to create model", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create model"})
		return
	}

	// Cache the new model
	cacheKey := model.ID
	if err := h.cache.Set(c.Request.Context(), cacheKey, model); err != nil {
		h.logger.Warn("failed to cache model", zap.Error(err))
	}

	c.JSON(http.StatusCreated, model)
}

// GetModel retrieves a model by ID
func (h *ModelHandler) GetModel(c *gin.Context) {
	id := c.Param("id")

	// Try cache first
	model, err := h.cache.Get(c.Request.Context(), id)
	if err != nil {
		h.logger.Warn("cache error", zap.Error(err))
	}

	if model == nil {
		// Cache miss, get from database
		model, err = h.repo.GetByID(c.Request.Context(), id)
		if err != nil {
			h.logger.Error("failed to get model", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}

		// Cache the result
		if err := h.cache.Set(c.Request.Context(), id, model); err != nil {
			h.logger.Warn("failed to cache model", zap.Error(err))
		}
	}

	c.JSON(http.StatusOK, model)
}

// GetModelByNameVersion retrieves a model by name and version
func (h *ModelHandler) GetModelByNameVersion(c *gin.Context) {
	name := c.Param("name")
	version := c.Param("version")

	cacheKey := name + ":" + version

	// Try cache first
	model, err := h.cache.Get(c.Request.Context(), cacheKey)
	if err != nil {
		h.logger.Warn("cache error", zap.Error(err))
	}

	if model == nil {
		// Cache miss, get from database
		model, err = h.repo.GetByNameVersion(c.Request.Context(), name, version)
		if err != nil {
			h.logger.Error("failed to get model",
				zap.String("name", name),
				zap.String("version", version),
				zap.Error(err),
			)
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found"})
			return
		}

		// Cache the result
		if err := h.cache.Set(c.Request.Context(), cacheKey, model); err != nil {
			h.logger.Warn("failed to cache model", zap.Error(err))
		}
	}

	c.JSON(http.StatusOK, model)
}

// ListModels lists all models with optional filtering
func (h *ModelHandler) ListModels(c *gin.Context) {
	status := c.DefaultQuery("status", "")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	models, err := h.repo.List(c.Request.Context(), status, limit, offset)
	if err != nil {
		h.logger.Error("failed to list models", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list models"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"models": models,
		"count":  len(models),
		"limit":  limit,
		"offset": offset,
	})
}

// UpdateModel updates a model
func (h *ModelHandler) UpdateModel(c *gin.Context) {
	id := c.Param("id")

	var req models.UpdateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	model, err := h.repo.Update(c.Request.Context(), id, &req)
	if err != nil {
		h.logger.Error("failed to update model", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update model"})
		return
	}

	// Invalidate cache
	if err := h.cache.Delete(c.Request.Context(), id); err != nil {
		h.logger.Warn("failed to invalidate cache", zap.Error(err))
	}

	// Also invalidate name:version cache
	cacheKey := model.Name + ":" + model.Version
	if err := h.cache.Delete(c.Request.Context(), cacheKey); err != nil {
		h.logger.Warn("failed to invalidate cache", zap.Error(err))
	}

	c.JSON(http.StatusOK, model)
}

// DeleteModel deletes a model
func (h *ModelHandler) DeleteModel(c *gin.Context) {
	id := c.Param("id")

	// Get model first to invalidate caches
	model, _ := h.repo.GetByID(c.Request.Context(), id)

	if err := h.repo.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("failed to delete model", zap.String("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete model"})
		return
	}

	// Invalidate caches
	if err := h.cache.Delete(c.Request.Context(), id); err != nil {
		h.logger.Warn("failed to invalidate cache", zap.Error(err))
	}

	if model != nil {
		cacheKey := model.Name + ":" + model.Version
		if err := h.cache.Delete(c.Request.Context(), cacheKey); err != nil {
			h.logger.Warn("failed to invalidate cache", zap.Error(err))
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "model deleted successfully"})
}

// HealthCheck returns service health status
func (h *ModelHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"service": "metadata-service",
	})
}
