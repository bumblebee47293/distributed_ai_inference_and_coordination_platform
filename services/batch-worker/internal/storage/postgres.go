package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// JobStatus represents the status of a batch job
type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
)

// BatchJob represents a batch inference job
type BatchJob struct {
	ID          string                   `json:"id"`
	Model       string                   `json:"model"`
	Version     string                   `json:"version"`
	Inputs      []map[string]interface{} `json:"inputs"`
	Status      JobStatus                `json:"status"`
	Progress    float64                  `json:"progress"`
	TotalItems  int                      `json:"total_items"`
	Completed   int                      `json:"completed"`
	ResultURL   string                   `json:"result_url,omitempty"`
	ErrorMsg    string                   `json:"error_msg,omitempty"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
	CompletedAt *time.Time               `json:"completed_at,omitempty"`
}

// PostgresStore handles database operations for batch jobs
type PostgresStore struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewPostgresStore creates a new PostgreSQL store
func NewPostgresStore(connectionURL string, logger *zap.Logger) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	store := &PostgresStore{
		db:     db,
		logger: logger,
	}

	// Initialize schema
	if err := store.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return store, nil
}

// initSchema creates the batch_jobs table if it doesn't exist
func (s *PostgresStore) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS batch_jobs (
		id VARCHAR(255) PRIMARY KEY,
		model VARCHAR(255) NOT NULL,
		version VARCHAR(50) NOT NULL,
		inputs JSONB NOT NULL,
		status VARCHAR(50) NOT NULL,
		progress FLOAT DEFAULT 0,
		total_items INT NOT NULL,
		completed INT DEFAULT 0,
		result_url TEXT,
		error_msg TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		completed_at TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_batch_jobs_status ON batch_jobs(status);
	CREATE INDEX IF NOT EXISTS idx_batch_jobs_created_at ON batch_jobs(created_at);
	`

	_, err := s.db.Exec(query)
	return err
}

// CreateJob creates a new batch job
func (s *PostgresStore) CreateJob(ctx context.Context, job *BatchJob) error {
	inputsJSON, err := json.Marshal(job.Inputs)
	if err != nil {
		return fmt.Errorf("failed to marshal inputs: %w", err)
	}

	query := `
		INSERT INTO batch_jobs (id, model, version, inputs, status, total_items, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err = s.db.ExecContext(ctx, query,
		job.ID,
		job.Model,
		job.Version,
		inputsJSON,
		job.Status,
		job.TotalItems,
		job.CreatedAt,
		job.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	s.logger.Info("created batch job",
		zap.String("job_id", job.ID),
		zap.String("model", job.Model),
		zap.Int("total_items", job.TotalItems),
	)

	return nil
}

// UpdateJobProgress updates the progress of a batch job
func (s *PostgresStore) UpdateJobProgress(ctx context.Context, jobID string, completed int, progress float64) error {
	query := `
		UPDATE batch_jobs
		SET completed = $1, progress = $2, updated_at = $3
		WHERE id = $4
	`

	_, err := s.db.ExecContext(ctx, query, completed, progress, time.Now(), jobID)
	if err != nil {
		return fmt.Errorf("failed to update job progress: %w", err)
	}

	return nil
}

// UpdateJobStatus updates the status of a batch job
func (s *PostgresStore) UpdateJobStatus(ctx context.Context, jobID string, status JobStatus, resultURL, errorMsg string) error {
	query := `
		UPDATE batch_jobs
		SET status = $1, result_url = $2, error_msg = $3, updated_at = $4, completed_at = $5
		WHERE id = $6
	`

	var completedAt *time.Time
	if status == StatusCompleted || status == StatusFailed {
		now := time.Now()
		completedAt = &now
	}

	_, err := s.db.ExecContext(ctx, query, status, resultURL, errorMsg, time.Now(), completedAt, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	s.logger.Info("updated job status",
		zap.String("job_id", jobID),
		zap.String("status", string(status)),
	)

	return nil
}

// GetJob retrieves a batch job by ID
func (s *PostgresStore) GetJob(ctx context.Context, jobID string) (*BatchJob, error) {
	query := `
		SELECT id, model, version, inputs, status, progress, total_items, completed,
		       result_url, error_msg, created_at, updated_at, completed_at
		FROM batch_jobs
		WHERE id = $1
	`

	var job BatchJob
	var inputsJSON []byte
	var resultURL, errorMsg sql.NullString
	var completedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, jobID).Scan(
		&job.ID,
		&job.Model,
		&job.Version,
		&inputsJSON,
		&job.Status,
		&job.Progress,
		&job.TotalItems,
		&job.Completed,
		&resultURL,
		&errorMsg,
		&job.CreatedAt,
		&job.UpdatedAt,
		&completedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if err := json.Unmarshal(inputsJSON, &job.Inputs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal inputs: %w", err)
	}

	if resultURL.Valid {
		job.ResultURL = resultURL.String
	}
	if errorMsg.Valid {
		job.ErrorMsg = errorMsg.String
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return &job, nil
}

// Close closes the database connection
func (s *PostgresStore) Close() error {
	return s.db.Close()
}
