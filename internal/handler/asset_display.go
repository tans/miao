package handler

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/storage"
)

func resolveStoredAssetURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	raw = unwrapAssetPreviewURL(raw)

	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "wxfile://") ||
		strings.HasPrefix(lower, "cloud://") {
		return raw
	}

	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return raw
	}

	cfg := config.Load()
	provider, err := GetStorageProvider()
	if err == nil && provider != nil {
		if readableURL, readErr := storage.ResolveDisplayURL(context.Background(), provider, configuredStorageBucket(cfg), raw, 2*time.Hour); readErr == nil && readableURL != "" {
			if strings.HasPrefix(strings.ToLower(readableURL), "http://") || strings.HasPrefix(strings.ToLower(readableURL), "https://") {
				return readableURL
			}
			return raw
		}
	}

	base := strings.TrimSpace(cfg.Static.CDN)
	if base == "" {
		base = strings.TrimSpace(cfg.Static.Host)
	}
	if base == "" {
		return raw
	}

	base = strings.TrimRight(base, "/")
	if strings.HasPrefix(raw, "/") {
		return base + raw
	}
	return base + "/" + raw
}

func unwrapAssetPreviewURL(raw string) string {
	const marker = "/api/v1/assets/preview"

	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || !strings.Contains(strings.ToLower(trimmed), marker) {
		return raw
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return raw
	}
	if !strings.Contains(strings.ToLower(parsed.Path), marker) {
		return raw
	}

	encoded := parsed.Query().Get("raw")
	if encoded == "" {
		return raw
	}
	decoded, err := url.QueryUnescape(encoded)
	if err != nil || decoded == "" {
		return raw
	}
	return decoded
}
