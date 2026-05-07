package storage

import (
	"context"
	"io"
	"testing"
	"time"
)

type stubSignerProvider struct {
	url string
}

func (s *stubSignerProvider) Upload(ctx context.Context, key string, file io.Reader, size int64, contentType string) (string, error) {
	panic("not used")
}

func (s *stubSignerProvider) Delete(ctx context.Context, key string) error { return nil }
func (s *stubSignerProvider) GetURL(ctx context.Context, key string) (string, error) {
	return s.url + "/" + key, nil
}
func (s *stubSignerProvider) Exists(ctx context.Context, key string) (bool, error) { return true, nil }
func (s *stubSignerProvider) GetUploadSignedURL(ctx context.Context, key, contentType string, expiresInSeconds int) (string, error) {
	return "", nil
}
func (s *stubSignerProvider) GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	return "signed://" + key, nil
}

func TestResolveDisplayURLSignsProtectedAssetKeys(t *testing.T) {
	provider := &stubSignerProvider{url: "https://cdn.example.com"}

	got, err := ResolveDisplayURL(context.Background(), provider, "bucket", "https://cdn.example.com/claim-processed/162/a.mp4", time.Hour)
	if err != nil {
		t.Fatalf("ResolveDisplayURL() error = %v", err)
	}
	if got != "signed://claim-processed/162/a.mp4" {
		t.Fatalf("ResolveDisplayURL() = %q, want signed url", got)
	}

	got, err = ResolveDisplayURL(context.Background(), provider, "bucket", "https://cdn.example.com/private/license/1/a.jpg", time.Hour)
	if err != nil {
		t.Fatalf("ResolveDisplayURL() error = %v", err)
	}
	if got != "signed://private/license/1/a.jpg" {
		t.Fatalf("ResolveDisplayURL() = %q, want signed url", got)
	}

	got, err = ResolveDisplayURL(context.Background(), provider, "bucket", "https://cdn.example.com/public/watermarked/162/a.mp4", time.Hour)
	if err != nil {
		t.Fatalf("ResolveDisplayURL() error = %v", err)
	}
	if got != "https://cdn.example.com/public/watermarked/162/a.mp4" {
		t.Fatalf("ResolveDisplayURL() = %q, want public url", got)
	}
}
