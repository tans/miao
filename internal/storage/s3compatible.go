package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

// S3CompatibleConfig holds the configuration for S3-compatible storage.
type S3CompatibleConfig struct {
	Provider          string // Provider type: rustfs, s3, oss, cos
	Endpoint          string // API server URL (e.g., https://cos.ap-guangzhou.myqcloud.com)
	Bucket            string // Bucket name
	AccessKey         string // Access key ID
	SecretKey         string // Secret access key
	Region            string // Region (default: "us-east-1")
	CDNHost           string // CDN hostname for public URLs
	UsePathStyle      bool   // Use path-style addressing (true for S3/rustfs, false for COS)
	HostnameImmutable bool   // Keep endpoint host unchanged when signing requests
}

// S3CompatibleProvider implements StorageProvider for S3-compatible object storage.
type S3CompatibleProvider struct {
	config    S3CompatibleConfig
	client    *s3.Client
	presigner *s3.PresignClient
	cosClient *cos.Client
}

// NewS3CompatibleProvider creates a new S3-compatible storage provider using AWS SDK v2.
func NewS3CompatibleProvider(config S3CompatibleConfig) *S3CompatibleProvider {
	if config.Region == "" {
		config.Region = "us-east-1"
	}

	resolver := aws.EndpointResolverWithOptionsFunc(
		func(endpoint, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               config.Endpoint,
				SigningRegion:     config.Region,
				HostnameImmutable: config.HostnameImmutable,
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
		o.UsePathStyle = config.UsePathStyle
		o.UseAccelerate = false
	})

	var cosClient *cos.Client
	if strings.EqualFold(config.Provider, "cos") {
		bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region))
		if err == nil {
			baseURL := &cos.BaseURL{BucketURL: bucketURL}
			cosClient = cos.NewClient(baseURL, &http.Client{
				Transport: &cos.AuthorizationTransport{
					SecretID:  config.AccessKey,
					SecretKey: config.SecretKey,
				},
			})
		}
	}

	return &S3CompatibleProvider{
		config:    config,
		client:    client,
		presigner: s3.NewPresignClient(client),
		cosClient: cosClient,
	}
}

// Upload uploads a file to S3-compatible storage using AWS SDK v2 S3 API.
func (p *S3CompatibleProvider) Upload(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error) {
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

// Delete deletes a file from S3-compatible storage.
func (p *S3CompatibleProvider) Delete(ctx context.Context, key string) error {
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
func (p *S3CompatibleProvider) GetURL(ctx context.Context, key string) (string, error) {
	if p.config.CDNHost != "" {
		return fmt.Sprintf("%s/%s", p.config.CDNHost, key), nil
	}
	return fmt.Sprintf("%s/%s/%s", p.config.Endpoint, p.config.Bucket, key), nil
}

// Exists checks if a file exists in S3-compatible storage.
func (p *S3CompatibleProvider) Exists(ctx context.Context, key string) (bool, error) {
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
func (p *S3CompatibleProvider) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	if p.cosClient != nil {
		presignedURL, err := p.cosClient.Object.GetPresignedURL(ctx, http.MethodGet, key, p.config.AccessKey, p.config.SecretKey, expiry, nil)
		if err != nil {
			return "", fmt.Errorf("cos presign get object: %w", err)
		}
		return presignedURL.String(), nil
	}

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

// GetUploadSignedURL returns a pre-signed PUT URL for direct client upload.
func (p *S3CompatibleProvider) GetUploadSignedURL(ctx context.Context, key, contentType string, expiresInSeconds int) (string, error) {
	if p.cosClient != nil {
		presignedURL, err := p.cosClient.Object.GetPresignedURL(ctx, http.MethodPut, key, p.config.AccessKey, p.config.SecretKey, time.Duration(expiresInSeconds)*time.Second, nil)
		if err != nil {
			return "", fmt.Errorf("cos presign put object: %w", err)
		}
		return presignedURL.String(), nil
	}

	input := &s3.PutObjectInput{
		Bucket:      &p.config.Bucket,
		Key:         &key,
		ContentType: &contentType,
	}

	presignedReq, err := p.presigner.PresignPutObject(ctx, input, func(o *s3.PresignOptions) {
		o.Expires = time.Duration(expiresInSeconds) * time.Second
	})
	if err != nil {
		return "", fmt.Errorf("presign put object: %w", err)
	}

	return presignedReq.URL, nil
}

func (p *S3CompatibleProvider) DownloadObject(ctx context.Context, key, rangeHeader string) (*ObjectDownload, error) {
	if p.cosClient != nil {
		var opt *cos.ObjectGetOptions
		if strings.TrimSpace(rangeHeader) != "" {
			opt = &cos.ObjectGetOptions{Range: rangeHeader}
		}
		resp, err := p.cosClient.Object.Get(ctx, key, opt)
		if err != nil {
			return nil, err
		}
		return &ObjectDownload{
			Body:         resp.Body,
			ContentType:  resp.Header.Get("Content-Type"),
			ContentRange: resp.Header.Get("Content-Range"),
			AcceptRanges: resp.Header.Get("Accept-Ranges"),
			ETag:         resp.Header.Get("ETag"),
			ContentLen:   resp.ContentLength,
			StatusCode:   resp.StatusCode,
		}, nil
	}

	input := &s3.GetObjectInput{
		Bucket: &p.config.Bucket,
		Key:    &key,
	}
	if strings.TrimSpace(rangeHeader) != "" {
		input.Range = &rangeHeader
	}

	resp, err := p.client.GetObject(ctx, input)
	if err != nil {
		return nil, err
	}

	contentType := ""
	if resp.ContentType != nil {
		contentType = *resp.ContentType
	}
	contentRange := ""
	if resp.ContentRange != nil {
		contentRange = *resp.ContentRange
	}
	acceptRanges := ""
	if resp.AcceptRanges != nil {
		acceptRanges = *resp.AcceptRanges
	}
	etag := ""
	if resp.ETag != nil {
		etag = *resp.ETag
	}
	contentLen := int64(-1)
	if resp.ContentLength != nil {
		contentLen = *resp.ContentLength
	}

	statusCode := http.StatusOK
	if contentRange != "" {
		statusCode = http.StatusPartialContent
	}

	return &ObjectDownload{
		Body:         resp.Body,
		ContentType:  contentType,
		ContentRange: contentRange,
		AcceptRanges: acceptRanges,
		ETag:         etag,
		ContentLen:   contentLen,
		StatusCode:   statusCode,
	}, nil
}
