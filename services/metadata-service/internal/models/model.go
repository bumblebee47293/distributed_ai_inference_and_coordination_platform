package models

import "time"

// ModelMetadata represents metadata for an ML model
type ModelMetadata struct {
	ID              string            `json:"id" db:"id"`
	Name            string            `json:"name" db:"name"`
	Version         string            `json:"version" db:"version"`
	Framework       string            `json:"framework" db:"framework"` // pytorch, tensorflow, onnx
	Format          string            `json:"format" db:"format"`       // onnx, torchscript, savedmodel
	Description     string            `json:"description" db:"description"`
	InputShape      string            `json:"input_shape" db:"input_shape"`
	OutputShape     string            `json:"output_shape" db:"output_shape"`
	Tags            []string          `json:"tags" db:"tags"`
	Status          string            `json:"status" db:"status"` // active, deprecated, archived
	BackendURL      string            `json:"backend_url" db:"backend_url"`
	AvgLatencyMs    float64           `json:"avg_latency_ms" db:"avg_latency_ms"`
	RequestCount    int64             `json:"request_count" db:"request_count"`
	ErrorRate       float64           `json:"error_rate" db:"error_rate"`
	CreatedBy       string            `json:"created_by" db:"created_by"`
	CreatedAt       time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at" db:"updated_at"`
	Metadata        map[string]string `json:"metadata" db:"metadata"` // Additional key-value pairs
}

// CreateModelRequest represents a request to create a new model
type CreateModelRequest struct {
	Name        string            `json:"name" binding:"required"`
	Version     string            `json:"version" binding:"required"`
	Framework   string            `json:"framework" binding:"required"`
	Format      string            `json:"format" binding:"required"`
	Description string            `json:"description"`
	InputShape  string            `json:"input_shape"`
	OutputShape string            `json:"output_shape"`
	Tags        []string          `json:"tags"`
	BackendURL  string            `json:"backend_url" binding:"required"`
	CreatedBy   string            `json:"created_by"`
	Metadata    map[string]string `json:"metadata"`
}

// UpdateModelRequest represents a request to update a model
type UpdateModelRequest struct {
	Description  *string            `json:"description"`
	Status       *string            `json:"status"`
	BackendURL   *string            `json:"backend_url"`
	Tags         []string           `json:"tags"`
	Metadata     map[string]string  `json:"metadata"`
}

// ModelStats represents statistics for a model
type ModelStats struct {
	ModelID      string    `json:"model_id"`
	Version      string    `json:"version"`
	RequestCount int64     `json:"request_count"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	ErrorRate    float64   `json:"error_rate"`
	LastUsed     time.Time `json:"last_used"`
}
