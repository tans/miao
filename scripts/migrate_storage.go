package main

import (
	"context"
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
	uploadDir   = "web/static/uploads"
	concurrency = 5
)

func main() {
	rustfsEndpoint := os.Getenv("RUSTFS_ENDPOINT")
	rustfsBucket := os.Getenv("RUSTFS_BUCKET")
	accessKey := os.Getenv("RUSTFS_ACCESS_KEY")
	secretKey := os.Getenv("RUSTFS_SECRET_KEY")
	staticCDN := os.Getenv("STATIC_CDN")

	if rustfsEndpoint == "" || rustfsBucket == "" || accessKey == "" || secretKey == "" {
		log.Fatal("Missing required env vars: RUSTFS_ENDPOINT, RUSTFS_BUCKET, RUSTFS_ACCESS_KEY, RUSTFS_SECRET_KEY")
	}

	resolver := aws.EndpointResolverWithOptionsFunc(
		func(endpoint, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               rustfsEndpoint,
				SigningRegion:     "us-east-1",
				HostnameImmutable: true,
				Source:            aws.EndpointSourceCustom,
			}, nil
		},
	)

	awsCfg := aws.Config{
		Region:                      "us-east-1",
		Credentials:                 credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		EndpointResolverWithOptions: resolver,
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	// Find all files
	var files []string
	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.HasPrefix(info.Name(), ".") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to scan directory: %v", err)
	}

	log.Printf("Found %d files to migrate\n", len(files))

	// Migrate files with concurrency limit
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var success, failed int

	for _, filePath := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			key, err := filepath.Rel(uploadDir, path)
			if err != nil {
				log.Printf("FAIL: %s - %v\n", path, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			f, err := os.Open(path)
			if err != nil {
				log.Printf("FAIL: %s - open error: %v\n", path, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			contentType := detectContentType(path)

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)

			_, err = client.PutObject(ctx, &s3.PutObjectInput{
				Bucket:      &rustfsBucket,
				Key:         &key,
				Body:        f,
				ContentType: &contentType,
			})
			cancel()
			f.Close()

			if err != nil {
				log.Printf("FAIL: %s - %v\n", path, err)
				mu.Lock()
				failed++
				mu.Unlock()
			} else {
				url := key
				if staticCDN != "" {
					url = staticCDN + "/" + key
				}
				log.Printf("OK: %s -> %s\n", path, url)
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(filePath)
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
