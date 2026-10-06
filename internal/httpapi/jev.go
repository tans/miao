package httpapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/jev"
)

const jevUpstreamCommit = jev.UpstreamCommit
const jevDefaultModel = jev.DefaultModel

// Tests inject an isolated endpoint; production always uses the Gateway.
var jevEvaluationURL = jev.Endpoint

type jevQuestion = jev.Question
type jevAnswer = jev.Answer

func (s *Server) evaluateJev(ctx context.Context, tenantID, userID, appID string, state map[string]any, questions map[string]jevQuestion) (map[string]jevAnswer, error) {
	key := strings.TrimSpace(env("MIAO_JEV_API_KEY", ""))
	if key == "" {
		config, err := s.readAIConfig(ctx)
		if err != nil {
			return nil, err
		}
		if config.Provider == "vercel" {
			key = config.Key
		}
	}
	if key == "" {
		return nil, fmt.Errorf("Jev 需要 Vercel Gateway 密钥；非 Vercel 生成模型请单独配置 MIAO_JEV_API_KEY")
	}
	if err := harness.ReserveModelRequest(ctx); err != nil {
		return nil, err
	}
	usage, err := s.reserveAIUsage(ctx, tenantID, userID, appID)
	if err != nil {
		return nil, err
	}
	evaluator := jev.Evaluator{
		APIKey: key,
		Model:  defaultString(strings.TrimSpace(env("MIAO_JEV_MODEL", jevDefaultModel)), jevDefaultModel),
		URL:    jevEvaluationURL,
	}
	result, evaluationErr := evaluator.Evaluate(ctx, state, questions)
	status := result.Status
	if status == 0 {
		status = 502
	}
	_, recordErr := s.PB.Update(context.Background(), "ai_usage", stringValue(usage["id"]), map[string]any{"status": status, "input_tokens": result.InputTokens})
	if evaluationErr != nil {
		return nil, evaluationErr
	}
	if recordErr != nil {
		return nil, fmt.Errorf("Jev 用量回执保存失败: %w", recordErr)
	}
	return result.Answers, nil
}
