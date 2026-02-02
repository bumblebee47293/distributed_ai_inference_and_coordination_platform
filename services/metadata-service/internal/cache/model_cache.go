package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yourusername/ai-platform/metadata-service/internal/models"
	"go.uber.org/zap"
)

// ModelCache handles Redis caching for model metadata
type ModelCache struct {
	client *redis.Client
	ttl    time.Duration
	logger *zap.Logger
}

// NewModelCache creates a new model cache
func NewModelCache(client *redis.Client, logger *zap.Logger) *ModelCache {
	return &ModelCache{
		client: client,
		ttl:    15 * time.Minute, // Cache for 15 minutes
		logger: logger,
	}
}

// Get retrieves a model from cache
func (c *ModelCache) Get(ctx context.Context, key string) (*models.ModelMetadata, error) {
	data, err := c.client.Get(ctx, c.modelKey(key)).Bytes()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	var model models.ModelMetadata
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("failed to unmarshal model: %w", err)
	}

	c.logger.Debug("cache hit", zap.String("key", key))

	return &model, nil
}

// Set stores a model in cache
func (c *ModelCache) Set(ctx context.Context, key string, model *models.ModelMetadata) error {
	data, err := json.Marshal(model)
	if err != nil {
		return fmt.Errorf("failed to marshal model: %w", err)
	}

	if err := c.client.Set(ctx, c.modelKey(key), data, c.ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	c.logger.Debug("cache set", zap.String("key", key))

	return nil
}

// Delete removes a model from cache
func (c *ModelCache) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, c.modelKey(key)).Err(); err != nil {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}

	c.logger.Debug("cache deleted", zap.String("key", key))

	return nil
}

// DeleteByPattern deletes all keys matching a pattern
func (c *ModelCache) DeleteByPattern(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, c.modelKey(pattern), 0).Iterator()
	for iter.Next(ctx) {
		if err := c.client.Del(ctx, iter.Val()).Err(); err != nil {
			c.logger.Error("failed to delete key", zap.String("key", iter.Val()), zap.Error(err))
		}
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys: %w", err)
	}

	return nil
}

// modelKey generates a cache key for a model
func (c *ModelCache) modelKey(key string) string {
	return fmt.Sprintf("model:%s", key)
}
