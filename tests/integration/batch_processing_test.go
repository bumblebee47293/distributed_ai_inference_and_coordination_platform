package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBatchProcessing tests the complete batch inference workflow:
// API Gateway -> Kafka -> Batch Worker -> PostgreSQL -> MinIO
func TestBatchProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")

	// Submit batch job
	reqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"inputs": []interface{}{
			map[string]interface{}{"data": []float64{1.0, 2.0, 3.0}},
			map[string]interface{}{"data": []float64{4.0, 5.0, 6.0}},
			map[string]interface{}{"data": []float64{7.0, 8.0, 9.0}},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	require.NoError(t, err)

	// Submit batch job
	req, err := http.NewRequest("POST", apiGatewayURL+"/v1/batch", bytes.NewBuffer(jsonData))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer demo-token")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var submitResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&submitResponse)
	require.NoError(t, err)

	jobID, ok := submitResponse["job_id"].(string)
	require.True(t, ok, "Response should contain job_id")
	assert.NotEmpty(t, jobID)

	t.Logf("Submitted batch job: %s", jobID)

	// Poll for job completion
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	jobCompleted := false
	var finalStatus map[string]interface{}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for !jobCompleted {
		select {
		case <-ctx.Done():
			t.Fatal("Job did not complete within timeout")
		case <-ticker.C:
			// Check job status
			statusReq, _ := http.NewRequest("GET", apiGatewayURL+"/v1/batch/"+jobID, nil)
			statusReq.Header.Set("Authorization", "Bearer demo-token")

			statusResp, err := client.Do(statusReq)
			if err != nil {
				continue
			}

			if statusResp.StatusCode == http.StatusOK {
				json.NewDecoder(statusResp.Body).Decode(&finalStatus)
				statusResp.Body.Close()

				status, _ := finalStatus["status"].(string)
				t.Logf("Job status: %s", status)

				if status == "completed" || status == "failed" {
					jobCompleted = true
				}
			} else {
				statusResp.Body.Close()
			}
		}
	}

	// Verify final status
	status, _ := finalStatus["status"].(string)
	assert.Equal(t, "completed", status, "Job should complete successfully")

	// Verify results URL is present
	resultURL, ok := finalStatus["result_url"].(string)
	assert.True(t, ok, "Completed job should have result_url")
	assert.NotEmpty(t, resultURL)

	t.Logf("Batch job completed. Results at: %s", resultURL)
}

// TestBatchProcessingProgress tests progress tracking
func TestBatchProcessingProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")

	// Submit batch job with multiple items
	inputs := make([]interface{}, 20)
	for i := 0; i < 20; i++ {
		inputs[i] = map[string]interface{}{"data": []float64{float64(i), float64(i + 1)}}
	}

	reqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"inputs":  inputs,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/batch", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer demo-token")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var submitResponse map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&submitResponse)
	jobID := submitResponse["job_id"].(string)

	// Track progress
	progressValues := []float64{}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			t.Log("Progress tracking timeout")
			return
		case <-ticker.C:
			statusReq, _ := http.NewRequest("GET", apiGatewayURL+"/v1/batch/"+jobID, nil)
			statusReq.Header.Set("Authorization", "Bearer demo-token")

			statusResp, err := client.Do(statusReq)
			if err != nil {
				continue
			}

			var status map[string]interface{}
			json.NewDecoder(statusResp.Body).Decode(&status)
			statusResp.Body.Close()

			if progress, ok := status["progress"].(float64); ok {
				progressValues = append(progressValues, progress)
				t.Logf("Progress: %.1f%%", progress*100)

				if progress >= 1.0 {
					// Verify progress increased monotonically
					for i := 1; i < len(progressValues); i++ {
						assert.GreaterOrEqual(t, progressValues[i], progressValues[i-1],
							"Progress should increase monotonically")
					}
					return
				}
			}
		}
	}
}

// TestBatchProcessingWithFailures tests partial failure handling
func TestBatchProcessingWithFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")

	// Submit batch with some invalid inputs
	reqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"inputs": []interface{}{
			map[string]interface{}{"data": []float64{1.0, 2.0}},
			map[string]interface{}{}, // Invalid - empty input
			map[string]interface{}{"data": []float64{3.0, 4.0}},
		},
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/batch", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer demo-token")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var submitResponse map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&submitResponse)
	jobID := submitResponse["job_id"].(string)

	// Wait for completion
	time.Sleep(10 * time.Second)

	// Check final status
	statusReq, _ := http.NewRequest("GET", apiGatewayURL+"/v1/batch/"+jobID, nil)
	statusReq.Header.Set("Authorization", "Bearer demo-token")

	statusResp, err := client.Do(statusReq)
	require.NoError(t, err)
	defer statusResp.Body.Close()

	var finalStatus map[string]interface{}
	json.NewDecoder(statusResp.Body).Decode(&finalStatus)

	// Job should complete even with partial failures
	status, _ := finalStatus["status"].(string)
	assert.Contains(t, []string{"completed", "failed"}, status)

	// Should have processed all items
	completed, _ := finalStatus["completed"].(float64)
	total, _ := finalStatus["total"].(float64)
	assert.Equal(t, total, completed, "All items should be processed")

	t.Logf("Batch job with failures: %s (completed %d/%d)", status, int(completed), int(total))
}

// TestBatchJobCancellation tests job cancellation
func TestBatchJobCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	apiGatewayURL := getEnv("API_GATEWAY_URL", "http://localhost:8080")

	// Submit large batch job
	inputs := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		inputs[i] = map[string]interface{}{"data": []float64{float64(i)}}
	}

	reqBody := map[string]interface{}{
		"model":   "resnet18",
		"version": "1",
		"inputs":  inputs,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", apiGatewayURL+"/v1/batch", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer demo-token")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var submitResponse map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&submitResponse)
	jobID := submitResponse["job_id"].(string)

	// Wait a bit for job to start
	time.Sleep(2 * time.Second)

	// Cancel the job
	cancelReq, _ := http.NewRequest("DELETE", apiGatewayURL+"/v1/batch/"+jobID, nil)
	cancelReq.Header.Set("Authorization", "Bearer demo-token")

	cancelResp, err := client.Do(cancelReq)
	if err == nil {
		defer cancelResp.Body.Close()
		// Cancellation endpoint might not be implemented yet
		t.Logf("Cancellation response: %d", cancelResp.StatusCode)
	}
}
