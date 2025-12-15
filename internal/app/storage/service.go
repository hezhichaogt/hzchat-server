package storage

import (
	"context"
	"time"
)

// ServiceConfig holds the configuration required to connect to the storage service.
type ServiceConfig struct {
	S3BucketName      string
	S3Endpoint        string
	S3AccessKeyID     string
	S3SecretAccessKey string
}

// StorageService defines the public interface for the file storage service.
type StorageService interface {
	// PresignUpload generates a pre-signed URL for uploading a file.
	PresignUpload(
		ctx context.Context,
		key string,
		mimeType string,
		fileSize int64,
		duration time.Duration,
	) (string, error)

	// PresignDownload generates a pre-signed URL for downloading a file.
	PresignDownload(ctx context.Context, key string, duration time.Duration) (string, error)

	// Delete removes the file specified by the given key.
	Delete(ctx context.Context, key string) error

	// GetObjectMetadata retrieves the object's metadata.
	GetObjectMetadata(ctx context.Context, key string) (map[string]string, error)
}

// NewStorageService is the factory function for StorageService.
// It initializes and returns a concrete implementation based on the provided configuration.
func NewStorageService(cfg ServiceConfig) (StorageService, error) {
	// Currently, only S3 compatible implementations are supported.
	return newS3Client(cfg)
}
