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

	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "wxfile://") ||
		strings.HasPrefix(lower, "cloud://") {
		return raw
	}

	if strings.Contains(lower, "/api/v1/assets/preview?raw=") {
		return raw
	}

	cfg := config.Load()
	if shouldProxyStoredAsset(cfg, raw) {
		if proxyURL := buildAssetProxyURL(cfg, raw); proxyURL != "" {
			return proxyURL
		}
	}

	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return raw
	}

	provider, err := GetStorageProvider()
	if err == nil && provider != nil {
		if readableURL, readErr := storage.GetDownloadURL(context.Background(), provider, configuredStorageBucket(cfg), raw, 2*time.Hour); readErr == nil && readableURL != "" {
			if shouldProxyStoredAsset(cfg, readableURL) {
				if proxyURL := buildAssetProxyURL(cfg, readableURL); proxyURL != "" {
					return proxyURL
				}
			}
			return readableURL
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

func buildAssetProxyURL(cfg *config.Config, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || cfg == nil {
		return ""
	}

	base := strings.TrimSpace(cfg.Static.Host)
	if base == "" {
		base = strings.TrimSpace(cfg.Static.CDN)
	}
	if base == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(raw), "data:") ||
		strings.HasPrefix(strings.ToLower(raw), "wxfile://") ||
		strings.HasPrefix(strings.ToLower(raw), "cloud://") {
		return raw
	}
	if strings.Contains(raw, "/api/v1/assets/preview?raw=") {
		return raw
	}
	base = strings.TrimRight(base, "/")
	return base + "/api/v1/assets/preview?raw=" + url.QueryEscape(raw)
}

func shouldProxyStoredAsset(cfg *config.Config, raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || cfg == nil {
		return false
	}

	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "wxfile://") ||
		strings.HasPrefix(lower, "cloud://") ||
		strings.Contains(lower, "/api/v1/assets/preview?raw=") {
		return false
	}

	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return true
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Host))
	if host == "" {
		return false
	}
	if strings.Contains(host, ".cos.") && strings.Contains(host, "myqcloud.com") {
		return true
	}

	staticHost := strings.ToLower(strings.TrimSpace(cfg.Static.Host))
	staticCDN := strings.ToLower(strings.TrimSpace(cfg.Static.CDN))
	return host == strings.TrimPrefix(staticHost, "https://") ||
		host == strings.TrimPrefix(staticHost, "http://") ||
		host == strings.TrimPrefix(staticCDN, "https://") ||
		host == strings.TrimPrefix(staticCDN, "http://")
}
