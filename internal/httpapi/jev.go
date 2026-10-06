package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Jev is the experimental json-render evaluator contract pinned to upstream
// commit fc2a696a50a30cb30c878ab1eb65e102487eea0f. The upstream JS adapter
// posts this exact v4 evaluation request; Go owns credentials and validation.
const jevUpstreamCommit = "fc2a696a50a30cb30c878ab1eb65e102487eea0f"

const jevDefaultModel = "typesafe-ai/jev"
const jevEvaluationEndpoint = "https://ai-gateway.vercel.sh/v4/ai/evaluation-model"

// Kept as a variable so the protocol can be tested against an isolated server.
var jevEvaluationURL = jevEvaluationEndpoint

type jevQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

type jevAnswer struct {
	Choice     string
	Confidence float64
	HasConf    bool
}

func (s *Server) evaluateJev(ctx context.Context, tenantID, userID, appID string, state map[string]any, questions map[string]jevQuestion) (map[string]jevAnswer, error) {
	config, err := s.readAIConfig(ctx)
	if err != nil {
		return nil, err
	}
	if config.Key == "" {
		return nil, fmt.Errorf("企业尚未配置 AI 服务密钥")
	}
	usage, err := s.reserveAIUsage(ctx, tenantID, userID, appID)
	if err != nil {
		return nil, err
	}
	markUsage := func(status int, inputTokens int) {
		updates := map[string]any{"status": status}
		if inputTokens > 0 {
			updates["input_tokens"] = inputTokens
		}
		_, _ = s.PB.Update(context.Background(), "ai_usage", stringValue(usage["id"]), updates)
	}
	payload, err := json.Marshal(map[string]any{"state": state, "questions": questions})
	if err != nil {
		markUsage(400, 0)
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, jevEvaluationURL, bytes.NewReader(payload))
	if err != nil {
		markUsage(400, 0)
		return nil, err
	}
	model := defaultString(strings.TrimSpace(env("MIAO_JEV_MODEL", jevDefaultModel)), jevDefaultModel)
	req.Header.Set("Authorization", "Bearer "+config.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ai-gateway-protocol-version", "0.0.1")
	req.Header.Set("ai-gateway-auth-method", "api-key")
	req.Header.Set("ai-evaluation-model-specification-version", "4")
	req.Header.Set("ai-model-id", model)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		markUsage(502, 0)
		return nil, err
	}
	defer resp.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if readErr != nil {
		markUsage(502, 0)
		return nil, readErr
	}
	var response map[string]any
	if json.Unmarshal(data, &response) != nil {
		markUsage(resp.StatusCode, 0)
		return nil, fmt.Errorf("Jev evaluator response is invalid")
	}
	inputTokens, _ := findUsage(response)
	markUsage(resp.StatusCode, inputTokens)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Jev evaluator returned %d: %s", resp.StatusCode, clip(string(data), 500))
	}
	answers, ok := response["answers"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Jev evaluator response has no answers")
	}
	result := make(map[string]jevAnswer, len(questions))
	for name, question := range questions {
		raw, ok := answers[name].(map[string]any)
		if !ok || stringValue(raw["type"]) != "choice" {
			return nil, fmt.Errorf("Jev evaluator omitted choice %q", name)
		}
		choice := stringValue(raw["choice"])
		if _, allowed := question.Criteria[choice]; !allowed {
			return nil, fmt.Errorf("Jev evaluator returned an unavailable choice for %q", name)
		}
		answer := jevAnswer{Choice: choice}
		if confidence, ok := raw["confidence"].(float64); ok {
			if confidence < 0 || confidence > 1 {
				return nil, fmt.Errorf("Jev evaluator returned invalid confidence for %q", name)
			}
			answer.Confidence, answer.HasConf = confidence, true
		}
		result[name] = answer
	}
	return result, nil
}
