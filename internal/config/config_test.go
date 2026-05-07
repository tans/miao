package config

import "testing"

func TestValidateWechatMiniConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := &Config{
			WechatMini: WechatMiniConfig{
				AppID:     "wx123",
				AppSecret: "secret123",
			},
		}

		if err := ValidateWechatMiniConfig(cfg); err != nil {
			t.Fatalf("expected valid config, got error: %v", err)
		}
	})

	t.Run("missing app id", func(t *testing.T) {
		cfg := &Config{
			WechatMini: WechatMiniConfig{
				AppSecret: "secret123",
			},
		}

		if err := ValidateWechatMiniConfig(cfg); err == nil {
			t.Fatal("expected missing app id error, got nil")
		}
	})

	t.Run("missing app secret", func(t *testing.T) {
		cfg := &Config{
			WechatMini: WechatMiniConfig{
				AppID: "wx123",
			},
		}

		if err := ValidateWechatMiniConfig(cfg); err == nil {
			t.Fatal("expected missing app secret error, got nil")
		}
	})

	t.Run("missing both", func(t *testing.T) {
		cfg := &Config{}

		if err := ValidateWechatMiniConfig(cfg); err == nil {
			t.Fatal("expected missing wechat config error, got nil")
		}
	})
}
