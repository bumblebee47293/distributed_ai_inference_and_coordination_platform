package triton

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewClient(logger, "localhost:8001")

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:8001", client.baseURL)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
}

func TestClient_BaseURL(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name     string
		tritonURL string
		expected string
	}{
		{
			name:      "localhost",
			tritonURL: "localhost:8001",
			expected:  "http://localhost:8001",
		},
		{
			name:      "IP address",
			tritonURL: "192.168.1.100:8001",
			expected:  "http://192.168.1.100:8001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(logger, tt.tritonURL)
			assert.Equal(t, tt.expected, client.baseURL)
		})
	}
}

func TestClient_Infer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewClient(logger, "localhost:8001")

	ctx := context.Background()
	input := map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}}

	result, err := client.Infer(ctx, "resnet18", "v1", input)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "resnet18", result["model_name"])
	assert.Equal(t, "v1", result["model_version"])
	assert.NotNil(t, result["prediction"])
}

func TestClient_InferHTTP_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/models/resnet18/infer", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"model_name":"resnet18","outputs":[{"class":"cat"}]}`))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	client := NewClient(logger, server.URL[7:]) // Remove "http://" prefix

	ctx := context.Background()
	input := map[string]interface{}{"data": []float64{1.0}}

	result, err := client.InferHTTP(ctx, "resnet18", "v1", input)
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestClient_InferHTTP_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"model not found"}`))
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	client := NewClient(logger, server.URL[7:])

	ctx := context.Background()
	input := map[string]interface{}{"data": []float64{1.0}}

	_, err := client.InferHTTP(ctx, "unknown", "v1", input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestClient_HealthCheck_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/health/ready", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	client := NewClient(logger, server.URL[7:])

	err := client.HealthCheck(context.Background())
	assert.NoError(t, err)
}

func TestClient_HealthCheck_NotReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	logger, _ := zap.NewDevelopment()
	client := NewClient(logger, server.URL[7:])

	err := client.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not ready")
}

func TestClient_ContextCancellation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewClient(logger, "localhost:8001")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.HealthCheck(ctx)
	assert.Error(t, err)
}
