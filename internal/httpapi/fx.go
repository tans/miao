package httpapi

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

const vercelGateway = "https://ai-gateway.vercel.sh"

type aiConfig struct{ Key, Provider, BaseURL, Model, Source string }

func (s *Server) readAIConfig(ctx context.Context) (aiConfig, error) {
	provider := "vercel"
	if os.Getenv("MIAO_AI_PROVIDER") == "capi" {
		provider = "capi"
	}
	cfg := aiConfig{Key: os.Getenv("AI_GATEWAY_API_KEY"), Provider: provider, BaseURL: env("MIAO_AI_BASE_URL", "http://127.0.0.1:3210/api/v1"), Model: env("MIAO_AI_MODEL", "gpt-5.2"), Source: "none"}
	if cfg.Key != "" {
		cfg.Source = "environment"
	}
	if saved, err := s.PB.Find(ctx, "platform_settings", "name = "+pbFilterString("ai_gateway_api_key")); err == nil {
		secret := os.Getenv("MIAO_SETTINGS_ENCRYPTION_KEY")
		if len(secret) < 32 {
			return cfg, fmt.Errorf("MIAO_SETTINGS_ENCRYPTION_KEY must contain at least 32 characters")
		}
		key := sha256.Sum256([]byte(secret))
		parts := strings.Split(stringValue(saved["value"]), ".")
		if len(parts) != 4 || parts[0] != "v1" {
			return cfg, fmt.Errorf("stored AI key has unsupported format")
		}
		iv, e := base64.RawURLEncoding.DecodeString(parts[1])
		if e != nil {
			return cfg, e
		}
		tag, e := base64.RawURLEncoding.DecodeString(parts[2])
		if e != nil {
			return cfg, e
		}
		ciphertext, e := base64.RawURLEncoding.DecodeString(parts[3])
		if e != nil {
			return cfg, e
		}
		block, e := aes.NewCipher(key[:])
		if e != nil {
			return cfg, e
		}
		gcm, e := cipher.NewGCM(block)
		if e != nil {
			return cfg, e
		}
		if len(tag) != gcm.Overhead() {
			return cfg, fmt.Errorf("stored AI key is invalid")
		}
		payload := append(ciphertext, tag...)
		plain, e := gcm.Open(nil, iv, payload, nil)
		if e != nil {
			return cfg, e
		}
		cfg.Key = string(plain)
		cfg.Source = "admin"
	}
	return cfg, nil
}

func (s *Server) aiRateLimited(ctx context.Context, tenantID, userID string) bool {
	return aiRateLimitedWith(ctx, s.PB, tenantID, userID)
}

func aiRateLimitedWith(ctx context.Context, pb *pocketbase.Client, tenantID, userID string) bool {
	start := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	_, tenantCount, _, e1 := pb.List(ctx, "ai_usage", "tenant_id = "+pbFilterString(tenantID)+" && created >= "+pbFilterString(start), "", 1, 1)
	_, userCount, _, e2 := pb.List(ctx, "ai_usage", "tenant_id = "+pbFilterString(tenantID)+" && user_id = "+pbFilterString(userID)+" && created >= "+pbFilterString(start), "", 1, 1)
	_, globalCount, _, e3 := pb.List(ctx, "ai_usage", "created >= "+pbFilterString(start), "", 1, 1)
	return e1 != nil || e2 != nil || e3 != nil || tenantCount >= 120 || userCount >= 30 || globalCount >= 500
}

func (s *Server) checkAIQuota(ctx context.Context, tenantID, userID string) error {
	return checkAIQuotaWith(ctx, s.PB, tenantID, userID)
}

func checkAIQuotaWith(ctx context.Context, pb *pocketbase.Client, tenantID, userID string) error {
	tenant, err := pb.Get(ctx, "tenants", tenantID)
	if err != nil {
		return err
	}
	limit := intValue(tenant["ai_daily_limit"])
	if limit > 0 {
		start := time.Now().UTC()
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		_, count, _, err := pb.List(ctx, "ai_usage", "tenant_id = "+pbFilterString(tenantID)+" && created >= "+pbFilterString(start.Format(time.RFC3339Nano)), "", 1, 1)
		if err != nil {
			return err
		}
		if count >= limit {
			return fmt.Errorf("工作区已达到今日 AI 请求预算")
		}
	}
	if aiRateLimitedWith(ctx, pb, tenantID, userID) {
		return fmt.Errorf("AI 请求次数过多，请稍后重试")
	}
	return nil
}

func (s *Server) reserveAIUsage(ctx context.Context, tenantID, userID, appID string) (map[string]any, error) {
	var usage map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		if err := checkAIQuotaWith(ctx, tx, tenantID, userID); err != nil {
			return err
		}
		var err error
		usage, err = tx.Create(ctx, "ai_usage", map[string]any{"tenant_id": tenantID, "user_id": userID, "app_id": appID, "status": 100, "input_tokens": 0, "output_tokens": 0})
		return err
	})
	return usage, err
}

func defaultAny(value, fallback any) any {
	if value != nil {
		return value
	}
	return fallback
}
func findUsage(value any) (int, int) {
	if items, ok := value.([]any); ok {
		for _, child := range items {
			if a, b := findUsage(child); a+b > 0 {
				return a, b
			}
		}
		return 0, 0
	}
	m, ok := value.(map[string]any)
	if !ok {
		return 0, 0
	}
	if u, ok := m["usage"].(map[string]any); ok {
		a := intValue(defaultAny(u["inputTokens"], defaultAny(u["promptTokens"], defaultAny(u["prompt_tokens"], 0))))
		b := intValue(defaultAny(u["outputTokens"], defaultAny(u["completionTokens"], defaultAny(u["completion_tokens"], 0))))
		if a+b > 0 {
			return a, b
		}
	}
	for _, child := range m {
		if a, b := findUsage(child); a+b > 0 {
			return a, b
		}
	}
	return 0, 0
}

func (s *Server) callAI(ctx context.Context, tenantID, userID, appID string, body map[string]any) (map[string]any, [2]int, error) {
	config, err := s.readAIConfig(ctx)
	if err != nil {
		return nil, [2]int{}, err
	}
	if config.Key == "" {
		return nil, [2]int{}, fmt.Errorf("企业尚未配置 AI 服务密钥")
	}
	usage, err := s.reserveAIUsage(ctx, tenantID, userID, appID)
	if err != nil {
		return nil, [2]int{}, err
	}
	body["model"] = config.Model
	body["stream"] = false
	encoded, _ := json.Marshal(body)
	target := vercelGateway + "/v1/chat/completions"
	if config.Provider == "capi" {
		target = strings.TrimRight(config.BaseURL, "/") + "/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(encoded))
	if err != nil {
		return nil, [2]int{}, err
	}
	req.Header.Set("Authorization", "Bearer "+config.Key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_, _ = s.PB.Update(context.Background(), "ai_usage", stringValue(usage["id"]), map[string]any{"status": 502})
		return nil, [2]int{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	_, _ = s.PB.Update(context.Background(), "ai_usage", stringValue(usage["id"]), map[string]any{"status": resp.StatusCode})
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, [2]int{}, fmt.Errorf("AI gateway returned %d: %s", resp.StatusCode, clip(string(data), 500))
	}
	result := map[string]any{}
	if json.Unmarshal(data, &result) != nil {
		return nil, [2]int{}, fmt.Errorf("AI gateway response is invalid")
	}
	a, b := findUsage(result)
	if a+b > 0 {
		_, _ = s.PB.Update(context.Background(), "ai_usage", stringValue(usage["id"]), map[string]any{"input_tokens": a, "output_tokens": b})
	}
	return result, [2]int{a, b}, nil
}
