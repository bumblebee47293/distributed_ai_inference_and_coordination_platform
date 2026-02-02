package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/yourusername/ai-platform/batch-worker/internal/storage"
	"go.uber.org/zap"
)

// InferenceRequest represents a single inference request
type InferenceRequest struct {
	Model   string                 `json:"model"`
	Version string                 `json:"version"`
	Input   map[string]interface{} `json:"input"`
}

// InferenceResult represents the result of an inference
type InferenceResult struct {
	Input      map[string]interface{} `json:"input"`
	Prediction map[string]interface{} `json:"prediction"`
	Latency    int64                  `json:"latency_ms"`
	Error      string                 `json:"error,omitempty"`
}

// PostgresStoreInterface defines the interface for Postgres operations
type PostgresStoreInterface interface {
	CreateJob(ctx context.Context, job *storage.BatchJob) error
	GetJob(ctx context.Context, jobID string) (*storage.BatchJob, error)
	UpdateJobProgress(ctx context.Context, jobID string, completed int, progress float64) error
	UpdateJobStatus(ctx context.Context, jobID string, status storage.JobStatus, resultURL, errorMsg string) error
	Close() error
}

// MinIOStoreInterface defines the interface for MinIO operations
type MinIOStoreInterface interface {
	UploadResults(ctx context.Context, jobID string, results []map[string]interface{}) (string, error)
}

// Pool represents a worker pool for processing batch jobs
type Pool struct {
	size            int
	orchestratorURL string
	pgStore         PostgresStoreInterface
	minioStore      MinIOStoreInterface
	logger          *zap.Logger
	httpClient      *http.Client
}

// NewPool creates a new worker pool
func NewPool(size int, orchestratorURL string, pgStore PostgresStoreInterface, minioStore MinIOStoreInterface, logger *zap.Logger) *Pool {
	return &Pool{
		size:            size,
		orchestratorURL: orchestratorURL,
		pgStore:         pgStore,
		minioStore:      minioStore,
		logger:          logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ProcessJob processes a batch job with worker pool
func (p *Pool) ProcessJob(ctx context.Context, job *storage.BatchJob) error {
	p.logger.Info("processing batch job",
		zap.String("job_id", job.ID),
		zap.Int("total_items", job.TotalItems),
		zap.Int("workers", p.size),
	)

	// Update status to processing
	if err := p.pgStore.UpdateJobStatus(ctx, job.ID, storage.StatusProcessing, "", ""); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Create channels for work distribution
	inputChan := make(chan struct {
		index int
		input map[string]interface{}
	}, len(job.Inputs))
	resultChan := make(chan struct {
		index  int
		result InferenceResult
	}, len(job.Inputs))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < p.size; i++ {
		wg.Add(1)
		go p.worker(ctx, &wg, job, inputChan, resultChan)
	}

	// Send inputs to workers
	go func() {
		for i, input := range job.Inputs {
			select {
			case inputChan <- struct {
				index int
				input map[string]interface{}
			}{index: i, input: input}:
			case <-ctx.Done():
				return
			}
		}
		close(inputChan)
	}()

	// Collect results
	results := make([]map[string]interface{}, len(job.Inputs))
	completed := 0
	errorCount := 0

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results as they come in
	for result := range resultChan {
		completed++
		progress := float64(completed) / float64(job.TotalItems)

		// Store result
		resultData := map[string]interface{}{
			"input":      result.result.Input,
			"prediction": result.result.Prediction,
			"latency_ms": result.result.Latency,
		}

		if result.result.Error != "" {
			resultData["error"] = result.result.Error
			errorCount++
		}

		results[result.index] = resultData

		// Update progress every 10% or on completion
		if completed%max(1, job.TotalItems/10) == 0 || completed == job.TotalItems {
			if err := p.pgStore.UpdateJobProgress(ctx, job.ID, completed, progress); err != nil {
				p.logger.Error("failed to update progress", zap.Error(err))
			}

			p.logger.Info("batch job progress",
				zap.String("job_id", job.ID),
				zap.Int("completed", completed),
				zap.Int("total", job.TotalItems),
				zap.Float64("progress", progress),
			)
		}
	}

	// Upload results to MinIO
	resultURL, err := p.minioStore.UploadResults(ctx, job.ID, results)
	if err != nil {
		p.logger.Error("failed to upload results", zap.Error(err))
		if err := p.pgStore.UpdateJobStatus(ctx, job.ID, storage.StatusFailed, "", err.Error()); err != nil {
			p.logger.Error("failed to update job status", zap.Error(err))
		}
		return fmt.Errorf("failed to upload results: %w", err)
	}

	// Determine final status
	finalStatus := storage.StatusCompleted
	errorMsg := ""
	if errorCount > 0 {
		errorMsg = fmt.Sprintf("%d/%d items failed", errorCount, job.TotalItems)
		if errorCount == job.TotalItems {
			finalStatus = storage.StatusFailed
		}
	}

	// Update final status
	if err := p.pgStore.UpdateJobStatus(ctx, job.ID, finalStatus, resultURL, errorMsg); err != nil {
		return fmt.Errorf("failed to update final status: %w", err)
	}

	p.logger.Info("batch job completed",
		zap.String("job_id", job.ID),
		zap.String("status", string(finalStatus)),
		zap.Int("total", job.TotalItems),
		zap.Int("errors", errorCount),
		zap.String("result_url", resultURL),
	)

	return nil
}

// worker processes individual inference requests
func (p *Pool) worker(
	ctx context.Context,
	wg *sync.WaitGroup,
	job *storage.BatchJob,
	inputChan <-chan struct {
		index int
		input map[string]interface{}
	},
	resultChan chan<- struct {
		index  int
		result InferenceResult
	},
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case work, ok := <-inputChan:
			if !ok {
				return
			}

			// Process inference
			result := p.processInference(ctx, job.Model, job.Version, work.input)

			// Send result
			select {
			case resultChan <- struct {
				index  int
				result InferenceResult
			}{index: work.index, result: result}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// processInference sends an inference request to the orchestrator
func (p *Pool) processInference(ctx context.Context, model, version string, input map[string]interface{}) InferenceResult {
	start := time.Now()

	req := InferenceRequest{
		Model:   model,
		Version: version,
		Input:   input,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return InferenceResult{
			Input: input,
			Error: fmt.Sprintf("failed to marshal request: %v", err),
		}
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.orchestratorURL+"/v1/infer", bytes.NewBuffer(reqBody))
	if err != nil {
		return InferenceResult{
			Input: input,
			Error: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return InferenceResult{
			Input:   input,
			Error:   fmt.Sprintf("request failed: %v", err),
			Latency: time.Since(start).Milliseconds(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return InferenceResult{
			Input:   input,
			Error:   fmt.Sprintf("inference failed with status %d", resp.StatusCode),
			Latency: time.Since(start).Milliseconds(),
		}
	}

	var prediction map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&prediction); err != nil {
		return InferenceResult{
			Input:   input,
			Error:   fmt.Sprintf("failed to decode response: %v", err),
			Latency: time.Since(start).Milliseconds(),
		}
	}

	return InferenceResult{
		Input:      input,
		Prediction: prediction,
		Latency:    time.Since(start).Milliseconds(),
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
