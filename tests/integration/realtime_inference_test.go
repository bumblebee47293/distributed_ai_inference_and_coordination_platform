package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealtimeInference tests the complete real-time inference flow:
// API Gateway -> Model Router -> Inference Orchestrator -> Triton
func TestRealtimeInference(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Test configuration
	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")
	
	tests := []struct {
		name           string
		model          string
		version        string
		input          map[string]interface{}
		expectedStatus int
		expectError    bool
	}{
		{
			name:    "successful inference",
			model:   "resnet18",
			version: "1",
			input: map[string]interface{}{
				"data": []float64{1.0, 2.0, 3.0, 4.0},
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "missing model",
			model:          "nonexistent",
			version:        "1",
			input:          map[string]interface{}{"data": []float64{1.0}},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:           "invalid input",
			model:          "resnet18",
			version:        "1",
			input:          map[string]interface{}{},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Prepare request
			reqBody := map[string]interface{}{
				"model":   tt.model,
				"version": tt.version,
				"input":   tt.input,
			}

			jsonData, err := json.Marshal(reqBody)
			require.NoError(t, err)

			// Make request to API Gateway
			req, err := http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(jsonData))
			require.NoError(t, err)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer demo-token")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Verify response
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var response map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)

			if tt.expectError {
				assert.Contains(t, response, "error")
			} else {
				assert.Contains(t, response, "prediction")
				assert.Contains(t, response, "latency_ms")
				assert.Contains(t, response, "model")
			}
		})
	}
}

// TestRealtimeInferenceWithTracing verifies distributed tracing
func TestRealtimeInferenceWithTracing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")

	reqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"input":   map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer demo-token")
	req.Header.Set("X-Request-ID", "test-trace-123")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify trace ID is returned
	traceID := resp.Header.Get("X-Trace-ID")
	assert.NotEmpty(t, traceID, "Trace ID should be present in response headers")
}

// TestRealtimeInferenceLatency tests performance characteristics
func TestRealtimeInferenceLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")

	reqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"input":   map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}},
	}

	jsonData, _ := json.Marshal(reqBody)

	// Measure latency
	start := time.Now()
	
	req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer demo-token")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	latency := time.Since(start)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Less(t, latency, 5*time.Second, "Inference should complete within 5 seconds")

	t.Logf("Inference latency: %v", latency)
}

// TestConcurrentInferences tests system under concurrent load
func TestConcurrentInferences(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")
	concurrency := 10

	reqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"input":   map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}},
	}

	jsonData, _ := json.Marshal(reqBody)

	// Channel to collect results
	results := make(chan error, concurrency)

	// Launch concurrent requests
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer demo-token")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				results <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- assert.AnError
				return
			}

			results <- nil
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < concurrency; i++ {
		err := <-results
		if err == nil {
			successCount++
		}
	}

	// At least 80% should succeed
	assert.GreaterOrEqual(t, successCount, concurrency*8/10, "At least 80% of concurrent requests should succeed")
	t.Logf("Concurrent requests: %d/%d succeeded", successCount, concurrency)
}

func getEnv(key, defaultValue string) string {
	// In a real implementation, use os.Getenv
	return defaultValue
}
