package storage

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

// LocalStorage implements StorageProvider for local filesystem storage.
// This is the default fallback when no cloud storage is configured.
type LocalStorage struct {
	baseDir string
	baseURL string
}

// NewLocalStorage creates a new LocalStorage instance.
func NewLocalStorage(baseDir, baseURL string) *LocalStorage {
	return &LocalStorage{
		baseDir: baseDir,
		baseURL: baseURL,
	}
}

// Upload saves a file to the local filesystem.
func (s *LocalStorage) Upload(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error) {
	// Ensure directory exists
	dir := filepath.Join(s.baseDir, filepath.Dir(key))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create directory: %w", err)
	}

	// Create file
	filePath := filepath.Join(s.baseDir, key)
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	// Copy data
	written, err := io.Copy(f, file)
	if err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	if written != size {
		os.Remove(filePath)
		return "", fmt.Errorf("size mismatch: expected %d, wrote %d", size, written)
	}

	return s.GetURL(ctx, key)
}

// Delete removes a file from local storage.
func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	filePath := filepath.Join(s.baseDir, key)
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("delete file: %w", err)
	}
	return nil
}

// GetURL returns the public URL for a local file.
func (s *LocalStorage) GetURL(ctx context.Context, key string) (string, error) {
	url := fmt.Sprintf("%s/%s", s.baseURL, key)
	return url, nil
}

// Exists checks if a file exists in local storage.
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	filePath := filepath.Join(s.baseDir, key)
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat file: %w", err)
	}
	return true, nil
}

// ServeFile serves a local file via HTTP (for development only).
func (s *LocalStorage) ServeFile(w http.ResponseWriter, r *http.Request, key string) {
	filePath := filepath.Join(s.baseDir, key)
	http.ServeFile(w, r, filePath)
}

// GetUploadSignedURL returns an error since local storage does not support presigned URLs.
func (s *LocalStorage) GetUploadSignedURL(ctx context.Context, key, contentType string, expiresInSeconds int) (string, error) {
	return "", fmt.Errorf("presigned URLs are not supported for local storage")
}

func (s *LocalStorage) CopyObject(ctx context.Context, srcKey, dstKey string) error {
	srcPath := filepath.Join(s.baseDir, filepath.Clean(srcKey))
	dstPath := filepath.Join(s.baseDir, filepath.Clean(dstKey))

	data, err := os.ReadFile(srcPath)
	if err != nil {
		if errorsIsNotExist(err) {
			return fs.ErrNotExist
		}
		return fmt.Errorf("read source file: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}
	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return fmt.Errorf("write destination file: %w", err)
	}
	return nil
}

func errorsIsNotExist(err error) bool {
	return os.IsNotExist(err)
}
