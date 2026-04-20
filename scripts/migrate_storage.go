package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tans/miao/internal/storage"
)

const (
	// RustFS configuration - set these env vars before running
	uploadDir = "web/static/uploads"
	concurrency = 5
)

func main() {
	// Load config from env
	rustfsEndpoint := os.Getenv("RUSTFS_ENDPOINT")
	rustfsBucket := os.Getenv("RUSTFS_BUCKET")
	rustfsToken := os.Getenv("RUSTFS_ACCESS_KEY")
	staticCDN := os.Getenv("STATIC_CDN")

	if rustfsEndpoint == "" || rustfsBucket == "" || rustfsToken == "" {
		log.Fatal("Missing required env vars: RUSTFS_ENDPOINT, RUSTFS_BUCKET, RUSTFS_ACCESS_KEY")
	}

	provider := storage.NewRustFSProvider(storage.RustFSConfig{
		Endpoint:  rustfsEndpoint,
		Bucket:    rustfsBucket,
		AccessKey: rustfsToken,
		CDNHost:   staticCDN,
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

	type result struct {
		path   string
		url    string
		err    error
		status string
	}
	results := make(chan result, len(files))

	for _, filePath := range files {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Calculate key (relative path from uploadDir)
			key, err := filepath.Rel(uploadDir, path)
			if err != nil {
				results <- result{path: path, err: err, status: "skip"}
				return
			}

			// Open file
			f, err := os.Open(path)
			if err != nil {
				results <- result{path: path, err: fmt.Errorf("open file: %w", err), status: "fail"}
				return
			}

			// Get file size
			stat, err := f.Stat()
			if err != nil {
				f.Close()
				results <- result{path: path, err: fmt.Errorf("stat file: %w", err), status: "fail"}
				return
			}

			// Detect content type
			contentType := detectContentType(path)

			// Upload
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			url, err := provider.Upload(ctx, key, f, stat.Size(), contentType)
			cancel()
			f.Close()

			if err != nil {
				mu.Lock()
				failed++
				mu.Unlock()
				results <- result{path: path, err: err, status: "fail"}
				log.Printf("FAIL: %s - %v\n", path, err)
			} else {
				mu.Lock()
				success++
				mu.Unlock()
				results <- result{path: path, url: url, status: "ok"}
				log.Printf("OK: %s -> %s\n", path, url)
			}
		}(filePath)
	}

	wg.Wait()
	close(results)

	log.Printf("\n=== Migration Complete ===")
	log.Printf("Success: %d, Failed: %d\n", success, failed)

	// Print failed files
	if failed > 0 {
		log.Println("\nFailed files:")
		for r := range results {
			if r.status == "fail" {
				log.Printf("  %s: %v\n", r.path, r.err)
			}
		}
	}
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
