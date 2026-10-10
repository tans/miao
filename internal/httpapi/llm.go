package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/settings"
)

// callLLM is a proposal-only transport. Callers must parse and validate the
// returned JSON before it can become a Harness candidate.
func (s *Server) callLLM(ctx context.Context, tenantID, userID, appID string, body map[string]any) (result map[string]any, tokens [2]int, callErr error) {
	config, err := settings.ReadLLMConfig(ctx, s.PB)
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
		_, updateErr := s.PB.Update(recordCtx, "ai_usage", stringValue(usage["id"]), map[string]any{
			"status": status, "latency_ms": time.Since(start).Milliseconds(),
			"input_tokens": tokens[0], "output_tokens": tokens[1],
			"input_known": inputKnown, "output_known": outputKnown,
		})
		if updateErr != nil {
			callErr = fmt.Errorf("LLM 用量回执保存失败")
		}
	}()

	body["model"] = config.Model
	body["stream"] = false
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, [2]int{}, err
	}
	target := settings.GatewayBase + "/v1/chat/completions"
	if config.Provider == "capi" {
		target = strings.TrimRight(config.BaseURL, "/") + "/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return nil, [2]int{}, err
	}
	req.Header.Set("Authorization", "Bearer "+config.Key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		s.Logger.Warn("llm call failed", "error", err)
		var netErr net.Error
		if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
			return nil, [2]int{}, businessError(504, "生成模型响应超时，请稍后重试")
		}
		return nil, [2]int{}, businessError(502, "生成模型暂时无法访问，请稍后重试")
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
	if a, b, ak, bk := llmTokenUsage(result); ak || bk {
		tokens = [2]int{a, b}
		inputKnown, outputKnown = ak, bk
	}
	status = resp.StatusCode
	return result, tokens, nil
}

func llmTokenUsage(value any) (int, int, bool, bool) {
	if m, ok := value.(map[string]any); ok {
		if usage, ok := m["usage"].(map[string]any); ok {
			read := func(names ...string) (int, bool) {
				for _, name := range names {
					if n, ok := usage[name].(float64); ok && n >= 0 && n <= 1e12 && n == float64(int64(n)) {
						return int(n), true
					}
				}
				return 0, false
			}
			a, ak := read("inputTokens", "promptTokens", "prompt_tokens")
			b, bk := read("outputTokens", "completionTokens", "completion_tokens")
			return a, b, ak, bk
		}
		for _, child := range m {
			if a, b, ak, bk := llmTokenUsage(child); ak || bk {
				return a, b, ak, bk
			}
		}
	}
	if items, ok := value.([]any); ok {
		for _, child := range items {
			if a, b, ak, bk := llmTokenUsage(child); ak || bk {
				return a, b, ak, bk
			}
		}
	}
	return 0, 0, false, false
}
