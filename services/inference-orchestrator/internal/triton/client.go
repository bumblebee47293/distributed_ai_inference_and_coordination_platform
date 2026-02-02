package triton

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Client wraps Triton Inference Server HTTP client
type Client struct {
	logger     *zap.Logger
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Triton client
func NewClient(logger *zap.Logger, tritonURL string) *Client {
	return &Client{
		logger:  logger,
		baseURL: "http://" + tritonURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// InferRequest represents a Triton inference request
type InferRequest struct {
	Model   string                 `json:"model"`
	Version string                 `json:"version,omitempty"`
	Inputs  []map[string]interface{} `json:"inputs"`
}

// InferResponse represents a Triton inference response
type InferResponse struct {
	ModelName    string                   `json:"model_name"`
	ModelVersion string                   `json:"model_version"`
	Outputs      []map[string]interface{} `json:"outputs"`
}

// Infer performs inference using Triton
func (c *Client) Infer(ctx context.Context, model, version string, input map[string]interface{}) (map[string]interface{}, error) {
	start := time.Now()

	// For demo purposes, return mock response
	// In production, this would make actual gRPC/HTTP calls to Triton
	c.logger.Info("executing inference",
		zap.String("model", model),
		zap.String("version", version),
	)

	// Simulate inference latency
	time.Sleep(50 * time.Millisecond)

	// Mock response
	result := map[string]interface{}{
		"model_name":    model,
		"model_version": version,
		"prediction": map[string]interface{}{
			"class":       "cat",
			"confidence":  0.95,
			"latency_ms":  time.Since(start).Milliseconds(),
		},
	}

	c.logger.Info("inference completed",
		zap.String("model", model),
		zap.Int64("latency_ms", time.Since(start).Milliseconds()),
	)

	return result, nil
}

// InferHTTP performs inference using Triton HTTP API
func (c *Client) InferHTTP(ctx context.Context, model, version string, input map[string]interface{}) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/v2/models/%s/infer", c.baseURL, model)

	reqBody := map[string]interface{}{
		"inputs": []map[string]interface{}{input},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("triton returned status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

// HealthCheck checks if Triton is healthy
func (c *Client) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v2/health/ready", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("triton not ready: status %d", resp.StatusCode)
	}

	return nil
}
