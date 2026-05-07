package storage

import (
	"context"
	"net/url"
	"strings"
	"time"
)

type downloadSigner interface {
	GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type objectCopier interface {
	CopyObject(ctx context.Context, srcKey, dstKey string) error
}

// ExtractObjectKey derives a storage object key from either a relative path or an absolute URL.
func ExtractObjectKey(raw, bucket string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return strings.TrimLeft(raw, "/")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	key := strings.TrimLeft(parsed.Path, "/")
	bucket = strings.TrimSpace(bucket)
	if bucket != "" && strings.HasPrefix(key, bucket+"/") {
		key = strings.TrimPrefix(key, bucket+"/")
	}
	return key
}

// GetDownloadURL returns a readable URL for the stored object, using a signed URL when supported.
func GetDownloadURL(ctx context.Context, provider StorageProvider, bucket, raw string, expiry time.Duration) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	key := ExtractObjectKey(raw, bucket)
	if signer, ok := provider.(downloadSigner); ok && key != "" {
		return signer.GetSignedURL(ctx, key, expiry)
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw, nil
	}

	if key == "" {
		key = strings.TrimLeft(raw, "/")
	}
	return provider.GetURL(ctx, key)
}

func ResolveDisplayURL(ctx context.Context, provider StorageProvider, bucket, raw string, expiry time.Duration) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}

	key := ExtractObjectKey(raw, bucket)
	if IsProtectedObjectKey(key) {
		return GetDownloadURL(ctx, provider, bucket, raw, expiry)
	}

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw, nil
	}
	if key == "" {
		key = strings.TrimLeft(raw, "/")
	}
	return provider.GetURL(ctx, key)
}

func CopyObject(ctx context.Context, provider StorageProvider, srcKey, dstKey string) error {
	copier, ok := provider.(objectCopier)
	if !ok {
		return ErrUnsupportedOperation
	}
	return copier.CopyObject(ctx, srcKey, dstKey)
}
