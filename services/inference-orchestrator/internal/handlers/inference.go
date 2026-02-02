package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/yourusername/ai-platform/inference-orchestrator/internal/triton"
)

type InferenceHandler struct {
	logger       *zap.Logger
	tritonClient *triton.Client
}

func NewInferenceHandler(logger *zap.Logger, tritonClient *triton.Client) *InferenceHandler {
	return &InferenceHandler{
		logger:       logger,
		tritonClient: tritonClient,
	}
}

type InferRequest struct {
	Model   string                 `json:"model" binding:"required"`
	Version string                 `json:"version"`
	Input   map[string]interface{} `json:"input" binding:"required"`
}

func (h *InferenceHandler) Infer(c *gin.Context) {
	var req InferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Version == "" {
		req.Version = "1"
	}

	h.logger.Info("processing inference",
		zap.String("model", req.Model),
		zap.String("version", req.Version),
	)

	result, err := h.tritonClient.Infer(c.Request.Context(), req.Model, req.Version, req.Input)
	if err != nil {
		h.logger.Error("inference failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "inference failed"})
		return
	}

	c.JSON(http.StatusOK, result)
}
