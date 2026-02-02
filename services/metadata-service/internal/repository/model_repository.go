package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/lib/pq"
	"github.com/yourusername/ai-platform/metadata-service/internal/models"
	"go.uber.org/zap"
)

// ModelRepository handles database operations for models
type ModelRepository struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewModelRepository creates a new model repository
func NewModelRepository(connectionURL string, logger *zap.Logger) (*ModelRepository, error) {
	db, err := sql.Open("postgres", connectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	repo := &ModelRepository{
		db:     db,
		logger: logger,
	}

	if err := repo.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return repo, nil
}

// initSchema creates the models table
func (r *ModelRepository) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS models (
		id VARCHAR(255) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		version VARCHAR(50) NOT NULL,
		framework VARCHAR(50) NOT NULL,
		format VARCHAR(50) NOT NULL,
		description TEXT,
		input_shape TEXT,
		output_shape TEXT,
		tags TEXT[],
		status VARCHAR(50) NOT NULL DEFAULT 'active',
		backend_url TEXT NOT NULL,
		avg_latency_ms FLOAT DEFAULT 0,
		request_count BIGINT DEFAULT 0,
		error_rate FLOAT DEFAULT 0,
		created_by VARCHAR(255),
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
		metadata JSONB,
		UNIQUE(name, version)
	);

	CREATE INDEX IF NOT EXISTS idx_models_name ON models(name);
	CREATE INDEX IF NOT EXISTS idx_models_status ON models(status);
	CREATE INDEX IF NOT EXISTS idx_models_created_at ON models(created_at);
	`

	_, err := r.db.Exec(query)
	return err
}

// Create creates a new model
func (r *ModelRepository) Create(ctx context.Context, req *models.CreateModelRequest) (*models.ModelMetadata, error) {
	id := uuid.New().String()
	now := time.Now()

	metadataJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO models (
			id, name, version, framework, format, description,
			input_shape, output_shape, tags, status, backend_url,
			created_by, created_at, updated_at, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING id, created_at, updated_at
	`

	model := &models.ModelMetadata{
		ID:          id,
		Name:        req.Name,
		Version:     req.Version,
		Framework:   req.Framework,
		Format:      req.Format,
		Description: req.Description,
		InputShape:  req.InputShape,
		OutputShape: req.OutputShape,
		Tags:        req.Tags,
		Status:      "active",
		BackendURL:  req.BackendURL,
		CreatedBy:   req.CreatedBy,
		Metadata:    req.Metadata,
	}

	err = r.db.QueryRowContext(ctx, query,
		id, req.Name, req.Version, req.Framework, req.Format,
		req.Description, req.InputShape, req.OutputShape,
		pq.Array(req.Tags), "active", req.BackendURL,
		req.CreatedBy, now, now, metadataJSON,
	).Scan(&model.ID, &model.CreatedAt, &model.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}

	r.logger.Info("created model",
		zap.String("id", model.ID),
		zap.String("name", model.Name),
		zap.String("version", model.Version),
	)

	return model, nil
}

// GetByID retrieves a model by ID
func (r *ModelRepository) GetByID(ctx context.Context, id string) (*models.ModelMetadata, error) {
	query := `
		SELECT id, name, version, framework, format, description,
		       input_shape, output_shape, tags, status, backend_url,
		       avg_latency_ms, request_count, error_rate,
		       created_by, created_at, updated_at, metadata
		FROM models
		WHERE id = $1
	`

	return r.scanModel(r.db.QueryRowContext(ctx, query, id))
}

// GetByNameVersion retrieves a model by name and version
func (r *ModelRepository) GetByNameVersion(ctx context.Context, name, version string) (*models.ModelMetadata, error) {
	query := `
		SELECT id, name, version, framework, format, description,
		       input_shape, output_shape, tags, status, backend_url,
		       avg_latency_ms, request_count, error_rate,
		       created_by, created_at, updated_at, metadata
		FROM models
		WHERE name = $1 AND version = $2
	`

	return r.scanModel(r.db.QueryRowContext(ctx, query, name, version))
}

// List retrieves all models with optional filtering
func (r *ModelRepository) List(ctx context.Context, status string, limit, offset int) ([]*models.ModelMetadata, error) {
	query := `
		SELECT id, name, version, framework, format, description,
		       input_shape, output_shape, tags, status, backend_url,
		       avg_latency_ms, request_count, error_rate,
		       created_by, created_at, updated_at, metadata
		FROM models
		WHERE ($1 = '' OR status = $1)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer rows.Close()

	var models []*models.ModelMetadata
	for rows.Next() {
		model, err := r.scanModelFromRows(rows)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}

	return models, rows.Err()
}

// Update updates a model
func (r *ModelRepository) Update(ctx context.Context, id string, req *models.UpdateModelRequest) (*models.ModelMetadata, error) {
	// Build dynamic update query
	query := `UPDATE models SET updated_at = $1`
	args := []interface{}{time.Now()}
	argCount := 2

	if req.Description != nil {
		query += fmt.Sprintf(", description = $%d", argCount)
		args = append(args, *req.Description)
		argCount++
	}

	if req.Status != nil {
		query += fmt.Sprintf(", status = $%d", argCount)
		args = append(args, *req.Status)
		argCount++
	}

	if req.BackendURL != nil {
		query += fmt.Sprintf(", backend_url = $%d", argCount)
		args = append(args, *req.BackendURL)
		argCount++
	}

	if req.Tags != nil {
		query += fmt.Sprintf(", tags = $%d", argCount)
		args = append(args, pq.Array(req.Tags))
		argCount++
	}

	if req.Metadata != nil {
		metadataJSON, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		query += fmt.Sprintf(", metadata = $%d", argCount)
		args = append(args, metadataJSON)
		argCount++
	}

	query += fmt.Sprintf(" WHERE id = $%d", argCount)
	args = append(args, id)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update model: %w", err)
	}

	r.logger.Info("updated model", zap.String("id", id))

	return r.GetByID(ctx, id)
}

// Delete deletes a model
func (r *ModelRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM models WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("model not found: %s", id)
	}

	r.logger.Info("deleted model", zap.String("id", id))

	return nil
}

// UpdateStats updates model statistics
func (r *ModelRepository) UpdateStats(ctx context.Context, id string, latencyMs float64, success bool) error {
	query := `
		UPDATE models
		SET request_count = request_count + 1,
		    avg_latency_ms = (avg_latency_ms * request_count + $1) / (request_count + 1),
		    error_rate = CASE WHEN $2 THEN error_rate ELSE (error_rate * request_count + 1) / (request_count + 1) END,
		    updated_at = $3
		WHERE id = $4
	`

	_, err := r.db.ExecContext(ctx, query, latencyMs, success, time.Now(), id)
	return err
}

// scanModel scans a single model from a row
func (r *ModelRepository) scanModel(row *sql.Row) (*models.ModelMetadata, error) {
	var model models.ModelMetadata
	var metadataJSON []byte
	var description, inputShape, outputShape, createdBy sql.NullString

	err := row.Scan(
		&model.ID, &model.Name, &model.Version, &model.Framework, &model.Format,
		&description, &inputShape, &outputShape,
		pq.Array(&model.Tags), &model.Status, &model.BackendURL,
		&model.AvgLatencyMs, &model.RequestCount, &model.ErrorRate,
		&createdBy, &model.CreatedAt, &model.UpdatedAt, &metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("model not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan model: %w", err)
	}

	if description.Valid {
		model.Description = description.String
	}
	if inputShape.Valid {
		model.InputShape = inputShape.String
	}
	if outputShape.Valid {
		model.OutputShape = outputShape.String
	}
	if createdBy.Valid {
		model.CreatedBy = createdBy.String
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &model.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &model, nil
}

// scanModelFromRows scans a model from rows
func (r *ModelRepository) scanModelFromRows(rows *sql.Rows) (*models.ModelMetadata, error) {
	var model models.ModelMetadata
	var metadataJSON []byte
	var description, inputShape, outputShape, createdBy sql.NullString

	err := rows.Scan(
		&model.ID, &model.Name, &model.Version, &model.Framework, &model.Format,
		&description, &inputShape, &outputShape,
		pq.Array(&model.Tags), &model.Status, &model.BackendURL,
		&model.AvgLatencyMs, &model.RequestCount, &model.ErrorRate,
		&createdBy, &model.CreatedAt, &model.UpdatedAt, &metadataJSON,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to scan model: %w", err)
	}

	if description.Valid {
		model.Description = description.String
	}
	if inputShape.Valid {
		model.InputShape = inputShape.String
	}
	if outputShape.Valid {
		model.OutputShape = outputShape.String
	}
	if createdBy.Valid {
		model.CreatedBy = createdBy.String
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &model.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &model, nil
}

// Close closes the database connection
func (r *ModelRepository) Close() error {
	return r.db.Close()
}
