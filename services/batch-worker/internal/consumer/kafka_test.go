package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/yourusername/ai-platform/batch-worker/internal/storage"
	"github.com/yourusername/ai-platform/batch-worker/internal/worker"
	"go.uber.org/zap"
)

// MockConsumerGroupSession implements sarama.ConsumerGroupSession
type MockConsumerGroupSession struct {
	ctx     context.Context
	marked  map[string]int64
	commits map[string]int64
}

func NewMockConsumerGroupSession() *MockConsumerGroupSession {
	return &MockConsumerGroupSession{
		ctx:     context.Background(),
		marked:  make(map[string]int64),
		commits: make(map[string]int64),
	}
}

func (m *MockConsumerGroupSession) Claims() map[string][]int32 {
	return map[string][]int32{"test-topic": {0}}
}

func (m *MockConsumerGroupSession) MemberID() string {
	return "test-member"
}

func (m *MockConsumerGroupSession) GenerationID() int32 {
	return 1
}

func (m *MockConsumerGroupSession) MarkOffset(topic string, partition int32, offset int64, metadata string) {
	m.marked[topic] = offset
}

func (m *MockConsumerGroupSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	m.marked[msg.Topic] = msg.Offset
}

func (m *MockConsumerGroupSession) Commit() {
	for topic, offset := range m.marked {
		m.commits[topic] = offset
	}
}

func (m *MockConsumerGroupSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {
}

func (m *MockConsumerGroupSession) Context() context.Context {
	return m.ctx
}

// MockConsumerGroupClaim implements sarama.ConsumerGroupClaim
type MockConsumerGroupClaim struct {
	topic     string
	partition int32
	messages  chan *sarama.ConsumerMessage
}

func NewMockConsumerGroupClaim(topic string, partition int32) *MockConsumerGroupClaim {
	return &MockConsumerGroupClaim{
		topic:     topic,
		partition: partition,
		messages:  make(chan *sarama.ConsumerMessage, 10),
	}
}

func (m *MockConsumerGroupClaim) Topic() string {
	return m.topic
}

func (m *MockConsumerGroupClaim) Partition() int32 {
	return m.partition
}

func (m *MockConsumerGroupClaim) InitialOffset() int64 {
	return 0
}

func (m *MockConsumerGroupClaim) HighWaterMarkOffset() int64 {
	return 100
}

func (m *MockConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage {
	return m.messages
}

func TestConsumerGroupHandler_Setup(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pgStore := &storage.PostgresStore{}
	pool := &worker.Pool{}

	handler := &consumerGroupHandler{
		pool:    pool,
		pgStore: pgStore,
		logger:  logger,
	}

	session := NewMockConsumerGroupSession()
	err := handler.Setup(session)

	assert.NoError(t, err)
}

func TestConsumerGroupHandler_Cleanup(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pgStore := &storage.PostgresStore{}
	pool := &worker.Pool{}

	handler := &consumerGroupHandler{
		pool:    pool,
		pgStore: pgStore,
		logger:  logger,
	}

	session := NewMockConsumerGroupSession()
	err := handler.Cleanup(session)

	assert.NoError(t, err)
}

func TestConsumerGroupHandler_ConsumeClaim_ValidMessage(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Create mock stores
	pgStore := &MockPostgresStore{
		jobs: make(map[string]*storage.BatchJob),
	}
	
	minioStore := &MockMinIOStore{
		uploadedResults: make(map[string][]map[string]interface{}),
	}

	// Create pool with mock server
	pool := worker.NewPool(1, "http://localhost:8082", pgStore, minioStore, logger)

	handler := &consumerGroupHandler{
		pool:    pool,
		pgStore: pgStore,
		logger:  logger,
	}

	session := NewMockConsumerGroupSession()
	claim := NewMockConsumerGroupClaim("test-topic", 0)

	// Create a valid job message
	jobMsg := map[string]interface{}{
		"job_id":  "test-job-123",
		"model":   "resnet18",
		"version": "v1",
		"inputs": []interface{}{
			map[string]interface{}{"data": []float64{1.0, 2.0}},
		},
	}

	msgData, _ := json.Marshal(jobMsg)
	message := &sarama.ConsumerMessage{
		Topic:     "test-topic",
		Partition: 0,
		Offset:    1,
		Key:       []byte("test-job-123"),
		Value:     msgData,
		Timestamp: time.Now(),
	}

	// Send message and close channel
	go func() {
		claim.messages <- message
		close(claim.messages)
	}()

	// This will process the message and return when channel is closed
	err := handler.ConsumeClaim(session, claim)

	assert.NoError(t, err)
	assert.Equal(t, int64(1), session.marked["test-topic"])
}

func TestConsumerGroupHandler_ConsumeClaim_InvalidJSON(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	pgStore := &MockPostgresStore{jobs: make(map[string]*storage.BatchJob)}
	minioStore := &MockMinIOStore{uploadedResults: make(map[string][]map[string]interface{})}
	pool := worker.NewPool(1, "http://localhost:8082", pgStore, minioStore, logger)

	handler := &consumerGroupHandler{
		pool:    pool,
		pgStore: pgStore,
		logger:  logger,
	}

	session := NewMockConsumerGroupSession()
	claim := NewMockConsumerGroupClaim("test-topic", 0)

	// Invalid JSON message
	message := &sarama.ConsumerMessage{
		Topic:     "test-topic",
		Partition: 0,
		Offset:    1,
		Key:       []byte("test-job-invalid"),
		Value:     []byte("invalid json"),
		Timestamp: time.Now(),
	}

	go func() {
		claim.messages <- message
		close(claim.messages)
	}()

	err := handler.ConsumeClaim(session, claim)

	// Should not error, but should mark message as processed
	assert.NoError(t, err)
	assert.Equal(t, int64(1), session.marked["test-topic"])
}

// Mock implementations for testing
type MockPostgresStore struct {
	jobs map[string]*storage.BatchJob
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

type MockMinIOStore struct {
	uploadedResults map[string][]map[string]interface{}
}

func (m *MockMinIOStore) UploadResults(ctx context.Context, jobID string, results []map[string]interface{}) (string, error) {
	m.uploadedResults[jobID] = results
	return "http://minio/results/" + jobID + ".json", nil
}
