package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
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
	VideoProcessing VideoProcessingConfig
	Commission CommissionConfig
	RateLimit  RateLimitConfig
	Stats      StatsConfig
	Margin     MarginConfig
}

type VideoProcessingConfig struct {
	Enabled           bool
	ServiceURL        string
	CallbackBaseURL   string
	CallbackSecret    string
	WatermarkTemplate string
	TargetFormat      string
	TargetResolution  string
}

type MarginConfig struct {
	Amount float64 // 保证金金额
}

type StatsConfig struct {
	DefaultPeriod string // 默认统计周期: "7d", "30d", "90d", "180d", "365d"
	Periods       []int  // 可选的统计周期天数列表
}

type RateLimitConfig struct {
	DefaultLimit     int           // 默认限流：每窗口请求数
	DefaultWindow    time.Duration // 默认限流：时间窗口
	StrictLimit      int           // 严格限流：每窗口请求数
	StrictWindow     time.Duration // 严格限流：时间窗口
}

type CommissionConfig struct {
	DiamondRate float64 // 钻石创作者抽成
	GoldRate    float64 // 黄金创作者抽成
	SilverRate  float64 // 白银创作者抽成
	BronzeRate  float64 // 青铜创作者抽成
}

type StaticConfig struct {
	Host           string // 静态资源主域名
	CDN            string // CDN域名（可与Host相同）
	Provider       string // 存储提供商: "local" | "rustfs" | "s3" | "oss" | "cos"
	DefaultNickname string // 默认昵称
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
	Port               string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	CORSAllowedOrigins string // 允许的CORS来源，多个用逗号分隔，空则使用 *
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
	// JWT Secret: MUST be set explicitly via environment variable
	rawJWTSecret := os.Getenv("JWT_SECRET")
	if rawJWTSecret == "" {
		if os.Getenv("GIN_MODE") == "release" {
			log.Fatal(fmt.Sprintf("[FATAL] JWT_SECRET environment variable is not set. " +
				"Production mode requires a secure, unique JWT_SECRET to be set via environment variable. " +
				"Refusing to start without a secret."))
		} else {
			log.Fatalf("[FATAL] JWT_SECRET environment variable is not set. " +
				"MUST be configured even in development environments.")
		}
	}

	// Admin JWT: 30 days, regular user JWT: 7 days
	adminExpireTime := getEnvDuration("JWT_ADMIN_EXPIRE_TIME", 30*24*time.Hour)
	if adminExpireTime == 0 {
		adminExpireTime = 30 * 24 * time.Hour
	}

	return &Config{
		Server: ServerConfig{
			Port:               getEnv("SERVER_PORT", "8888"),
			ReadTimeout:        30 * time.Second,
			WriteTimeout:       30 * time.Second,
			CORSAllowedOrigins: getEnv("CORS_ALLOWED_ORIGINS", ""),
		},
		Database: DatabaseConfig{
			Path: getEnv("DB_PATH", "./data/miao.db"),
		},
		JWT: JWTConfig{
			Secret:          rawJWTSecret,
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
			Host:            getEnv("STATIC_HOST", "https://miao-test.clawos.cc"),
			CDN:             getEnv("STATIC_CDN", "https://miao-test.clawos.cc"),
			Provider:        getEnv("STORAGE_PROVIDER", "local"),
			DefaultNickname: getEnv("DEFAULT_NICKNAME", "喵喵"),
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
		VideoProcessing: VideoProcessingConfig{
			Enabled:           getEnvBool("VIDEO_PROCESSING_ENABLED", true),
			ServiceURL:        getEnv("VIDEO_PROCESSING_SERVICE_URL", "http://127.0.0.1:9096"),
			CallbackBaseURL:   getEnv("VIDEO_PROCESSING_CALLBACK_BASE_URL", "http://127.0.0.1:8888"),
			CallbackSecret:    getEnv("VIDEO_PROCESSING_CALLBACK_SECRET", ""),
			WatermarkTemplate: getEnv("VIDEO_PROCESSING_WATERMARK_TEMPLATE", "miao-default"),
			TargetFormat:      getEnv("VIDEO_PROCESSING_TARGET_FORMAT", "mp4"),
			TargetResolution:  getEnv("VIDEO_PROCESSING_TARGET_RESOLUTION", "1080P"),
		},
		Commission: CommissionConfig{
			DiamondRate: getEnvFloat("COMMISSION_DIAMOND_RATE", 0.10),
			GoldRate:    getEnvFloat("COMMISSION_GOLD_RATE", 0.12),
			SilverRate:  getEnvFloat("COMMISSION_SILVER_RATE", 0.15),
			BronzeRate:  getEnvFloat("COMMISSION_BRONZE_RATE", 0.20),
		},
		RateLimit: RateLimitConfig{
			DefaultLimit:  getEnvInt("RATELIMIT_DEFAULT_LIMIT", 100),
			DefaultWindow: getEnvDuration("RATELIMIT_DEFAULT_WINDOW", time.Minute),
			StrictLimit:   getEnvInt("RATELIMIT_STRICT_LIMIT", 20),
			StrictWindow:  getEnvDuration("RATELIMIT_STRICT_WINDOW", time.Minute),
		},
		Stats: StatsConfig{
			DefaultPeriod: getEnv("STATS_DEFAULT_PERIOD", "7d"),
			Periods:       []int{7, 30, 90, 180, 365},
		},
		Margin: MarginConfig{
			Amount: getEnvFloat("MARGIN_AMOUNT", 10.0),
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

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch value {
		case "1", "true", "TRUE", "yes", "YES", "on", "ON":
			return true
		case "0", "false", "FALSE", "no", "NO", "off", "OFF":
			return false
		}
	}
	return defaultValue
}
