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

// TestCircuitBreaker tests circuit breaker behavior under failure conditions
func TestCircuitBreaker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")
	client := &http.Client{Timeout: 5 * time.Second}

	// Test with a model that will fail (non-existent backend)
	reqBody := map[string]interface{}{
		"model":   "failing-model",
		"version": "1",
		"input":   map[string]interface{}{"data": []float64{1.0, 2.0}},
	}

	jsonData, _ := json.Marshal(reqBody)

	// Send multiple failing requests to trip the circuit breaker
	failureCount := 0
	circuitOpenCount := 0

	for i := 0; i < 10; i++ {
		req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer demo-token")

		resp, err := client.Do(req)
		if err != nil {
			failureCount++
			continue
		}

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)
		resp.Body.Close()

		if resp.StatusCode >= 500 {
			failureCount++
			
			// Check if error message indicates circuit breaker is open
			if errorMsg, ok := response["error"].(string); ok {
				if contains(errorMsg, "circuit") || contains(errorMsg, "breaker") {
					circuitOpenCount++
				}
			}
		}

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Failures: %d, Circuit breaker trips: %d", failureCount, circuitOpenCount)
	
	// Circuit breaker should have tripped at least once
	assert.Greater(t, circuitOpenCount, 0, "Circuit breaker should trip after consecutive failures")
}

// TestCircuitBreakerRecovery tests circuit breaker recovery
func TestCircuitBreakerRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")
	client := &http.Client{Timeout: 5 * time.Second}

	// First, trip the circuit breaker with failing requests
	failingReqBody := map[string]interface{}{
		"model":   "failing-model",
		"version": "1",
		"input":   map[string]interface{}{"data": []float64{1.0}},
	}

	jsonData, _ := json.Marshal(failingReqBody)

	// Send failing requests
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer demo-token")
		
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Log("Circuit breaker should be open now")

	// Wait for circuit breaker timeout (usually 30-60 seconds)
	t.Log("Waiting for circuit breaker to enter half-open state...")
	time.Sleep(35 * time.Second)

	// Try with a valid model - circuit breaker should allow test request
	validReqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"input":   map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}},
	}

	validJsonData, _ := json.Marshal(validReqBody)
	req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(validJsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer demo-token")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Circuit breaker should allow the request through in half-open state
	assert.NotEqual(t, http.StatusServiceUnavailable, resp.StatusCode,
		"Circuit breaker should allow test requests in half-open state")

	t.Logf("Recovery test response status: %d", resp.StatusCode)
}

// TestCircuitBreakerMetrics tests circuit breaker metrics exposure
func TestCircuitBreakerMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	modelRouterURL := getEnv("MODEL_ROUTER_URL", "http://localhost:8081")
	client := &http.Client{Timeout: 5 * time.Second}

	// Get metrics
	req, _ := http.NewRequest("GET", modelRouterURL+"/metrics", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Read metrics (Prometheus format)
	var metricsData bytes.Buffer
	metricsData.ReadFrom(resp.Body)
	metrics := metricsData.String()

	// Check for circuit breaker related metrics
	assert.NotEmpty(t, metrics, "Metrics should not be empty")
	
	// Common circuit breaker metric patterns
	hasCircuitMetrics := contains(metrics, "circuit") || 
		contains(metrics, "breaker") || 
		contains(metrics, "failure") ||
		contains(metrics, "success")

	assert.True(t, hasCircuitMetrics, "Metrics should contain circuit breaker information")

	t.Logf("Metrics endpoint accessible, contains circuit breaker data: %v", hasCircuitMetrics)
}

// TestBackendHealthTracking tests backend health status tracking
func TestBackendHealthTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	modelRouterURL := getEnv("MODEL_ROUTER_URL", "http://localhost:8081")
	client := &http.Client{Timeout: 5 * time.Second}

	// Get health status
	req, _ := http.NewRequest("GET", modelRouterURL+"/health", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var healthResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&healthResp)

	// Check health response structure
	assert.Contains(t, healthResp, "status")
	status := healthResp["status"].(string)
	assert.Contains(t, []string{"healthy", "degraded", "unhealthy"}, status)

	t.Logf("Model router health status: %s", status)

	// Check if backend health information is included
	if backends, ok := healthResp["backends"]; ok {
		t.Logf("Backend health information available: %v", backends)
	}
}

// TestLoadBalancing tests load distribution across multiple backends
func TestLoadBalancing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")
	client := &http.Client{Timeout: 5 * time.Second}

	reqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"input":   map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}},
	}

	jsonData, _ := json.Marshal(reqBody)

	// Send multiple requests
	requestCount := 20
	successCount := 0
	latencies := []time.Duration{}

	for i := 0; i < requestCount; i++ {
		start := time.Now()
		
		req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer demo-token")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		latency := time.Since(start)
		latencies = append(latencies, latency)

		if resp.StatusCode == http.StatusOK {
			successCount++
		}

		resp.Body.Close()
		time.Sleep(50 * time.Millisecond)
	}

	// Calculate average latency
	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}
	avgLatency := totalLatency / time.Duration(len(latencies))

	t.Logf("Load balancing test: %d/%d successful", successCount, requestCount)
	t.Logf("Average latency: %v", avgLatency)

	// At least 80% should succeed
	assert.GreaterOrEqual(t, successCount, requestCount*8/10,
		"At least 80% of requests should succeed with load balancing")

	// Average latency should be reasonable
	assert.Less(t, avgLatency, 2*time.Second,
		"Average latency should be under 2 seconds")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		len(s) > len(substr)+1 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
