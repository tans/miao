package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type downloadSigner interface {
	GetSignedURL(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type ObjectDownload struct {
	Body         io.ReadCloser
	ContentType  string
	ContentRange string
	AcceptRanges string
	ETag         string
	ContentLen   int64
	StatusCode   int
}

type ObjectDownloader interface {
	DownloadObject(ctx context.Context, key, rangeHeader string) (*ObjectDownload, error)
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

func BuildProxyDownloadURL(baseHost, secret, raw string, expiry time.Duration) string {
	baseHost = strings.TrimSpace(baseHost)
	secret = strings.TrimSpace(secret)
	raw = strings.TrimSpace(raw)
	if baseHost == "" || secret == "" || raw == "" {
		return raw
	}

	asset := base64.RawURLEncoding.EncodeToString([]byte(raw))
	expires := strconv.FormatInt(time.Now().Add(expiry).Unix(), 10)
	signature := signProxyDownload(secret, asset, expires)

	values := url.Values{}
	values.Set("asset", asset)
	values.Set("expires", expires)
	values.Set("signature", signature)

	return strings.TrimRight(baseHost, "/") + "/media/claim-asset?" + values.Encode()
}

func VerifyProxyDownloadURL(secret, asset, expires, signature string) (string, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return "", fmt.Errorf("missing proxy secret")
	}
	if asset == "" || expires == "" || signature == "" {
		return "", fmt.Errorf("invalid proxy token")
	}

	expireAt, err := strconv.ParseInt(expires, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid expires")
	}
	if time.Now().Unix() > expireAt {
		return "", fmt.Errorf("proxy token expired")
	}

	expected := signProxyDownload(secret, asset, expires)
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature))) {
		return "", fmt.Errorf("invalid proxy signature")
	}

	decoded, err := base64.RawURLEncoding.DecodeString(asset)
	if err != nil {
		return "", fmt.Errorf("invalid asset")
	}
	return strings.TrimSpace(string(decoded)), nil
}

func IsClaimAssetKey(key string) bool {
	key = strings.TrimLeft(strings.TrimSpace(key), "/")
	return strings.HasPrefix(key, "claim-source/") || strings.HasPrefix(key, "claim-processed/")
}

func signProxyDownload(secret, asset, expires string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(asset))
	_, _ = mac.Write([]byte("\n"))
	_, _ = mac.Write([]byte(expires))
	return hex.EncodeToString(mac.Sum(nil))
}
