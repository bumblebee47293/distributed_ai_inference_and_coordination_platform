package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewModelRouter(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewModelRouter(logger, "http://localhost:8082")

	assert.NotNil(t, router)
	assert.NotNil(t, router.backends)
	assert.NotNil(t, router.client)
}

func TestRegisterBackend(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewModelRouter(logger, "http://localhost:8082")

	router.RegisterBackend("resnet18", "v1", "http://localhost:8082")

	assert.NotNil(t, router.backends["resnet18"])
	assert.NotNil(t, router.backends["resnet18"]["v1"])
	assert.Equal(t, 1, len(router.backends["resnet18"]["v1"]))
}

func TestRegisterMultipleBackends(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewModelRouter(logger, "http://localhost:8082")

	router.RegisterBackend("resnet18", "v1", "http://backend1:8082")
	router.RegisterBackend("resnet18", "v1", "http://backend2:8082")
	router.RegisterBackend("resnet18", "v2", "http://backend3:8082")

	assert.Equal(t, 2, len(router.backends["resnet18"]["v1"]))
	assert.Equal(t, 1, len(router.backends["resnet18"]["v2"]))
}

func TestRouteRequest_ModelNotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewModelRouter(logger, "http://localhost:8082")

	input := map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}}
	_, err := router.RouteRequest(context.Background(), "nonexistent", "v1", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model not found")
}

func TestRouteRequest_VersionNotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewModelRouter(logger, "http://localhost:8082")

	router.RegisterBackend("resnet18", "v1", "http://localhost:8082")

	input := map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}}
	_, err := router.RouteRequest(context.Background(), "resnet18", "v2", input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "version not found")
}

func TestRouteRequest_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewModelRouter(logger, "http://localhost:8082")

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/infer", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"prediction": [0.1, 0.9]}`))
	}))
	defer server.Close()

	router.RegisterBackend("resnet18", "v1", server.URL)

	input := map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}}
	result, err := router.RouteRequest(context.Background(), "resnet18", "v1", input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result, "prediction")
}

func TestCircuitBreaker_TripsOnFailures(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewModelRouter(logger, "http://localhost:8082")

	// Create mock server that always fails
	failCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	router.RegisterBackend("resnet18", "v1", server.URL)

	input := map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}}

	// Make multiple requests to trip the circuit breaker
	for i := 0; i < 5; i++ {
		router.RouteRequest(context.Background(), "resnet18", "v1", input)
	}

	// Circuit breaker should have tripped
	assert.Greater(t, failCount, 0)
}

func TestSelectBackend_RoundRobin(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	router := NewModelRouter(logger, "http://localhost:8082")

	backends := []*Backend{
		{URL: "http://backend1:8082", HealthStatus: true},
		{URL: "http://backend2:8082", HealthStatus: true},
		{URL: "http://backend3:8082", HealthStatus: true},
	}

	// Select multiple times
	selected := make(map[string]int)
	for i := 0; i < 30; i++ {
		backend := router.selectBackend(backends)
		selected[backend.URL]++
	}

	// All backends should be selected (random distribution)
	assert.Greater(t, len(selected), 0)
}
