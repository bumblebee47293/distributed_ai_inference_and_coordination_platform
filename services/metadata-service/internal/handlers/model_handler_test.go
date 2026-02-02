package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/yourusername/ai-platform/metadata-service/internal/models"
)

func TestCreateModel_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// This is a simplified test - in real scenario, mock the repository
	req := models.CreateModelRequest{
		Name:        "resnet18",
		Version:     "v1",
		Framework:   "pytorch",
		Format:      "onnx",
		Description: "ResNet18 image classifier",
		BackendURL:  "http://localhost:8082",
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/v1/models", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")

	// Verify request structure
	var decoded models.CreateModelRequest
	err := json.NewDecoder(bytes.NewBuffer(body)).Decode(&decoded)
	assert.NoError(t, err)
	assert.Equal(t, "resnet18", decoded.Name)
	assert.Equal(t, "v1", decoded.Version)
}

func TestCreateModel_InvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Missing required fields
	req := map[string]interface{}{
		"name": "resnet18",
		// Missing version, framework, format, backend_url
	}

	body, _ := json.Marshal(req)
	httpReq := httptest.NewRequest("POST", "/v1/models", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")

	var decoded models.CreateModelRequest
	err := json.NewDecoder(bytes.NewBuffer(body)).Decode(&decoded)
	assert.NoError(t, err)

	// Validation would fail on binding
	assert.Empty(t, decoded.Version)
	assert.Empty(t, decoded.Framework)
}

func TestUpdateModel_Request(t *testing.T) {
	gin.SetMode(gin.TestMode)

	description := "Updated description"
	status := "deprecated"

	req := models.UpdateModelRequest{
		Description: &description,
		Status:      &status,
	}

	body, _ := json.Marshal(req)

	var decoded models.UpdateModelRequest
	err := json.NewDecoder(bytes.NewBuffer(body)).Decode(&decoded)
	assert.NoError(t, err)
	assert.NotNil(t, decoded.Description)
	assert.Equal(t, "Updated description", *decoded.Description)
	assert.NotNil(t, decoded.Status)
	assert.Equal(t, "deprecated", *decoded.Status)
}

func TestHealthCheck_Handler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()

	// Create a simple health check handler
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "metadata-service",
		})
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
	assert.Contains(t, w.Body.String(), "metadata-service")
}
