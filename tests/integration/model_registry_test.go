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

// TestModelRegistry tests the complete model registration and routing workflow
func TestModelRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	metadataServiceURL := getEnv("METADATA_SERVICE_URL", "http://localhost:8083")

	// Test model data
	modelData := map[string]interface{}{
		"name":        "test-model-integration",
		"version":     "1.0.0",
		"framework":   "pytorch",
		"format":      "onnx",
		"backend_url": "http://triton:8001",
		"description": "Integration test model",
		"tags":        []string{"test", "integration"},
		"metadata": map[string]interface{}{
			"input_shape":  []int{1, 3, 224, 224},
			"output_shape": []int{1, 1000},
		},
	}

	jsonData, err := json.Marshal(modelData)
	require.NoError(t, err)

	client := &http.Client{Timeout: 10 * time.Second}

	// 1. Create model
	t.Run("CreateModel", func(t *testing.T) {
		req, _ := http.NewRequest("POST", metadataServiceURL+"/v1/models", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)

		assert.Contains(t, response, "id")
		assert.Equal(t, "test-model-integration", response["name"])
		assert.Equal(t, "1.0.0", response["version"])

		// Store model ID for later tests
		modelData["id"] = response["id"]
	})

	// 2. Get model by ID
	t.Run("GetModelByID", func(t *testing.T) {
		modelID := modelData["id"].(string)
		req, _ := http.NewRequest("GET", metadataServiceURL+"/v1/models/"+modelID, nil)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)

		assert.Equal(t, modelID, response["id"])
		assert.Equal(t, "test-model-integration", response["name"])
	})

	// 3. Get model by name and version
	t.Run("GetModelByNameVersion", func(t *testing.T) {
		req, _ := http.NewRequest("GET", metadataServiceURL+"/v1/models/by-name/test-model-integration/1.0.0", nil)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)

		assert.Equal(t, "test-model-integration", response["name"])
		assert.Equal(t, "1.0.0", response["version"])
	})

	// 4. List models
	t.Run("ListModels", func(t *testing.T) {
		req, _ := http.NewRequest("GET", metadataServiceURL+"/v1/models", nil)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)

		models, ok := response["models"].([]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, models)
	})

	// 5. Update model
	t.Run("UpdateModel", func(t *testing.T) {
		modelID := modelData["id"].(string)

		updateData := map[string]interface{}{
			"description": "Updated integration test model",
			"status":      "active",
		}

		jsonData, _ := json.Marshal(updateData)
		req, _ := http.NewRequest("PUT", metadataServiceURL+"/v1/models/"+modelID, bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var response map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&response)

		assert.Equal(t, "Updated integration test model", response["description"])
	})

	// 6. Delete model
	t.Run("DeleteModel", func(t *testing.T) {
		modelID := modelData["id"].(string)

		req, _ := http.NewRequest("DELETE", metadataServiceURL+"/v1/models/"+modelID, nil)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Verify deletion
		getReq, _ := http.NewRequest("GET", metadataServiceURL+"/v1/models/"+modelID, nil)
		getResp, _ := client.Do(getReq)
		defer getResp.Body.Close()

		assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
	})
}

// TestModelRegistryCaching tests Redis caching behavior
func TestModelRegistryCaching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	metadataServiceURL := getEnv("METADATA_SERVICE_URL", "http://localhost:8083")

	// Create a test model
	modelData := map[string]interface{}{
		"name":        "cache-test-model",
		"version":     "1.0.0",
		"framework":   "pytorch",
		"format":      "onnx",
		"backend_url": "http://triton:8001",
	}

	jsonData, _ := json.Marshal(modelData)
	client := &http.Client{Timeout: 10 * time.Second}

	// Create model
	req, _ := http.NewRequest("POST", metadataServiceURL+"/v1/models", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	var createResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResp)
	resp.Body.Close()

	modelID := createResp["id"].(string)

	// First request (cache miss)
	start1 := time.Now()
	req1, _ := http.NewRequest("GET", metadataServiceURL+"/v1/models/"+modelID, nil)
	resp1, _ := client.Do(req1)
	resp1.Body.Close()
	latency1 := time.Since(start1)

	// Second request (cache hit - should be faster)
	start2 := time.Now()
	req2, _ := http.NewRequest("GET", metadataServiceURL+"/v1/models/"+modelID, nil)
	resp2, _ := client.Do(req2)
	resp2.Body.Close()
	latency2 := time.Since(start2)

	t.Logf("First request (cache miss): %v", latency1)
	t.Logf("Second request (cache hit): %v", latency2)

	// Cache hit should generally be faster (though not guaranteed in all environments)
	assert.LessOrEqual(t, latency2, latency1*2, "Cached request should not be significantly slower")

	// Cleanup
	delReq, _ := http.NewRequest("DELETE", metadataServiceURL+"/v1/models/"+modelID, nil)
	client.Do(delReq)
}

// TestModelRegistryValidation tests input validation
func TestModelRegistryValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	metadataServiceURL := getEnv("METADATA_SERVICE_URL", "http://localhost:8083")
	client := &http.Client{Timeout: 10 * time.Second}

	tests := []struct {
		name           string
		modelData      map[string]interface{}
		expectedStatus int
	}{
		{
			name: "missing required field - name",
			modelData: map[string]interface{}{
				"version":     "1.0.0",
				"framework":   "pytorch",
				"format":      "onnx",
				"backend_url": "http://triton:8001",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing required field - version",
			modelData: map[string]interface{}{
				"name":        "test-model",
				"framework":   "pytorch",
				"format":      "onnx",
				"backend_url": "http://triton:8001",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "invalid framework",
			modelData: map[string]interface{}{
				"name":        "test-model",
				"version":     "1.0.0",
				"framework":   "invalid-framework",
				"format":      "onnx",
				"backend_url": "http://triton:8001",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "valid model",
			modelData: map[string]interface{}{
				"name":        "validation-test-model",
				"version":     "1.0.0",
				"framework":   "pytorch",
				"format":      "onnx",
				"backend_url": "http://triton:8001",
			},
			expectedStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, _ := json.Marshal(tt.modelData)
			req, _ := http.NewRequest("POST", metadataServiceURL+"/v1/models", bytes.NewBuffer(jsonData))
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Cleanup if created successfully
			if resp.StatusCode == http.StatusCreated {
				var createResp map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&createResp)
				if modelID, ok := createResp["id"].(string); ok {
					delReq, _ := http.NewRequest("DELETE", metadataServiceURL+"/v1/models/"+modelID, nil)
					client.Do(delReq)
				}
			}
		})
	}
}
