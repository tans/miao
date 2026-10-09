package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

func validUsageKind(kind string) bool {
	return containsString([]string{"", "llm", "jev", "unclassified"}, kind)
}
func usageKind(row map[string]any) string {
	kind := stringValue(row["kind"])
	if kind != "llm" && kind != "jev" {
		return "unclassified"
	}
	return kind
}

func usageFilter(from, to time.Time, tenant, kind string) string {
	parts := []string{"created >= " + pbFilterString(from.Format("2006-01-02 15:04:05.000Z")), "created <= " + pbFilterString(to.Format("2006-01-02 15:04:05.000Z"))}
	if tenant != "" {
		parts = append(parts, "tenant_id = "+pbFilterString(tenant))
	}
	if kind == "unclassified" {
		parts = append(parts, `(kind != "llm" && kind != "jev")`)
	} else if kind != "" {
		parts = append(parts, "kind = "+pbFilterString(kind))
	}
	return listFilter(parts...)
}

func emptyUsageTotals() map[string]int {
	return map[string]int{"requests": 0, "successes": 0, "errors": 0, "pending": 0, "input_tokens": 0, "output_tokens": 0, "input_unknown": 0, "output_unknown": 0}
}
func addUsage(total map[string]int, row map[string]any) {
	total["requests"]++
	status := intValue(row["status"])
	bucket := "pending"
	if status >= 300 {
		bucket = "errors"
	} else if status >= 200 {
		bucket = "successes"
	}
	total[bucket]++
	total["input_tokens"] += intValue(row["input_tokens"])
	total["output_tokens"] += intValue(row["output_tokens"])
	if !boolValue(row["input_known"]) {
		total["input_unknown"]++
	}
	if !boolValue(row["output_known"]) {
		total["output_unknown"]++
	}
}

func (s *Server) aggregateUsage(ctx context.Context, from, to time.Time, tenant, kind string) (map[string]any, error) {
	rows, err := s.PB.ListAll(ctx, "ai_usage", usageFilter(from, to, tenant, kind), "created,id")
	if err != nil {
		return nil, err
	}
	total := emptyUsageTotals()
	byKind := map[string]map[string]int{"llm": emptyUsageTotals(), "jev": emptyUsageTotals(), "unclassified": emptyUsageTotals()}
	byTenant := map[string]map[string]int{}
	for _, row := range rows {
		addUsage(total, row)
		addUsage(byKind[usageKind(row)], row)
		tid := stringValue(row["tenant_id"])
		if byTenant[tid] == nil {
			byTenant[tid] = emptyUsageTotals()
		}
		addUsage(byTenant[tid], row)
	}
	items := []map[string]any{}
	for tid, values := range byTenant {
		item := map[string]any{"tenant_id": tid}
		for k, v := range values {
			item[k] = v
		}
		items = append(items, item)
	}
	return map[string]any{"totals": total, "by_kind": byKind, "byTenant": items}, nil
}

func (s *Server) harnessUsage(ctx context.Context, runID string) (map[string]any, error) {
	rows, err := s.PB.ListAll(ctx, "ai_usage", "run_id = "+pbFilterString(runID), "created,id")
	if err != nil {
		return nil, err
	}
	totals := emptyUsageTotals()
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		addUsage(totals, row)
		items = append(items, map[string]any{
			"created_at": row["created"], "kind": usageKind(row), "provider": row["provider"], "model": row["model"],
			"status": row["status"], "latency_ms": row["latency_ms"], "input_tokens": row["input_tokens"], "output_tokens": row["output_tokens"],
			"input_known": boolValue(row["input_known"]), "output_known": boolValue(row["output_known"]),
		})
	}
	return map[string]any{"totals": totals, "items": items}, nil
}

func usageDateRange(r *http.Request, today bool) (time.Time, time.Time, error) {
	to := time.Now().UTC()
	from := to.Add(-6 * 24 * time.Hour)
	if today {
		from = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	}
	var err error
	if raw := r.URL.Query().Get("from"); raw != "" {
		from, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return from, to, fmt.Errorf("用量开始时间无效")
		}
	}
	if raw := r.URL.Query().Get("to"); raw != "" {
		to, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return from, to, fmt.Errorf("用量结束时间无效")
		}
	}
	if from.After(to) || to.Sub(from) > 31*24*time.Hour {
		return from, to, fmt.Errorf("时间范围最多为连续 31 天")
	}
	return from, to, nil
}

func (s *Server) adminUsageRequests(w http.ResponseWriter, r *http.Request) {
	s.usageRequests(w, r, "")
}
func (s *Server) workspaceUsageRequests(w http.ResponseWriter, r *http.Request) {
	s.usageRequests(w, r, stringValue(who(r).Tenant["id"]))
}
func (s *Server) usageRequests(w http.ResponseWriter, r *http.Request, tenant string) {
	from, to, err := usageDateRange(r, tenant != "")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	kind := r.URL.Query().Get("kind")
	if !validUsageKind(kind) {
		writeError(w, 400, "用量类型无效")
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	page, perPage := adminPage(r)
	rows, total, _, err := s.PB.List(ctx, "ai_usage", usageFilter(from, to, tenant, kind), "-created,-id", page, perPage)
	if err != nil {
		writeError(w, 503, "请求明细暂不可用")
		return
	}
	items := []map[string]any{}
	for _, row := range rows {
		item := map[string]any{"id": row["id"], "created_at": row["created"], "kind": usageKind(row), "provider": row["provider"], "model": row["model"], "status": row["status"], "latency_ms": row["latency_ms"], "input_tokens": row["input_tokens"], "output_tokens": row["output_tokens"], "input_known": boolValue(row["input_known"]), "output_known": boolValue(row["output_known"])}
		if tenant == "" {
			if space, e := s.PB.Get(ctx, "tenants", stringValue(row["tenant_id"])); e == nil {
				item["tenant"] = map[string]any{"id": space["id"], "name": space["name"]}
			}
		}
		items = append(items, item)
	}
	writeJSON(w, 200, pageResult(items, page, perPage, total))
}
