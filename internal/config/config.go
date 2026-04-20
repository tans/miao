package config

import (
	"os"
	"time"
)

type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	JWT        JWTConfig
	WechatMini WechatMiniConfig
	Admin      AdminConfig
	Static     StaticConfig
	Storage    StorageConfig
}

type StaticConfig struct {
	Host      string // 静态资源主域名
	CDN       string // CDN域名（可与Host相同）
	Provider  string // 存储提供商: "local" | "rustfs" | "s3" | "oss" | "cos"
}

type StorageConfig struct {
	Provider string // 存储提供商: "local" | "rustfs" | "s3" | "oss" | "cos"
	RustFS   RustFSConfig
	S3       S3Config
	OSS      OSSConfig
	COS      COSConfig
}

type RustFSConfig struct {
	Endpoint  string // rustfs API 地址
	Bucket    string
	AccessKey string
	SecretKey string
	Region    string
}

type S3Config struct {
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
}

type OSSConfig struct {
	Endpoint   string
	Bucket     string
	AccessKey  string // AccessKeyID for consistency with other providers
	SecretKey  string
	Region     string
	CDNHost    string
}

type COSConfig struct {
	AppID      string
	Bucket     string
	Region     string
	SecretKey  string
	SecretID   string
	CDNHost    string
}

type WechatMiniConfig struct {
	AppID     string
	AppSecret string
}

type AdminConfig struct {
	Username string
	Password string
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Path string
}

type JWTConfig struct {
	Secret         string
	ExpireTime     time.Duration
	AdminExpireTime time.Duration
}

func Load() *Config {
	// JWT Secret: uses JWT_SECRET env var if set, otherwise falls back to fixed default
	// IMPORTANT: For production, always set JWT_SECRET env var explicitly
	jwtSecret := getEnv("JWT_SECRET", "miaoplatform-prod-secret-2024")

	// Admin JWT: 30 days, regular user JWT: 7 days
	adminExpireTime := getEnvDuration("JWT_ADMIN_EXPIRE_TIME", 30*24*time.Hour)
	if adminExpireTime == 0 {
		adminExpireTime = 30 * 24 * time.Hour
	}

	return &Config{
		Server: ServerConfig{
			Port:         getEnv("SERVER_PORT", "8888"),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		Database: DatabaseConfig{
			Path: getEnv("DB_PATH", "./data/miao.db"),
		},
		JWT: JWTConfig{
			Secret:          jwtSecret,
			ExpireTime:      24 * time.Hour * 7,
			AdminExpireTime: adminExpireTime,
		},
		WechatMini: WechatMiniConfig{
			AppID:     getEnv("WECHAT_MINI_APPID", ""),
			AppSecret: getEnv("WECHAT_MINI_APPSECRET", ""),
		},
		Admin: AdminConfig{
			Username: getEnv("ADMIN_USERNAME", "admin"),
			Password: getEnv("ADMIN_PASSWORD", ""),
		},
		Static: StaticConfig{
			Host:     getEnv("STATIC_HOST", "https://miao-test.clawos.cc"),
			CDN:      getEnv("STATIC_CDN", "https://miao-test.clawos.cc"),
			Provider: getEnv("STORAGE_PROVIDER", "local"),
		},
		Storage: StorageConfig{
			Provider: getEnv("STORAGE_PROVIDER", "local"),
			RustFS: RustFSConfig{
				Endpoint:  getEnv("RUSTFS_ENDPOINT", ""),
				Bucket:    getEnv("RUSTFS_BUCKET", ""),
				AccessKey: getEnv("RUSTFS_ACCESS_KEY", ""),
				SecretKey: getEnv("RUSTFS_SECRET_KEY", ""),
				Region:    getEnv("RUSTFS_REGION", "us-east-1"),
			},
			S3: S3Config{
				Endpoint:        getEnv("S3_ENDPOINT", ""),
				Bucket:          getEnv("S3_BUCKET", ""),
				Region:          getEnv("S3_REGION", "us-east-1"),
				AccessKeyID:     getEnv("S3_ACCESS_KEY_ID", ""),
				SecretAccessKey: getEnv("S3_SECRET_ACCESS_KEY", ""),
			},
			OSS: OSSConfig{
				Endpoint:  getEnv("OSS_ENDPOINT", ""),
				Bucket:    getEnv("OSS_BUCKET", ""),
				AccessKey: getEnv("OSS_ACCESS_KEY", ""),
				SecretKey: getEnv("OSS_SECRET_KEY", ""),
				Region:    getEnv("OSS_REGION", ""),
				CDNHost:   getEnv("OSS_CDN_HOST", ""),
			},
			COS: COSConfig{
				AppID:     getEnv("COS_APP_ID", ""),
				Bucket:    getEnv("COS_BUCKET", ""),
				Region:    getEnv("COS_REGION", ""),
				SecretKey: getEnv("COS_SECRET_KEY", ""),
				SecretID:  getEnv("COS_SECRET_ID", ""),
				CDNHost:   getEnv("COS_CDN_HOST", ""),
			},
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
