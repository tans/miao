package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
// It uses the AWS SDK v2 S3 API with forcePathStyle for RustFS compatibility.
type RustFSProvider struct {
	config   RustFSConfig
	client   *s3.Client
	presigner *s3.PresignClient
}

// NewRustFSProvider creates a new RustFS storage provider using AWS SDK v2.
func NewRustFSProvider(config RustFSConfig) *RustFSProvider {
	if config.Region == "" {
		config.Region = "us-east-1"
	}

	resolver := aws.EndpointResolverWithOptionsFunc(
		func( endpoint, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               config.Endpoint,
				SigningRegion:     config.Region,
				HostnameImmutable: true,
				Source:            aws.EndpointSourceCustom,
			}, nil
		},
	)

	awsCfg := aws.Config{
		Region:                      config.Region,
		Credentials:                 credentials.NewStaticCredentialsProvider(config.AccessKey, config.SecretKey, ""),
		EndpointResolverWithOptions: resolver,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.UseAccelerate = false
	})

	return &RustFSProvider{
		config:   config,
		client:   client,
		presigner: s3.NewPresignClient(client),
	}
}

// Upload uploads a file to RustFS using AWS SDK v2 S3 API.
func (p *RustFSProvider) Upload(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("read data: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket:      &p.config.Bucket,
		Key:         &key,
		Body:        bytes.NewReader(data),
		ContentType: &contentType,
	}

	_, err = p.client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("put object: %w", err)
	}

	return p.GetURL(ctx, key)
}

// Delete deletes a file from RustFS.
func (p *RustFSProvider) Delete(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: &p.config.Bucket,
		Key:    &key,
	}

	_, err := p.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
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
	input := &s3.HeadObjectInput{
		Bucket: &p.config.Bucket,
		Key:    &key,
	}

	_, err := p.client.HeadObject(ctx, input)
	if err != nil {
		// Check if it's a "not found" error
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, fmt.Errorf("head object: %w", err)
	}

	return true, nil
}

// GetSignedURL returns a pre-signed URL for temporary private access.
func (p *RustFSProvider) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: &p.config.Bucket,
		Key:    &key,
	}

	presignedReq, err := p.presigner.PresignGetObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("presign get object: %w", err)
	}

	return presignedReq.URL, nil
}
