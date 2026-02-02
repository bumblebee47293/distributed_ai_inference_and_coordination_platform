package triton

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNewTritonClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewTritonClient("localhost:8001", logger)

	assert.NotNil(t, client)
	assert.Equal(t, "localhost:8001", client.serverURL)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
}

func TestTritonClient_ServerURL(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	tests := []struct {
		name      string
		serverURL string
		expected  string
	}{
		{
			name:      "localhost",
			serverURL: "localhost:8001",
			expected:  "localhost:8001",
		},
		{
			name:      "with http prefix",
			serverURL: "http://triton:8001",
			expected:  "http://triton:8001",
		},
		{
			name:      "IP address",
			serverURL: "192.168.1.100:8001",
			expected:  "192.168.1.100:8001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewTritonClient(tt.serverURL, logger)
			assert.Equal(t, tt.expected, client.serverURL)
		})
	}
}

func TestTritonClient_HealthCheck_Timeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	// Use invalid URL to trigger timeout
	client := NewTritonClient("http://invalid-host:8001", logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.HealthCheck(ctx)
	assert.Error(t, err)
}

func TestTritonClient_Infer_InvalidInput(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewTritonClient("http://localhost:8001", logger)

	ctx := context.Background()

	// Test with nil input
	_, err := client.Infer(ctx, "resnet18", "1", nil)
	assert.Error(t, err)

	// Test with empty model name
	_, err = client.Infer(ctx, "", "1", map[string]interface{}{"data": []float64{1.0}})
	assert.Error(t, err)
}

func TestTritonClient_BuildInferenceURL(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewTritonClient("http://localhost:8001", logger)

	tests := []struct {
		name      string
		modelName string
		version   string
		expected  string
	}{
		{
			name:      "basic model",
			modelName: "resnet18",
			version:   "1",
			expected:  "http://localhost:8001/v2/models/resnet18/versions/1/infer",
		},
		{
			name:      "different version",
			modelName: "bert",
			version:   "2",
			expected:  "http://localhost:8001/v2/models/bert/versions/2/infer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := client.buildInferenceURL(tt.modelName, tt.version)
			assert.Equal(t, tt.expected, url)
		})
	}
}

func TestTritonClient_ContextCancellation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := NewTritonClient("http://localhost:8001", logger)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.HealthCheck(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}
