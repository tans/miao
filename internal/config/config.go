package config

import (
	"os"
	"time"
)

type Config struct {
	Server      ServerConfig
	Database    DatabaseConfig
	JWT         JWTConfig
	WechatMini  WechatMiniConfig
	Admin      AdminConfig
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
	Secret     string
	ExpireTime time.Duration
}

func Load() *Config {
	// JWT Secret must be set in production
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		// Only use default for development
		jwtSecret = "miaoplatform-dev-secret-2024"
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
			Secret:     jwtSecret,
			ExpireTime: 24 * time.Hour * 7,
		},
		WechatMini: WechatMiniConfig{
			AppID:     getEnv("WECHAT_MINI_APPID", ""),
			AppSecret: getEnv("WECHAT_MINI_APPSECRET", ""),
		},
		Admin: AdminConfig{
			Username: getEnv("ADMIN_USERNAME", "admin"),
			Password: getEnv("ADMIN_PASSWORD", ""),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
