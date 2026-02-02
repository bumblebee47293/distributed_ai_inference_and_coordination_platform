package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/yourusername/ai-platform/batch-worker/internal/storage"
	"go.uber.org/zap"
)

// MockPostgresStore is a mock implementation of PostgresStore
type MockPostgresStore struct {
	jobs map[string]*storage.BatchJob
}

func NewMockPostgresStore() *MockPostgresStore {
	return &MockPostgresStore{
		jobs: make(map[string]*storage.BatchJob),
	}
}

func (m *MockPostgresStore) CreateJob(ctx context.Context, job *storage.BatchJob) error {
	m.jobs[job.ID] = job
	return nil
}

func (m *MockPostgresStore) GetJob(ctx context.Context, jobID string) (*storage.BatchJob, error) {
	if job, ok := m.jobs[jobID]; ok {
		return job, nil
	}
	return nil, nil
}

func (m *MockPostgresStore) UpdateJobProgress(ctx context.Context, jobID string, completed int, progress float64) error {
	if job, ok := m.jobs[jobID]; ok {
		job.Completed = completed
		job.Progress = progress
	}
	return nil
}

func (m *MockPostgresStore) UpdateJobStatus(ctx context.Context, jobID string, status storage.JobStatus, resultURL, errorMsg string) error {
	if job, ok := m.jobs[jobID]; ok {
		job.Status = status
		job.ResultURL = resultURL
		job.ErrorMsg = errorMsg
	}
	return nil
}

func (m *MockPostgresStore) Close() error {
	return nil
}

// MockMinIOStore is a mock implementation of MinIOStore
type MockMinIOStore struct {
	uploadedResults map[string][]map[string]interface{}
}

func NewMockMinIOStore() *MockMinIOStore {
	return &MockMinIOStore{
		uploadedResults: make(map[string][]map[string]interface{}),
	}
}

func (m *MockMinIOStore) UploadResults(ctx context.Context, jobID string, results []map[string]interface{}) (string, error) {
	m.uploadedResults[jobID] = results
	return "http://minio/results/" + jobID + ".json", nil
}

func TestNewPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pgStore := NewMockPostgresStore()
	minioStore := NewMockMinIOStore()

	pool := NewPool(5, "http://localhost:8082", pgStore, minioStore, logger)

	assert.NotNil(t, pool)
	assert.Equal(t, 5, pool.size)
	assert.Equal(t, "http://localhost:8082", pool.orchestratorURL)
}

func TestPool_ProcessJob_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pgStore := NewMockPostgresStore()
	minioStore := NewMockMinIOStore()

	// Create mock orchestrator server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"prediction": [0.1, 0.9]}`))
	}))
	defer server.Close()

	pool := NewPool(2, server.URL, pgStore, minioStore, logger)

	job := &storage.BatchJob{
		ID:      "test-job-1",
		Model:   "resnet18",
		Version: "v1",
		Inputs: []map[string]interface{}{
			{"data": []float64{1.0, 2.0, 3.0}},
			{"data": []float64{4.0, 5.0, 6.0}},
		},
		Status:     storage.StatusPending,
		TotalItems: 2,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()
	err := pool.ProcessJob(ctx, job)

	assert.NoError(t, err)
	assert.NotEmpty(t, minioStore.uploadedResults["test-job-1"])
	assert.Equal(t, 2, len(minioStore.uploadedResults["test-job-1"]))
}

func TestPool_ProcessJob_PartialFailure(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pgStore := NewMockPostgresStore()
	minioStore := NewMockMinIOStore()

	// Create mock server that fails every other request
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount%2 == 0 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"prediction": [0.1, 0.9]}`))
		}
	}))
	defer server.Close()

	pool := NewPool(2, server.URL, pgStore, minioStore, logger)

	job := &storage.BatchJob{
		ID:      "test-job-2",
		Model:   "resnet18",
		Version: "v1",
		Inputs: []map[string]interface{}{
			{"data": []float64{1.0}},
			{"data": []float64{2.0}},
			{"data": []float64{3.0}},
			{"data": []float64{4.0}},
		},
		Status:     storage.StatusPending,
		TotalItems: 4,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	ctx := context.Background()
	err := pool.ProcessJob(ctx, job)

	assert.NoError(t, err)
	// Should have results for all items (some with errors)
	assert.Equal(t, 4, len(minioStore.uploadedResults["test-job-2"]))
}

func TestPool_ProcessInference_Success(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pgStore := NewMockPostgresStore()
	minioStore := NewMockMinIOStore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/infer", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"prediction": [0.5, 0.5]}`))
	}))
	defer server.Close()

	pool := NewPool(1, server.URL, pgStore, minioStore, logger)

	result := pool.processInference(context.Background(), "resnet18", "v1", map[string]interface{}{"data": []float64{1.0}})

	assert.Empty(t, result.Error)
	assert.NotNil(t, result.Prediction)
	assert.GreaterOrEqual(t, result.Latency, int64(0))
}

func TestPool_ProcessInference_Timeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pgStore := NewMockPostgresStore()
	minioStore := NewMockMinIOStore()

	// Server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"prediction": [0.5, 0.5]}`))
	}))
	defer server.Close()

	pool := NewPool(1, server.URL, pgStore, minioStore, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result := pool.processInference(ctx, "resnet18", "v1", map[string]interface{}{"data": []float64{1.0}})

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "request failed")
}

func TestPool_ProcessInference_InvalidResponse(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pgStore := NewMockPostgresStore()
	minioStore := NewMockMinIOStore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	pool := NewPool(1, server.URL, pgStore, minioStore, logger)

	result := pool.processInference(context.Background(), "resnet18", "v1", map[string]interface{}{"data": []float64{1.0}})

	assert.NotEmpty(t, result.Error)
	assert.Contains(t, result.Error, "failed to decode response")
}
