package storage

import (
	"context"
	"io"
)

// StorageProvider defines the interface for file storage backends.
// Implementations: LocalStorage, S3CompatibleProvider, etc.
type StorageProvider interface {
	// Upload uploads a file to storage and returns the public access URL.
	Upload(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error)

	// Delete deletes a file from storage.
	Delete(ctx context.Context, key string) error

	// GetURL returns the public access URL for a file.
	GetURL(ctx context.Context, key string) (string, error)

	// Exists checks if a file exists in storage.
	Exists(ctx context.Context, key string) (bool, error)

	// GetUploadSignedURL returns a presigned URL for direct client upload.
	// expiresIn is the duration until the URL expires.
	GetUploadSignedURL(ctx context.Context, key, contentType string, expiresInSeconds int) (string, error)
}
