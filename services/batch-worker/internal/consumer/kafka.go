package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"github.com/yourusername/ai-platform/batch-worker/internal/storage"
	"github.com/yourusername/ai-platform/batch-worker/internal/worker"
	"go.uber.org/zap"
)

// PostgresStoreInterface defines the interface for Postgres operations
type PostgresStoreInterface interface {
	CreateJob(ctx context.Context, job *storage.BatchJob) error
	GetJob(ctx context.Context, jobID string) (*storage.BatchJob, error)
	UpdateJobProgress(ctx context.Context, jobID string, completed int, progress float64) error
	UpdateJobStatus(ctx context.Context, jobID string, status storage.JobStatus, resultURL, errorMsg string) error
	Close() error
}

// KafkaConsumer handles consuming batch jobs from Kafka
type KafkaConsumer struct {
	consumer sarama.ConsumerGroup
	topic    string
	pool     *worker.Pool
	pgStore  PostgresStoreInterface
	logger   *zap.Logger
}

// NewKafkaConsumer creates a new Kafka consumer
func NewKafkaConsumer(
	brokers []string,
	topic string,
	groupID string,
	pool *worker.Pool,
	pgStore PostgresStoreInterface,
	logger *zap.Logger,
) (*KafkaConsumer, error) {
	config := sarama.NewConfig()
	config.Version = sarama.V3_3_0_0
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetNewest
	config.Consumer.Return.Errors = true

	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	return &KafkaConsumer{
		consumer: consumer,
		topic:    topic,
		pool:     pool,
		pgStore:  pgStore,
		logger:   logger,
	}, nil
}

// Start starts consuming messages
func (c *KafkaConsumer) Start(ctx context.Context) error {
	handler := &consumerGroupHandler{
		pool:    c.pool,
		pgStore: c.pgStore,
		logger:  c.logger,
	}

	c.logger.Info("starting kafka consumer",
		zap.String("topic", c.topic),
	)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("shutting down kafka consumer")
			return c.consumer.Close()
		default:
			if err := c.consumer.Consume(ctx, []string{c.topic}, handler); err != nil {
				c.logger.Error("consumer error", zap.Error(err))
				return err
			}
		}
	}
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	pool    *worker.Pool
	pgStore PostgresStoreInterface
	logger  *zap.Logger
}

// Setup is run at the beginning of a new session
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	h.logger.Info("consumer group session started")
	return nil
}

// Cleanup is run at the end of a session
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.logger.Info("consumer group session ended")
	return nil
}

// ConsumeClaim processes messages from a partition
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case <-session.Context().Done():
			return nil
		case message, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			if message == nil {
				continue
			}

			h.logger.Info("received batch job message",
				zap.String("key", string(message.Key)),
				zap.Int64("offset", message.Offset),
			)

			// Parse job message
			var jobMsg map[string]interface{}
			if err := json.Unmarshal(message.Value, &jobMsg); err != nil {
				h.logger.Error("failed to unmarshal message", zap.Error(err))
				session.MarkMessage(message, "")
				continue
			}

			// Extract job details
			jobID, _ := jobMsg["job_id"].(string)
			model, _ := jobMsg["model"].(string)
			version, _ := jobMsg["version"].(string)
			inputsRaw, _ := jobMsg["inputs"].([]interface{})

			// Convert inputs
			inputs := make([]map[string]interface{}, 0, len(inputsRaw))
			for _, input := range inputsRaw {
				if inputMap, ok := input.(map[string]interface{}); ok {
					inputs = append(inputs, inputMap)
				}
			}

			// Create job record
			job := &storage.BatchJob{
				ID:         jobID,
				Model:      model,
				Version:    version,
				Inputs:     inputs,
				Status:     storage.StatusPending,
				TotalItems: len(inputs),
				Completed:  0,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}

			// Save job to database
			if err := h.pgStore.CreateJob(session.Context(), job); err != nil {
				h.logger.Error("failed to create job", zap.Error(err))
				session.MarkMessage(message, "")
				continue
			}

			// Process job with worker pool
			if err := h.pool.ProcessJob(session.Context(), job); err != nil {
				h.logger.Error("failed to process job",
					zap.String("job_id", jobID),
					zap.Error(err),
				)
			}

			// Mark message as processed
			session.MarkMessage(message, "")
		}
	}
}
