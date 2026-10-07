package httpapi

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/jev"
	"github.com/tans/miao/internal/pocketbase"
)

const vercelGateway = "https://ai-gateway.vercel.sh"

type aiConfig struct {
	Key, Provider, BaseURL, Model, Source string
	Enabled                               bool
}
type jevConfig struct {
	Key, Provider, Model, Source string
	Enabled, Inherited           bool
}

type aiProviderSettings struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
}

type jevProviderSettings struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func (s *Server) readAdminSecret(ctx context.Context, name string) (string, error) {
	saved, err := s.PB.Find(ctx, "platform_settings", "name = "+pbFilterString(name))
	if err != nil {
		var pbErr *pocketbase.Error
		if errors.As(err, &pbErr) && pbErr.Status == 404 {
			return "", nil
		}
		return "", err
	}
	secret := os.Getenv("MIAO_SETTINGS_ENCRYPTION_KEY")
	if len(secret) < 32 {
		return "", fmt.Errorf("MIAO_SETTINGS_ENCRYPTION_KEY must contain at least 32 characters")
	}
	key := sha256.Sum256([]byte(secret))
	parts := strings.Split(stringValue(saved["value"]), ".")
	if len(parts) != 4 || parts[0] != "v1" {
		return "", fmt.Errorf("stored AI key has unsupported format")
	}
	iv, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	tag, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(iv) != gcm.NonceSize() || len(tag) != gcm.Overhead() {
		return "", fmt.Errorf("stored AI key is invalid")
	}
	plain, err := gcm.Open(nil, iv, append(ciphertext, tag...), nil)
	return string(plain), err
}

func readJSONSetting[T any](ctx context.Context, pb *pocketbase.Client, name string, target *T) error {
	row, err := pb.Find(ctx, "platform_settings", "name = "+pbFilterString(name))
	if err != nil {
		var pbErr *pocketbase.Error
		if errors.As(err, &pbErr) && pbErr.Status == 404 {
			return nil
		}
		return err
	}
	return json.Unmarshal([]byte(stringValue(row["value"])), target)
}

func (s *Server) readLLMProviderSettings(ctx context.Context) (aiProviderSettings, error) {
	settings := aiProviderSettings{Enabled: true, Provider: env("MIAO_AI_PROVIDER", "vercel"), BaseURL: env("MIAO_AI_BASE_URL", "http://127.0.0.1:3210/api/v1"), Model: env("MIAO_AI_MODEL", "gpt-5.2")}
	if err := readJSONSetting(ctx, s.PB, "ai_llm_config", &settings); err != nil {
		return settings, err
	}
	if settings.Provider != "vercel" && settings.Provider != "capi" {
		return settings, fmt.Errorf("unsupported LLM provider")
	}
	if settings.Model == "" {
		return settings, fmt.Errorf("LLM model is empty")
	}
	if settings.Provider == "capi" && !validAIBaseURL(settings.BaseURL) {
		return settings, fmt.Errorf("invalid LLM base URL")
	}
	return settings, nil
}

func (s *Server) readJevProviderSettings(ctx context.Context) (jevProviderSettings, error) {
	settings := jevProviderSettings{Enabled: true, Provider: env("MIAO_JEV_PROVIDER", jev.ProviderVercel), Model: env("MIAO_JEV_MODEL", "")}
	if err := readJSONSetting(ctx, s.PB, "jev_config", &settings); err != nil {
		return settings, err
	}
	if settings.Provider == "" {
		settings.Provider = jev.ProviderVercel
	}
	if settings.Provider != jev.ProviderVercel && settings.Provider != jev.ProviderTypesafe {
		return settings, fmt.Errorf("unsupported Jev provider")
	}
	if settings.Model == "" {
		settings.Model = jevDefaultModel
		if settings.Provider == jev.ProviderTypesafe {
			settings.Model = jevOfficialDefaultModel
		}
	}
	return settings, nil
}

func (s *Server) readAIConfig(ctx context.Context) (aiConfig, error) {
	settings, err := s.readLLMProviderSettings(ctx)
	if err != nil {
		return aiConfig{}, err
	}
	cfg := aiConfig{Key: os.Getenv("AI_GATEWAY_API_KEY"), Provider: settings.Provider, BaseURL: settings.BaseURL, Model: settings.Model, Source: "none", Enabled: settings.Enabled}
	if cfg.Key != "" {
		cfg.Source = "environment"
	}
	if saved, e := s.readAdminSecret(ctx, "ai_gateway_api_key"); e != nil {
		return cfg, e
	} else if saved != "" {
		cfg.Key = saved
		cfg.Source = "admin"
	}
	return cfg, nil
}

func (s *Server) readJevConfig(ctx context.Context) (jevConfig, error) {
	settings, err := s.readJevProviderSettings(ctx)
	if err != nil {
		return jevConfig{}, err
	}
	cfg := jevConfig{Provider: settings.Provider, Model: settings.Model, Enabled: settings.Enabled, Key: strings.TrimSpace(env("MIAO_JEV_API_KEY", "")), Source: "none"}
	if cfg.Key != "" {
		cfg.Source = "environment"
	}
	if saved, e := s.readAdminSecret(ctx, "jev_api_key"); e != nil {
		return cfg, e
	} else if saved != "" {
		cfg.Key, cfg.Source = saved, "admin"
	}
	// Only Vercel Gateway credentials can be shared with the LLM service;
	// CAPI and official Typesafe keys must never be reused across providers.
	if cfg.Key == "" && cfg.Provider == jev.ProviderVercel {
		llm, e := s.readAIConfig(ctx)
		if e != nil {
			return cfg, e
		}
		if llm.Provider == "vercel" && llm.Key != "" {
			cfg.Key, cfg.Source, cfg.Inherited = llm.Key, "llm", true
		}
	}
	return cfg, nil
}

func (s *Server) aiRateLimited(ctx context.Context, tenantID, userID string) bool {
	return aiRateLimitedWith(ctx, s.PB, tenantID, userID, "llm")
}

func aiRateLimitedWith(ctx context.Context, pb *pocketbase.Client, tenantID, userID, kind string) bool {
	start := time.Now().Add(-time.Minute).UTC().Format("2006-01-02 15:04:05.000Z")
	kindFilter := " && kind = " + pbFilterString(kind)
	_, tenantCount, _, e1 := pb.List(ctx, "ai_usage", "tenant_id = "+pbFilterString(tenantID)+kindFilter+" && created >= "+pbFilterString(start), "", 1, 1)
	_, userCount, _, e2 := pb.List(ctx, "ai_usage", "tenant_id = "+pbFilterString(tenantID)+" && user_id = "+pbFilterString(userID)+kindFilter+" && created >= "+pbFilterString(start), "", 1, 1)
	_, globalCount, _, e3 := pb.List(ctx, "ai_usage", "created >= "+pbFilterString(start), "", 1, 1)
	return e1 != nil || e2 != nil || e3 != nil || tenantCount >= 120 || userCount >= 30 || globalCount >= 500
}

func (s *Server) checkAIQuota(ctx context.Context, tenantID, userID string) error {
	return checkAIQuotaWith(ctx, s.PB, tenantID, userID, "llm")
}

func checkAIQuotaWith(ctx context.Context, pb *pocketbase.Client, tenantID, userID, kind string) error {
	tenant, err := pb.Get(ctx, "tenants", tenantID)
	if err != nil {
		return err
	}
	limit := intValue(tenant["ai_daily_limit"])
	if limit > 0 {
		start := time.Now().UTC()
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		_, count, _, err := pb.List(ctx, "ai_usage", "tenant_id = "+pbFilterString(tenantID)+" && created >= "+pbFilterString(start.Format("2006-01-02 15:04:05.000Z")), "", 1, 1)
		if err != nil {
			return err
		}
		if count >= limit {
			return fmt.Errorf("工作区已达到今日 AI 请求预算")
		}
	}
	field := "ai_llm_daily_limit"
	if kind == "jev" {
		field = "ai_jev_daily_limit"
	}
	kindLimit := intValue(tenant[field])
	if kindLimit > 0 {
		start := time.Now().UTC()
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
		_, count, _, err := pb.List(ctx, "ai_usage", "tenant_id = "+pbFilterString(tenantID)+" && kind = "+pbFilterString(kind)+" && created >= "+pbFilterString(start.Format("2006-01-02 15:04:05.000Z")), "", 1, 1)
		if err != nil {
			return err
		}
		if count >= kindLimit {
			return fmt.Errorf("工作区已达到今日 %s 请求预算", map[string]string{"llm": "LLM", "jev": "JEV"}[kind])
		}
	}
	if aiRateLimitedWith(ctx, pb, tenantID, userID, kind) {
		return fmt.Errorf("AI 请求次数过多，请稍后重试")
	}
	return nil
}

func (s *Server) reserveAIUsage(ctx context.Context, tenantID, userID, appID, kind, provider, model string) (map[string]any, error) {
	var usage map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		if err := checkAIQuotaWith(ctx, tx, tenantID, userID, kind); err != nil {
			return err
		}
		var err error
		usage, err = tx.Create(ctx, "ai_usage", map[string]any{"tenant_id": tenantID, "user_id": userID, "app_id": appID, "kind": kind, "provider": provider, "model": model, "status": 100, "input_tokens": 0, "output_tokens": 0, "input_known": false, "output_known": false})
		return err
	})
	return usage, err
}

func tokenUsage(value any) (int, int, bool, bool) {
	if m, ok := value.(map[string]any); ok {
		if u, ok := m["usage"].(map[string]any); ok {
			read := func(names ...string) (int, bool) {
				for _, name := range names {
					if value, ok := u[name].(float64); ok && value >= 0 && value <= 1e12 && value == float64(int64(value)) {
						return int(value), true
					}
				}
				return 0, false
			}
			a, ak := read("inputTokens", "promptTokens", "prompt_tokens")
			b, bk := read("outputTokens", "completionTokens", "completion_tokens")
			return a, b, ak, bk
		}
		for _, child := range m {
			if a, b, ak, bk := tokenUsage(child); ak || bk {
				return a, b, ak, bk
			}
		}
	}
	if items, ok := value.([]any); ok {
		for _, child := range items {
			if a, b, ak, bk := tokenUsage(child); ak || bk {
				return a, b, ak, bk
			}
		}
	}
	return 0, 0, false, false
}

func (s *Server) callAI(ctx context.Context, tenantID, userID, appID string, body map[string]any) (result map[string]any, tokens [2]int, callErr error) {
	config, err := s.readAIConfig(ctx)
	if err != nil {
		return nil, [2]int{}, err
	}
	if !config.Enabled {
		return nil, [2]int{}, fmt.Errorf("LLM 服务已停用")
	}
	if config.Key == "" {
		return nil, [2]int{}, fmt.Errorf("企业尚未配置 AI 服务密钥")
	}
	if err := harness.ReserveModelRequest(ctx); err != nil {
		return nil, [2]int{}, err
	}
	usage, err := s.reserveAIUsage(ctx, tenantID, userID, appID, "llm", config.Provider, config.Model)
	if err != nil {
		return nil, [2]int{}, err
	}
	start := time.Now()
	status := 502
	inputKnown, outputKnown := false, false
	defer func() {
		recordCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := s.PB.Update(recordCtx, "ai_usage", stringValue(usage["id"]), map[string]any{"status": status, "latency_ms": time.Since(start).Milliseconds(), "input_tokens": tokens[0], "output_tokens": tokens[1], "input_known": inputKnown, "output_known": outputKnown})
		if err != nil {
			callErr = fmt.Errorf("LLM 用量回执保存失败")
		}
	}()
	body["model"] = config.Model
	body["stream"] = false
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, [2]int{}, err
	}
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
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return nil, [2]int{}, err
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if readErr != nil || len(data) > 8<<20 {
		return nil, [2]int{}, fmt.Errorf("AI gateway response unavailable or too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		status = resp.StatusCode
		return nil, [2]int{}, fmt.Errorf("AI gateway returned HTTP %d", resp.StatusCode)
	}
	result = map[string]any{}
	if json.Unmarshal(data, &result) != nil || len(asSliceMap(result["choices"])) == 0 {
		return nil, [2]int{}, fmt.Errorf("AI gateway response is invalid")
	}
	a, b, ak, bk := tokenUsage(result)
	inputKnown, outputKnown = ak, bk
	status = resp.StatusCode
	return result, [2]int{a, b}, nil
}
