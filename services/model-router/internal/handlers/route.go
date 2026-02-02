package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/yourusername/ai-platform/model-router/internal/router"
)

type RouteHandler struct {
	logger *zap.Logger
	router *router.ModelRouter
}

func NewRouteHandler(logger *zap.Logger, router *router.ModelRouter) *RouteHandler {
	return &RouteHandler{
		logger: logger,
		router: router,
	}
}

type RouteRequest struct {
	RequestID string                 `json:"request_id"`
	Model     string                 `json:"model" binding:"required"`
	Version   string                 `json:"version"`
	Input     map[string]interface{} `json:"input" binding:"required"`
}

func (h *RouteHandler) RouteInference(c *gin.Context) {
	var req RouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Version == "" {
		req.Version = "v1"
	}

	h.logger.Info("routing inference request",
		zap.String("request_id", req.RequestID),
		zap.String("model", req.Model),
		zap.String("version", req.Version),
	)

	result, err := h.router.RouteRequest(c.Request.Context(), req.Model, req.Version, req.Input)
	if err != nil {
		h.logger.Error("routing failed", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
