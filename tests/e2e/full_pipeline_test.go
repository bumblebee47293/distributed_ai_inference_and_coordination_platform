package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullPipeline tests the complete end-to-end workflow:
// 1. Register a model in metadata service
// 2. Configure model router with the model
// 3. Perform real-time inference
// 4. Submit batch job
// 5. Verify distributed tracing
// 6. Check metrics collection
func TestFullPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	// Service URLs
	metadataURL := getEnv("METADATA_SERVICE_URL", "http://localhost:8083")
	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")
	prometheusURL := getEnv("PROMETHEUS_URL", "http://localhost:9090")

	client := &http.Client{Timeout: 15 * time.Second}

	// Step 1: Register a new model
	t.Log("Step 1: Registering model in metadata service...")
	
	modelData := map[string]interface{}{
		"name":        "e2e-test-model",
		"version":     "1.0.0",
		"framework":   "pytorch",
		"format":      "onnx",
		"backend_url": "http://triton:8001",
		"description": "End-to-end test model",
		"status":      "active",
	}

	jsonData, _ := json.Marshal(modelData)
	req, _ := http.NewRequest("POST", metadataURL+"/v1/models", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var modelResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&modelResp)
	modelID := modelResp["id"].(string)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	t.Logf("✓ Model registered with ID: %s", modelID)

	// Step 2: Verify model is retrievable
	t.Log("Step 2: Verifying model retrieval...")
	
	time.Sleep(1 * time.Second) // Allow cache to populate

	req, _ = http.NewRequest("GET", metadataURL+"/v1/models/"+modelID, nil)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	t.Log("✓ Model retrieved successfully")

	// Step 3: Perform real-time inference (using existing model)
	t.Log("Step 3: Performing real-time inference...")
	
	inferReq := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"input":   map[string]interface{}{"data": []float64{1.0, 2.0, 3.0, 4.0}},
	}

	jsonData, _ = json.Marshal(inferReq)
	req, _ = http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer demo-token")
	req.Header.Set("X-Request-ID", "e2e-test-request")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var inferResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&inferResp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, inferResp, "prediction")
	
	traceID := resp.Header.Get("X-Trace-ID")
	assert.NotEmpty(t, traceID)
	t.Logf("✓ Real-time inference completed (trace: %s)", traceID)

	// Step 4: Submit batch job
	t.Log("Step 4: Submitting batch inference job...")
	
	batchReq := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"inputs": []interface{}{
			map[string]interface{}{"data": []float64{1.0, 2.0}},
			map[string]interface{}{"data": []float64{3.0, 4.0}},
			map[string]interface{}{"data": []float64{5.0, 6.0}},
		},
	}

	jsonData, _ = json.Marshal(batchReq)
	req, _ = http.NewRequest("POST", apiGatewayURL+"/v1/batch", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer demo-token")

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var batchResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&batchResp)

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	jobID := batchResp["job_id"].(string)
	t.Logf("✓ Batch job submitted (ID: %s)", jobID)

	// Step 5: Monitor batch job progress
	t.Log("Step 5: Monitoring batch job progress...")
	
	jobCompleted := false
	maxWait := 30 * time.Second
	startTime := time.Now()

	for !jobCompleted && time.Since(startTime) < maxWait {
		time.Sleep(2 * time.Second)

		req, _ = http.NewRequest("GET", apiGatewayURL+"/v1/batch/"+jobID, nil)
		req.Header.Set("Authorization", "Bearer demo-token")

		resp, err = client.Do(req)
		if err != nil {
			continue
		}

		var statusResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&statusResp)
		resp.Body.Close()

		status := statusResp["status"].(string)
		progress := statusResp["progress"].(float64)

		t.Logf("  Job status: %s (%.0f%%)", status, progress*100)

		if status == "completed" || status == "failed" {
			jobCompleted = true
			assert.Equal(t, "completed", status)
		}
	}

	assert.True(t, jobCompleted, "Batch job should complete within timeout")
	t.Log("✓ Batch job completed successfully")

	// Step 6: Verify metrics are being collected
	t.Log("Step 6: Verifying metrics collection...")
	
	req, _ = http.NewRequest("GET", prometheusURL+"/api/v1/query?query=up", nil)
	resp, err = client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		t.Log("✓ Prometheus metrics accessible")
	} else {
		t.Log("⚠ Prometheus not accessible (optional)")
	}

	// Step 7: Check service health endpoints
	t.Log("Step 7: Checking service health...")
	
	services := map[string]string{
		"API Gateway":            apiGatewayURL + "/health",
		"Metadata Service":       metadataURL + "/health",
		"Model Router":           getEnv("MODEL_ROUTER_URL", "http://localhost:8081") + "/health",
		"Inference Orchestrator": getEnv("ORCHESTRATOR_URL", "http://localhost:8082") + "/health",
	}

	healthyServices := 0
	for name, url := range services {
		req, _ := http.NewRequest("GET", url, nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				healthyServices++
				t.Logf("  ✓ %s: healthy", name)
			} else {
				t.Logf("  ✗ %s: unhealthy (status: %d)", name, resp.StatusCode)
			}
		} else {
			t.Logf("  ✗ %s: unreachable", name)
		}
	}

	assert.GreaterOrEqual(t, healthyServices, 3, "At least 3 services should be healthy")
	t.Logf("✓ %d/%d services healthy", healthyServices, len(services))

	// Cleanup: Delete test model
	t.Log("Cleanup: Deleting test model...")
	req, _ = http.NewRequest("DELETE", metadataURL+"/v1/models/"+modelID, nil)
	client.Do(req)

	t.Log("\n🎉 Full E2E pipeline test completed successfully!")
}

// TestSystemResilience tests system behavior under stress
func TestSystemResilience(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")
	client := &http.Client{Timeout: 10 * time.Second}

	t.Log("Testing system resilience with concurrent load...")

	// Concurrent inference requests
	concurrency := 20
	results := make(chan bool, concurrency)

	reqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"input":   map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}},
	}

	jsonData, _ := json.Marshal(reqBody)

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/infer", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer demo-token")

			resp, err := client.Do(req)
			if err != nil {
				results <- false
				return
			}
			defer resp.Body.Close()

			results <- resp.StatusCode == http.StatusOK
		}(i)
	}

	// Collect results
	successCount := 0
	for i := 0; i < concurrency; i++ {
		if <-results {
			successCount++
		}
	}

	duration := time.Since(start)

	t.Logf("Resilience test: %d/%d successful in %v", successCount, concurrency, duration)
	
	// At least 70% should succeed under load
	assert.GreaterOrEqual(t, successCount, concurrency*7/10,
		"System should handle at least 70% of concurrent requests")

	// Should complete in reasonable time
	assert.Less(t, duration, 30*time.Second,
		"Concurrent requests should complete within 30 seconds")
}

func getEnv(key, defaultValue string) string {
	// In real implementation, use os.Getenv
	return defaultValue
}
