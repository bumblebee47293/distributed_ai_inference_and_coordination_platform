package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/IBM/sarama"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// InferenceRequest represents a real-time inference request
type InferenceRequest struct {
	Model   string                 `json:"model" binding:"required"`
	Version string                 `json:"version"`
	Input   map[string]interface{} `json:"input" binding:"required"`
}

// BatchInferenceRequest represents a batch inference request
type BatchInferenceRequest struct {
	Model   string                   `json:"model" binding:"required"`
	Version string                   `json:"version"`
	Inputs  []map[string]interface{} `json:"inputs" binding:"required"`
}

// InferenceResponse represents the inference response
type InferenceResponse struct {
	RequestID  string                 `json:"request_id"`
	Model      string                 `json:"model"`
	Version    string                 `json:"version"`
	Prediction map[string]interface{} `json:"prediction"`
	Latency    int64                  `json:"latency_ms"`
}

// BatchJobResponse represents a batch job submission response
type BatchJobResponse struct {
	JobID     string    `json:"job_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// JobStatusResponse represents job status
type JobStatusResponse struct {
	JobID      string    `json:"job_id"`
	Status     string    `json:"status"`
	Progress   float64   `json:"progress"`
	TotalItems int       `json:"total_items"`
	Completed  int       `json:"completed"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ResultURL  string    `json:"result_url,omitempty"`
}

// InferenceHandler handles inference requests
type InferenceHandler struct {
	logger          *zap.Logger
	routerURL       string
	kafkaProducer   sarama.SyncProducer
	kafkaTopic      string
	httpClient      *http.Client
}

// NewInferenceHandler creates a new inference handler
func NewInferenceHandler(
	logger *zap.Logger,
	routerURL string,
	kafkaProducer sarama.SyncProducer,
	kafkaTopic string,
) *InferenceHandler {
	return &InferenceHandler{
		logger:        logger,
		routerURL:     routerURL,
		kafkaProducer: kafkaProducer,
		kafkaTopic:    kafkaTopic,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RealTimeInference handles synchronous inference requests
func (h *InferenceHandler) RealTimeInference(c *gin.Context) {
	ctx := c.Request.Context()
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(ctx, "RealTimeInference")
	defer span.End()

	requestID := uuid.New().String()
	startTime := time.Now()

	var req InferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	// Set default version if not provided
	if req.Version == "" {
		req.Version = "v1"
	}

	span.SetAttributes(
		attribute.String("model", req.Model),
		attribute.String("version", req.Version),
		attribute.String("request_id", requestID),
	)

	h.logger.Info("processing inference request",
		zap.String("request_id", requestID),
		zap.String("model", req.Model),
		zap.String("version", req.Version),
	)

	// Forward request to model router
	routerReq := map[string]interface{}{
		"request_id": requestID,
		"model":      req.Model,
		"version":    req.Version,
		"input":      req.Input,
	}

	reqBody, err := json.Marshal(routerReq)
	if err != nil {
		h.logger.Error("failed to marshal request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		"POST",
		h.routerURL+"/v1/route",
		bytes.NewBuffer(reqBody),
	)
	if err != nil {
		h.logger.Error("failed to create request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Request-ID", requestID)

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		h.logger.Error("failed to forward request", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "service unavailable"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		h.logger.Error("router returned error",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)
		c.JSON(resp.StatusCode, gin.H{"error": "inference failed"})
		return
	}

	var routerResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&routerResp); err != nil {
		h.logger.Error("failed to decode response", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	latency := time.Since(startTime).Milliseconds()

	response := InferenceResponse{
		RequestID:  requestID,
		Model:      req.Model,
		Version:    req.Version,
		Prediction: routerResp,
		Latency:    latency,
	}

	h.logger.Info("inference completed",
		zap.String("request_id", requestID),
		zap.Int64("latency_ms", latency),
	)

	c.JSON(http.StatusOK, response)
}

// BatchInference handles batch inference job submission
func (h *InferenceHandler) BatchInference(c *gin.Context) {
	ctx := c.Request.Context()
	tracer := otel.Tracer("api-gateway")
	ctx, span := tracer.Start(ctx, "BatchInference")
	defer span.End()

	var req BatchInferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("invalid request", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "details": err.Error()})
		return
	}

	// Set default version if not provided
	if req.Version == "" {
		req.Version = "v1"
	}

	jobID := uuid.New().String()

	span.SetAttributes(
		attribute.String("model", req.Model),
		attribute.String("version", req.Version),
		attribute.String("job_id", jobID),
		attribute.Int("input_count", len(req.Inputs)),
	)

	h.logger.Info("submitting batch job",
		zap.String("job_id", jobID),
		zap.String("model", req.Model),
		zap.Int("input_count", len(req.Inputs)),
	)

	// Create job message
	job := map[string]interface{}{
		"job_id":     jobID,
		"model":      req.Model,
		"version":    req.Version,
		"inputs":     req.Inputs,
		"created_at": time.Now().UTC(),
	}

	jobBytes, err := json.Marshal(job)
	if err != nil {
		h.logger.Error("failed to marshal job", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	// Send to Kafka
	msg := &sarama.ProducerMessage{
		Topic: h.kafkaTopic,
		Key:   sarama.StringEncoder(jobID),
		Value: sarama.ByteEncoder(jobBytes),
	}

	partition, offset, err := h.kafkaProducer.SendMessage(msg)
	if err != nil {
		h.logger.Error("failed to send message to kafka", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit job"})
		return
	}

	h.logger.Info("batch job submitted",
		zap.String("job_id", jobID),
		zap.Int32("partition", partition),
		zap.Int64("offset", offset),
	)

	response := BatchJobResponse{
		JobID:     jobID,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}

	c.JSON(http.StatusAccepted, response)
}

// GetJobStatus retrieves the status of a batch job
func (h *InferenceHandler) GetJobStatus(c *gin.Context) {
	jobID := c.Param("id")

	h.logger.Info("retrieving job status", zap.String("job_id", jobID))

	// TODO: Query metadata service or database for job status
	// For now, return a mock response
	response := JobStatusResponse{
		JobID:      jobID,
		Status:     "processing",
		Progress:   0.45,
		TotalItems: 100,
		Completed:  45,
		CreatedAt:  time.Now().Add(-5 * time.Minute).UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	c.JSON(http.StatusOK, response)
}
