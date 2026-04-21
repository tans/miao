package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	concurrency = 5
)

func main() {
	// RustFS (source) configuration
	rustfsEndpoint := os.Getenv("RUSTFS_ENDPOINT")
	rustfsBucket := os.Getenv("RUSTFS_BUCKET")
	rustfsAccessKey := os.Getenv("RUSTFS_ACCESS_KEY")
	rustfsSecretKey := os.Getenv("RUSTFS_SECRET_KEY")

	// COS (destination) configuration
	cosEndpoint := os.Getenv("COS_ENDPOINT")
	cosBucket := os.Getenv("COS_BUCKET")
	cosRegion := os.Getenv("COS_REGION")
	cosSecretID := os.Getenv("COS_SECRET_ID")
	cosSecretKey := os.Getenv("COS_SECRET_KEY")
	cosAppID := os.Getenv("COS_APP_ID")

	// Validate required env vars
	if rustfsEndpoint == "" || rustfsBucket == "" || rustfsAccessKey == "" || rustfsSecretKey == "" {
		log.Fatal("Missing required RustFS env vars: RUSTFS_ENDPOINT, RUSTFS_BUCKET, RUSTFS_ACCESS_KEY, RUSTFS_SECRET_KEY")
	}
	if cosBucket == "" || cosSecretID == "" || cosSecretKey == "" {
		log.Fatal("Missing required COS env vars: COS_BUCKET, COS_SECRET_ID, COS_SECRET_KEY")
	}
	if cosRegion == "" {
		cosRegion = "ap-guangzhou"
	}
	if cosEndpoint == "" {
		cosEndpoint = "https://cos.ap-guangzhou.myqcloud.com"
	}

	// Initialize RustFS client (source)
	rustfsResolver := aws.EndpointResolverWithOptionsFunc(
		func(endpoint, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               rustfsEndpoint,
				SigningRegion:     "us-east-1",
				HostnameImmutable: true,
				Source:            aws.EndpointSourceCustom,
			}, nil
		},
	)
	rustfsCfg := aws.Config{
		Region:                      "us-east-1",
		Credentials:                 credentials.NewStaticCredentialsProvider(rustfsAccessKey, rustfsSecretKey, ""),
		EndpointResolverWithOptions: rustfsResolver,
		HTTPClient:                   &http.Client{Timeout: 60 * time.Second},
	}
	rustfsClient := s3.NewFromConfig(rustfsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// Initialize COS client (destination)
	// COS bucket naming convention: {bucket}-{appid}
	cosBucketName := fmt.Sprintf("%s-%s", cosBucket, cosAppID)
	cosResolver := aws.EndpointResolverWithOptionsFunc(
		func(endpoint, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cosEndpoint,
				SigningRegion:     cosRegion,
				HostnameImmutable: true,
				Source:            aws.EndpointSourceCustom,
			}, nil
		},
	)
	cosCfg := aws.Config{
		Region:                      cosRegion,
		Credentials:                 credentials.NewStaticCredentialsProvider(cosSecretID, cosSecretKey, ""),
		EndpointResolverWithOptions: cosResolver,
		HTTPClient:                   &http.Client{Timeout: 60 * time.Second},
	}
	cosClient := s3.NewFromConfig(cosCfg, func(o *s3.Options) {
		o.UsePathStyle = false // COS uses virtual-hosted-style
	})

	// List objects from RustFS
	log.Println("Fetching file list from RustFS...")
	result, err := rustfsClient.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: &rustfsBucket,
	})
	if err != nil {
		log.Fatalf("Failed to list RustFS objects: %v", err)
	}

	var files []string
	for _, obj := range result.Contents {
		if obj.Key != nil && *obj.Key != "" {
			files = append(files, *obj.Key)
		}
	}
	log.Printf("Found %d files to migrate\n", len(files))

	if len(files) == 0 {
		log.Println("No files to migrate")
		return
	}

	// Migrate files with concurrency limit
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var success, failed int

	for _, key := range files {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Download from RustFS
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			resp, err := rustfsClient.GetObject(ctx, &s3.GetObjectInput{
				Bucket: &rustfsBucket,
				Key:    &key,
			})
			if err != nil {
				log.Printf("FAIL (download): %s - %v\n", key, err)
				cancel()
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			// Determine content type
			contentType := detectContentType(key)

			// Upload to COS
			_, err = cosClient.PutObject(context.Background(), &s3.PutObjectInput{
				Bucket:      &cosBucketName,
				Key:         &key,
				Body:        resp.Body,
				ContentType: &contentType,
			})
			cancel()

			if err != nil {
				log.Printf("FAIL (upload): %s - %v\n", key, err)
				mu.Lock()
				failed++
				mu.Unlock()
			} else {
				log.Printf("OK: %s", key)
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(key)
	}

	wg.Wait()

	log.Printf("\n=== Migration Complete ===")
	log.Printf("Success: %d, Failed: %d\n", success, failed)
}

func detectContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".avi":
		return "video/x-msvideo"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}