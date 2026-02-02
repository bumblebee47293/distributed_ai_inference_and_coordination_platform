package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/zap"
)

// MinIOStore handles object storage operations
type MinIOStore struct {
	client *minio.Client
	bucket string
	logger *zap.Logger
}

// NewMinIOStore creates a new MinIO store
func NewMinIOStore(endpoint, accessKey, secretKey, bucket string, logger *zap.Logger) (*MinIOStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false, // Set to true for HTTPS
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	store := &MinIOStore{
		client: client,
		bucket: bucket,
		logger: logger,
	}

	// Ensure bucket exists
	if err := store.ensureBucket(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket: %w", err)
	}

	return store, nil
}

// ensureBucket creates the bucket if it doesn't exist
func (s *MinIOStore) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}

	if !exists {
		err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
		if err != nil {
			return err
		}
		s.logger.Info("created bucket", zap.String("bucket", s.bucket))
	}

	return nil
}

// UploadResults uploads batch inference results to MinIO
func (s *MinIOStore) UploadResults(ctx context.Context, jobID string, results []map[string]interface{}) (string, error) {
	// Convert results to JSON
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	// Object name: results/{jobID}.json
	objectName := fmt.Sprintf("results/%s.json", jobID)

	// Upload to MinIO
	_, err = s.client.PutObject(
		ctx,
		s.bucket,
		objectName,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: "application/json",
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to upload results: %w", err)
	}

	// Generate presigned URL (valid for 7 days)
	url, err := s.client.PresignedGetObject(ctx, s.bucket, objectName, 7*24*3600, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	s.logger.Info("uploaded results",
		zap.String("job_id", jobID),
		zap.String("object", objectName),
		zap.Int("size_bytes", len(data)),
	)

	return url.String(), nil
}

// GetResults retrieves batch inference results from MinIO
func (s *MinIOStore) GetResults(ctx context.Context, jobID string) ([]map[string]interface{}, error) {
	objectName := fmt.Sprintf("results/%s.json", jobID)

	object, err := s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer object.Close()

	var results []map[string]interface{}
	if err := json.NewDecoder(object).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	return results, nil
}
