package handler

import (
	"context"
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

	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "wxfile://") ||
		strings.HasPrefix(lower, "cloud://") {
		return raw
	}

	provider, err := GetStorageProvider()
	if err == nil && provider != nil {
		cfg := config.Load()
		if readableURL, readErr := storage.GetDownloadURL(context.Background(), provider, configuredStorageBucket(cfg), raw, 2*time.Hour); readErr == nil && readableURL != "" {
			return readableURL
		}
	}

	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return raw
	}

	cfg := config.Load()
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
