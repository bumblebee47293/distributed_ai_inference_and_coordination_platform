package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/sony/gobreaker"
	"go.uber.org/zap"
)

// Backend represents a model serving backend
type Backend struct {
	URL            string
	CircuitBreaker *gobreaker.CircuitBreaker
	HealthStatus   bool
	LastCheck      time.Time
	AvgLatency     time.Duration
	mu             sync.RWMutex
}

// ModelRouter handles intelligent routing of inference requests
type ModelRouter struct {
	logger   *zap.Logger
	backends map[string]map[string][]*Backend // model -> version -> backends
	mu       sync.RWMutex
	client   *http.Client
}

// NewModelRouter creates a new model router
func NewModelRouter(logger *zap.Logger, defaultURL string) *ModelRouter {
	return &ModelRouter{
		logger:   logger,
		backends: make(map[string]map[string][]*Backend),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RegisterBackend registers a new backend for a model version
func (r *ModelRouter) RegisterBackend(model, version, url string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.backends[model] == nil {
		r.backends[model] = make(map[string][]*Backend)
	}

	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        fmt.Sprintf("%s-%s", model, version),
		MaxRequests: 3,
		Interval:    10 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
	})

	backend := &Backend{
		URL:            url,
		CircuitBreaker: cb,
		HealthStatus:   true,
		LastCheck:      time.Now(),
	}

	r.backends[model][version] = append(r.backends[model][version], backend)
	r.logger.Info("registered backend",
		zap.String("model", model),
		zap.String("version", version),
		zap.String("url", url),
	)
}

// RouteRequest routes an inference request to the appropriate backend
func (r *ModelRouter) RouteRequest(ctx context.Context, model, version string, input map[string]interface{}) (map[string]interface{}, error) {
	r.mu.RLock()
	versions, ok := r.backends[model]
	if !ok {
		r.mu.RUnlock()
		return nil, fmt.Errorf("model not found: %s", model)
	}

	backends, ok := versions[version]
	if !ok || len(backends) == 0 {
		r.mu.RUnlock()
		return nil, fmt.Errorf("version not found: %s/%s", model, version)
	}
	r.mu.RUnlock()

	// Select backend using round-robin (could be enhanced with latency-based routing)
	backend := r.selectBackend(backends)

	// Execute request through circuit breaker
	result, err := backend.CircuitBreaker.Execute(func() (interface{}, error) {
		return r.executeRequest(ctx, backend, model, version, input)
	})

	if err != nil {
		return nil, err
	}

	return result.(map[string]interface{}), nil
}

// selectBackend selects a backend using round-robin strategy
func (r *ModelRouter) selectBackend(backends []*Backend) *Backend {
	// Simple random selection (in production, use weighted round-robin based on latency)
	return backends[rand.Intn(len(backends))]
}

// executeRequest executes the actual HTTP request to the backend
func (r *ModelRouter) executeRequest(ctx context.Context, backend *Backend, model, version string, input map[string]interface{}) (map[string]interface{}, error) {
	start := time.Now()

	reqBody := map[string]interface{}{
		"model":   model,
		"version": version,
		"input":   input,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", backend.URL+"/v1/infer", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		backend.mu.Lock()
		backend.HealthStatus = false
		backend.mu.Unlock()
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Update backend metrics
	latency := time.Since(start)
	backend.mu.Lock()
	backend.HealthStatus = true
	backend.AvgLatency = latency
	backend.LastCheck = time.Now()
	backend.mu.Unlock()

	return result, nil
}
