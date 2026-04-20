package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// RustFSConfig holds the configuration for RustFS storage.
type RustFSConfig struct {
	Endpoint  string // RustFS API server URL (e.g., https://rustfs.clawos.cc)
	Bucket    string // Bucket name
	AccessKey string // Access key ID
	SecretKey string // Secret access key
	Region    string // Region (default: "us-east-1")
	CDNHost   string // CDN hostname for public URLs
}

// RustFSProvider implements StorageProvider for RustFS object storage.
type RustFSProvider struct {
	config   RustFSConfig
	client   *http.Client
}

// NewRustFSProvider creates a new RustFS storage provider.
func NewRustFSProvider(config RustFSConfig) *RustFSProvider {
	if config.Region == "" {
		config.Region = "us-east-1"
	}
	return &RustFSProvider{
		config: config,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Upload uploads a file to RustFS and returns the public CDN URL.
func (p *RustFSProvider) Upload(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error) {
	// Read all data into memory (for simplicity; for large files, use streaming)
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read data: %w", err)
	}

	// Build the upload URL
	uploadURL := fmt.Sprintf("%s/%s/%s", p.config.Endpoint, p.config.Bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-Length", fmt.Sprintf("%d", size))

	// Add authentication (simplified - adapt based on actual RustFS auth mechanism)
	p.addAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return p.GetURL(ctx, key)
}

// Delete deletes a file from RustFS.
func (p *RustFSProvider) Delete(ctx context.Context, key string) error {
	deleteURL := fmt.Sprintf("%s/%s/%s", p.config.Endpoint, p.config.Bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	p.addAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	return nil
}

// GetURL returns the public CDN URL for a file.
func (p *RustFSProvider) GetURL(ctx context.Context, key string) (string, error) {
	if p.config.CDNHost != "" {
		return fmt.Sprintf("%s/%s", p.config.CDNHost, key), nil
	}
	return fmt.Sprintf("%s/%s/%s", p.config.Endpoint, p.config.Bucket, key), nil
}

// Exists checks if a file exists in RustFS.
func (p *RustFSProvider) Exists(ctx context.Context, key string) (bool, error) {
	headURL := fmt.Sprintf("%s/%s/%s", p.config.Endpoint, p.config.Bucket, key)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, headURL, nil)
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}

	p.addAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("head request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return false, fmt.Errorf("exists check failed: status=%d", resp.StatusCode)
}

// addAuthHeader adds authentication headers to the request.
// Note: This is a placeholder implementation. Adapt based on actual RustFS auth mechanism.
// Common options: AWS S3-style HMAC, Bearer token, API key, etc.
func (p *RustFSProvider) addAuthHeader(req *http.Request) {
	// AWS S3-style signature placeholder
	// In production, implement actual signing: https://docs.aws.amazon.com/AmazonS3/latest/userguide/RESTAuthentication.html
	if p.config.AccessKey != "" && p.config.SecretKey != "" {
		req.Header.Set("X-Access-Key", p.config.AccessKey)
		// TODO: Add proper HMAC signature when RustFS auth mechanism is confirmed
	}
}

// UploadResponse represents the response from RustFS upload API.
type UploadResponse struct {
	URL      string `json:"url"`
	Key      string `json:"key"`
	ETag     string `json:"etag"`
	Size     int64  `json:"size"`
	Status   int    `json:"status"`
	Message  string `json:"message"`
}

func (p *RustFSProvider) parseUploadResponse(body io.Reader) (*UploadResponse, error) {
	var resp UploadResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &resp, nil
}

// GetSignedURL returns a signed URL for private access (if supported).
func (p *RustFSProvider) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	// Build the signed URL request
	signURL := fmt.Sprintf("%s/%s/%s?sign=true&expiry=%d",
		p.config.Endpoint, p.config.Bucket, url.PathEscape(key), int(expiry.Seconds()))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, signURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	p.addAuthHeader(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sign request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("sign failed: status=%d, body=%s", resp.StatusCode, string(body))
	}

	var result struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return result.URL, nil
}
