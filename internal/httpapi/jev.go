package httpapi

import (
	"context"
	"fmt"
	"time"

	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/jev"
	"github.com/tans/miao/internal/settings"
)

const jevUpstreamCommit = jev.UpstreamCommit
const jevDefaultModel = jev.DefaultModel
const jevOfficialDefaultModel = jev.OfficialDefaultModel

type jevQuestion = jev.Question
type jevAnswer = jev.Answer

func (s *Server) evaluateJev(ctx context.Context, tenantID, userID, appID string, state map[string]any, questions map[string]jevQuestion) (map[string]jevAnswer, error) {
	config, err := settings.ReadJevConfig(ctx, s.PB)
	if err != nil {
		return nil, err
	}
	if !config.Enabled {
		return nil, fmt.Errorf("JEV 服务已停用")
	}
	if config.Key == "" {
		return nil, fmt.Errorf("JEV 尚未配置独立密钥，且无法复用 LLM 密钥")
	}
	if err := harness.ReserveModelRequest(ctx); err != nil {
		return nil, err
	}
	usage, err := s.reserveAIUsage(ctx, tenantID, userID, appID, "jev", config.Provider, config.Model)
	if err != nil {
		return nil, err
	}
	evaluator := jev.Evaluator{
		Provider: config.Provider,
		APIKey:   config.Key,
		Model:    config.Model,
	}
	start := time.Now()
	result, evaluationErr := evaluator.Evaluate(ctx, state, questions)
	status := result.Status
	if status == 0 || evaluationErr != nil && status < 300 {
		status = 502
	}
	recordCtx, recordCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer recordCancel()
	_, recordErr := s.PB.Update(recordCtx, "ai_usage", stringValue(usage["id"]), map[string]any{"status": status, "input_tokens": result.InputTokens, "input_known": result.InputKnown, "output_tokens": result.OutputTokens, "output_known": result.OutputKnown, "latency_ms": time.Since(start).Milliseconds()})
	if evaluationErr != nil {
		return nil, evaluationErr
	}
	if recordErr != nil {
		return nil, fmt.Errorf("Jev 用量回执保存失败: %w", recordErr)
	}
	return result.Answers, nil
}
