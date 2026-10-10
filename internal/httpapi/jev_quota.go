package httpapi

import (
	"context"
	"fmt"
	"time"

	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/pocketbase"
)

func aiRateLimitedWith(ctx context.Context, pb *pocketbase.Client, tenantID, userID, kind string) bool {
	start := time.Now().Add(-time.Minute).UTC().Format("2006-01-02 15:04:05.000Z")
	kindFilter := " && kind = " + pbFilterString(kind)
	_, tenantCount, _, e1 := pb.List(ctx, "ai_usage", "tenant_id = "+pbFilterString(tenantID)+kindFilter+" && created >= "+pbFilterString(start), "", 1, 1)
	_, userCount, _, e2 := pb.List(ctx, "ai_usage", "tenant_id = "+pbFilterString(tenantID)+" && user_id = "+pbFilterString(userID)+kindFilter+" && created >= "+pbFilterString(start), "", 1, 1)
	_, globalCount, _, e3 := pb.List(ctx, "ai_usage", "created >= "+pbFilterString(start), "", 1, 1)
	return e1 != nil || e2 != nil || e3 != nil || tenantCount >= 120 || userCount >= 30 || globalCount >= 500
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
	field := ""
	kindLabel := ""
	switch kind {
	case "llm":
		field, kindLabel = "ai_llm_daily_limit", "LLM"
	case "jev":
		field, kindLabel = "ai_jev_daily_limit", "JEV"
	default:
		return fmt.Errorf("AI 用量类型无效")
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
			return fmt.Errorf("工作区已达到今日 %s 请求预算", kindLabel)
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
		data := map[string]any{"tenant_id": tenantID, "user_id": userID, "app_id": appID, "kind": kind, "provider": provider, "model": model, "status": 100, "input_tokens": 0, "output_tokens": 0, "input_known": false, "output_known": false}
		if runID := harness.RunID(ctx); runID != "" {
			data["run_id"] = runID
		}
		var err error
		usage, err = tx.Create(ctx, "ai_usage", data)
		return err
	})
	return usage, err
}
