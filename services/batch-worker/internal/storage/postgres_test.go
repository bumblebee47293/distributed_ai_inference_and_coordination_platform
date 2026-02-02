package storage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestBatchJob_Creation(t *testing.T) {
	job := &BatchJob{
		ID:         "test-job-1",
		Model:      "resnet18",
		Version:    "v1",
		Inputs:     []map[string]interface{}{{"data": []float64{1.0, 2.0}}},
		Status:     StatusPending,
		TotalItems: 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	assert.Equal(t, "test-job-1", job.ID)
	assert.Equal(t, StatusPending, job.Status)
	assert.Equal(t, 1, job.TotalItems)
}

func TestJobStatus_Values(t *testing.T) {
	assert.Equal(t, JobStatus("pending"), StatusPending)
	assert.Equal(t, JobStatus("processing"), StatusProcessing)
	assert.Equal(t, JobStatus("completed"), StatusCompleted)
	assert.Equal(t, JobStatus("failed"), StatusFailed)
}

// Integration test - requires PostgreSQL
func TestPostgresStore_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	logger, _ := zap.NewDevelopment()
	connectionURL := "postgres://postgres:postgres@localhost:5432/ai_platform_test?sslmode=disable"

	store, err := NewPostgresStore(connectionURL, logger)
	if err != nil {
		t.Skip("PostgreSQL not available:", err)
		return
	}
	defer store.Close()

	ctx := context.Background()

	// Test create job
	job := &BatchJob{
		ID:         "test-job-integration",
		Model:      "resnet18",
		Version:    "v1",
		Inputs:     []map[string]interface{}{{"data": []float64{1.0, 2.0}}},
		Status:     StatusPending,
		TotalItems: 1,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	err = store.CreateJob(ctx, job)
	assert.NoError(t, err)

	// Test get job
	retrieved, err := store.GetJob(ctx, job.ID)
	assert.NoError(t, err)
	assert.Equal(t, job.ID, retrieved.ID)
	assert.Equal(t, job.Model, retrieved.Model)

	// Test update progress
	err = store.UpdateJobProgress(ctx, job.ID, 1, 1.0)
	assert.NoError(t, err)

	// Test update status
	err = store.UpdateJobStatus(ctx, job.ID, StatusCompleted, "http://results.com/job1", "")
	assert.NoError(t, err)

	// Verify final state
	final, err := store.GetJob(ctx, job.ID)
	assert.NoError(t, err)
	assert.Equal(t, StatusCompleted, final.Status)
	assert.Equal(t, "http://results.com/job1", final.ResultURL)
}
