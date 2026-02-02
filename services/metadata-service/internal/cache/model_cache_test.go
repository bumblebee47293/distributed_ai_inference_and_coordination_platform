package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/yourusername/ai-platform/metadata-service/internal/models"
	"go.uber.org/zap"
)

func TestModelCache_SetAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	logger, _ := zap.NewDevelopment()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Test connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available:", err)
		return
	}

	cache := NewModelCache(client, logger)
	ctx := context.Background()

	model := &models.ModelMetadata{
		ID:          "test-model-1",
		Name:        "resnet18",
		Version:     "v1",
		Framework:   "pytorch",
		Format:      "onnx",
		Description: "Test model",
		Status:      "active",
		BackendURL:  "http://localhost:8082",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Test Set
	err := cache.Set(ctx, model.ID, model)
	assert.NoError(t, err)

	// Test Get
	retrieved, err := cache.Get(ctx, model.ID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, model.ID, retrieved.ID)
	assert.Equal(t, model.Name, retrieved.Name)

	// Test Delete
	err = cache.Delete(ctx, model.ID)
	assert.NoError(t, err)

	// Verify deleted
	retrieved, err = cache.Get(ctx, model.ID)
	assert.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestModelCache_CacheMiss(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	logger, _ := zap.NewDevelopment()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skip("Redis not available:", err)
		return
	}

	cache := NewModelCache(client, logger)
	ctx := context.Background()

	// Get non-existent key
	retrieved, err := cache.Get(ctx, "nonexistent-key")
	assert.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestModelCache_KeyGeneration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	cache := NewModelCache(client, logger)

	key := cache.modelKey("test-id")
	assert.Equal(t, "model:test-id", key)
}
