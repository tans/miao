package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Factory creates StorageProvider instances based on configuration.
type Factory struct {
	staticHost string
	staticCDN  string
	workDir    string
}

// NewFactory creates a new storage factory.
func NewFactory(staticHost, staticCDN, workDir string) *Factory {
	return &Factory{
		staticHost: staticHost,
		staticCDN:  staticCDN,
		workDir:    workDir,
	}
}

// StorageType represents the type of storage backend.
type StorageType string

const (
	StorageTypeLocal  StorageType = "local"
	StorageTypeRustFS StorageType = "rustfs"
	StorageTypeS3     StorageType = "s3"
	StorageTypeOSS    StorageType = "oss"
	StorageTypeCOS    StorageType = "cos"
)

// Config holds the storage configuration.
type Config struct {
	Type   StorageType
	Local  LocalConfig
	RustFS S3CompatibleConfig
	S3     S3Config
	OSS    OSSConfig
	COS    COSConfig
}

// LocalConfig holds local filesystem storage configuration.
type LocalConfig struct {
	BaseDir string
	BaseURL string
}

// S3Config holds AWS S3 compatible storage configuration.
type S3Config struct {
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	CDNHost         string
}

// OSSConfig holds Aliyun OSS configuration.
type OSSConfig struct {
	Endpoint    string
	Bucket      string
	AccessKeyID string
	SecretKey   string
	CDNHost     string
}

// COSConfig holds Tencent Cloud COS configuration.
type COSConfig struct {
	AppID     string
	Bucket    string
	Region    string
	SecretKey string
	SecretID  string
	CDNHost   string
}

// NewProvider creates a StorageProvider based on the configuration.
func (f *Factory) NewProvider(cfg Config) (StorageProvider, error) {
	switch cfg.Type {
	case StorageTypeRustFS:
		return f.newS3CompatibleProvider(cfg.RustFS)
	case StorageTypeS3:
		return f.newS3Provider(cfg.S3)
	case StorageTypeOSS:
		return f.newOSSProvider(cfg.OSS)
	case StorageTypeCOS:
		return f.newCOSProvider(cfg.COS)
	case StorageTypeLocal, "":
		return f.newLocalProvider(cfg.Local)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}

// newLocalProvider creates a LocalStorage provider.
func (f *Factory) newLocalProvider(cfg LocalConfig) (*LocalStorage, error) {
	baseDir := cfg.BaseDir
	if baseDir == "" {
		baseDir = filepath.Join(f.workDir, "web", "static", "uploads")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("%s/static/uploads", f.staticHost)
	}

	// Ensure directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}

	return NewLocalStorage(baseDir, baseURL), nil
}

// newS3CompatibleProvider creates an S3-compatible provider (path-style).
func (f *Factory) newS3CompatibleProvider(cfg S3CompatibleConfig) (*S3CompatibleProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("s3-compatible endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3-compatible bucket is required")
	}

	// Use CDN host if provided, otherwise fallback to endpoint
	if cfg.CDNHost == "" {
		cfg.CDNHost = f.staticCDN
	}

	if cfg.Provider == "" {
		cfg.Provider = "rustfs"
	}
	cfg.UsePathStyle = true // Path-style addressing

	return NewS3CompatibleProvider(cfg), nil
}

// newS3Provider creates an AWS S3 provider.
func (f *Factory) newS3Provider(cfg S3Config) (*S3CompatibleProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("s3 endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}

	s3Cfg := S3CompatibleConfig{
		Provider:     "s3",
		Endpoint:     cfg.Endpoint,
		Bucket:       cfg.Bucket,
		Region:       cfg.Region,
		AccessKey:    cfg.AccessKeyID,
		SecretKey:    cfg.SecretAccessKey,
		CDNHost:      cfg.CDNHost,
		UsePathStyle: true,
	}

	if s3Cfg.CDNHost == "" {
		s3Cfg.CDNHost = f.staticCDN
	}

	return NewS3CompatibleProvider(s3Cfg), nil
}

// newOSSProvider creates an Aliyun OSS provider.
func (f *Factory) newOSSProvider(cfg OSSConfig) (*S3CompatibleProvider, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("oss endpoint is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("oss bucket is required")
	}

	s3Cfg := S3CompatibleConfig{
		Provider:     "oss",
		Endpoint:     cfg.Endpoint,
		Bucket:       cfg.Bucket,
		AccessKey:    cfg.AccessKeyID,
		SecretKey:    cfg.SecretKey,
		CDNHost:      cfg.CDNHost,
		UsePathStyle: true,
	}

	if s3Cfg.CDNHost == "" {
		s3Cfg.CDNHost = f.staticCDN
	}

	return NewS3CompatibleProvider(s3Cfg), nil
}

// newCOSProvider creates a Tencent COS provider (virtual-hosted-style).
func (f *Factory) newCOSProvider(cfg COSConfig) (*S3CompatibleProvider, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("cos bucket is required")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("cos region is required")
	}

	bucketName := resolveCOSBucketName(cfg.Bucket, cfg.AppID)
	if bucketName == "" {
		return nil, fmt.Errorf("invalid cos bucket/appid configuration")
	}
	endpoint := fmt.Sprintf("https://cos.%s.myqcloud.com", cfg.Region)

	cdnHost := cfg.CDNHost
	if cdnHost == "" {
		cdnHost = fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucketName, cfg.Region)
	}
	if cdnHost == "" {
		cdnHost = f.staticCDN
	}

	return NewS3CompatibleProvider(S3CompatibleConfig{
		Provider:          "cos",
		Endpoint:          endpoint,
		Bucket:            bucketName,
		AccessKey:         cfg.SecretID,
		SecretKey:         cfg.SecretKey,
		Region:            cfg.Region,
		CDNHost:           cdnHost,
		UsePathStyle:      false, // COS uses virtual-hosted-style
		HostnameImmutable: false,
	}), nil
}

func resolveCOSBucketName(bucket, appID string) string {
	bucket = strings.TrimSpace(bucket)
	appID = strings.TrimSpace(appID)
	if bucket == "" {
		return ""
	}
	if strings.Contains(bucket, "-") {
		return bucket
	}
	if appID == "" {
		return bucket
	}
	return fmt.Sprintf("%s-%s", bucket, appID)
}

// DefaultConfig returns the default local storage configuration.
func DefaultConfig(workDir, staticHost string) Config {
	return Config{
		Type: StorageTypeLocal,
		Local: LocalConfig{
			BaseDir: filepath.Join(workDir, "web", "static", "uploads"),
			BaseURL: fmt.Sprintf("%s/static/uploads", staticHost),
		},
	}
}
