package httpapi

import (
	"context"
	"fmt"
	"github.com/tans/miao/internal/settings"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (s *Server) writeAdminAudit(ctx context.Context, id identity, action, targetType, targetID, reason string, status int) map[string]any {
	row, err := s.PB.Create(ctx, "platform_audit_logs", map[string]any{
		"actor_id": id.User["id"], "actor_email": id.User["email"], "action": action,
		"target_type": targetType, "target_id": clip(targetID, 64), "reason": clip(strings.TrimSpace(reason), 500), "status": status,
	})
	if err != nil {
		return nil
	}
	return row
}

func (s *Server) finishAdminAudit(ctx context.Context, audit map[string]any, action string, status int) {
	if audit != nil {
		_, _ = s.PB.Update(ctx, "platform_audit_logs", stringValue(audit["id"]), map[string]any{"action": action, "status": status})
	}
}

func (s *Server) adminCount(ctx context.Context, collection, filter string) (int, error) {
	_, count, _, err := s.PB.List(ctx, collection, filter, "", 1, 1)
	return count, err
}

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	end := time.Now().UTC()
	start := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	users, e1 := s.adminCount(ctx, "users", "")
	disabled, e2 := s.adminCount(ctx, "users", "disabled = true")
	workspaces, e3 := s.adminCount(ctx, "tenants", "")
	apps, e4 := s.adminCount(ctx, "apps", "")
	usage, e5 := s.aggregateAdminUsage(ctx, start, end)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil {
		writeError(w, 503, "平台总览暂时不可用")
		return
	}
	writeJSON(w, 200, map[string]any{"generated_at": end.Format(time.RFC3339Nano), "users": map[string]any{"total": users, "active": users - disabled, "disabled": disabled}, "workspaces": workspaces, "apps": apps, "ai_today": usage["totals"], "ai_today_by_kind": usage["by_kind"]})
}

func (s *Server) adminRuntime(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	config, err := settings.ReadLLMConfig(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "运行配置状态暂时不可用")
		return
	}
	registration, err := settings.ReadRegistration(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "注册策略暂时不可用")
		return
	}
	decision, err := settings.ReadJevConfig(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "JEV 运行配置状态暂时不可用")
		return
	}
	mailConfig, err := settings.ReadMail(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "邮件服务配置暂时不可用")
		return
	}
	writeJSON(w, 200, map[string]any{
		"registration": map[string]any{"mode": registration.Mode, "email_verification_required": registration.RequireEmailVerification, "allowed_email_domains": registration.AllowedEmailDomains},
		"mail":         map[string]any{"configured": mailConfig.Configured(), "public_url_configured": mailConfig.PublicURL != ""},
		"ai":           map[string]any{"enabled": config.Enabled, "provider": config.Provider, "model": config.Model, "configured": config.Enabled && config.Key != "", "source": config.Source, "encryption_key_ready": settings.EncryptionReady()},
		"jev":          map[string]any{"enabled": decision.Enabled, "provider": decision.Provider, "model": decision.Model, "configured": decision.Enabled && decision.Key != "", "source": decision.Source},
	})
}

func adminPage(r *http.Request) (int, int) {
	return queryInt(r, "page", 1, 1, 1000000), queryInt(r, "perPage", 25, 1, 100)
}
func adminSearch(q, field string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	return field + " ~ " + pbFilterString(clip(q, 120))
}
func appendFilter(parts []string, value string) []string {
	if value != "" {
		return append(parts, value)
	}
	return parts
}
func adminPageJSON(items []map[string]any, page, perPage, total int) map[string]any {
	return pageResult(items, page, perPage, total)
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	page, perPage := adminPage(r)
	filters := []string{}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		v := pbFilterString(clip(q, 120))
		filters = append(filters, "(email ~ "+v+" || name ~ "+v+")")
	}
	switch r.URL.Query().Get("status") {
	case "active":
		filters = append(filters, "disabled = false")
	case "disabled":
		filters = append(filters, "disabled = true")
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	rows, total, _, err := s.PB.List(ctx, "users", listFilter(filters...), "-created", page, perPage)
	if err != nil {
		writeError(w, 503, "用户列表暂时不可用")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row["id"], "email": row["email"], "name": row["name"], "verified": boolValue(row["verified"]), "disabled": boolValue(row["disabled"]), "created_at": row["created"], "updated_at": row["updated"]})
	}
	writeJSON(w, 200, adminPageJSON(items, page, perPage, total))
}

func (s *Server) adminUserStatus(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	body := mapBody(r)
	disabled, ok := body["disabled"].(bool)
	reason := strings.TrimSpace(stringValue(body["reason"]))
	target := pathID(r, "id")
	reject := func(action string, status int, msg string) {
		s.writeAdminAudit(r.Context(), id, action, "user", target, reason, status)
		writeError(w, status, msg)
	}
	if !ok {
		reject("user.status.rejected", 400, "账号状态无效")
		return
	}
	if len([]rune(reason)) < 5 || len([]rune(reason)) > 500 {
		reject("user.status.rejected", 400, "请填写 5 到 500 个字符的操作原因")
		return
	}
	if disabled && target == stringValue(id.User["id"]) {
		reject("user.status.rejected", 409, "不能停用当前登录的平台管理员账号")
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	user, err := s.PB.Get(ctx, "users", target)
	if err != nil {
		reject("user.status.rejected", 404, "用户不存在")
		return
	}
	adminEmails, err := settings.ReadAdmins(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "平台管理员名单暂不可用")
		return
	}
	verification, err := settings.ReadRegistration(ctx, s.PB)
	if err != nil {
		writeError(w, 503, "注册策略暂时不可用")
		return
	}
	if disabled && containsString(adminEmails.Emails, strings.ToLower(stringValue(user["email"]))) {
		available := 0
		for _, email := range adminEmails.Emails {
			row, e := s.PB.Find(ctx, "users", "email = "+pbFilterString(email))
			if e == nil && !boolValue(row["disabled"]) && (!verification.RequireEmailVerification || boolValue(row["verified"])) {
				available++
			}
		}
		if available <= 1 {
			reject("user.status.rejected", 409, "必须至少保留一个可用的平台管理员账号")
			return
		}
	}
	audit := s.writeAdminAudit(ctx, id, "user.status.change_requested", "user", target, reason, 102)
	if audit == nil {
		writeError(w, 503, "平台审计服务暂不可用，未执行账号状态变更")
		return
	}
	if _, err = s.PB.Update(ctx, "users", target, map[string]any{"disabled": disabled}); err != nil {
		s.finishAdminAudit(ctx, audit, "user.status.failed", 503)
		writeError(w, 503, "账号状态更新失败")
		return
	}
	s.finishAdminAudit(ctx, audit, fmt.Sprintf("user.%s", map[bool]string{true: "disabled", false: "enabled"}[disabled]), 200)
	writeJSON(w, 200, map[string]any{"ok": true, "id": target, "disabled": disabled})
}

func (s *Server) adminWorkspaces(w http.ResponseWriter, r *http.Request) {
	page, perPage := adminPage(r)
	filter := adminSearch(r.URL.Query().Get("q"), "name")
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		v := pbFilterString(clip(q, 120))
		filter = "(name ~ " + v + " || slug ~ " + v + ")"
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	rows, total, _, err := s.PB.List(ctx, "tenants", filter, "-created", page, perPage)
	if err != nil {
		writeError(w, 503, "工作区列表暂时不可用")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		owner, _ := s.PB.Get(ctx, "users", stringValue(row["owner_id"]))
		members, _ := s.adminCount(ctx, "tenant_members", "tenant_id = "+pbFilterString(stringValue(row["id"])))
		apps, _ := s.adminCount(ctx, "apps", "tenant_id = "+pbFilterString(stringValue(row["id"])))
		var ownerView any
		if len(owner) > 0 {
			ownerView = map[string]any{"id": owner["id"], "name": owner["name"], "email": owner["email"], "disabled": boolValue(owner["disabled"])}
		}
		items = append(items, map[string]any{"id": row["id"], "name": row["name"], "slug": row["slug"], "owner": ownerView, "member_count": members, "app_count": apps, "created_at": row["created"]})
	}
	writeJSON(w, 200, adminPageJSON(items, page, perPage, total))
}

func (s *Server) adminApps(w http.ResponseWriter, r *http.Request) {
	page, perPage := adminPage(r)
	filters := appendFilter(nil, adminSearch(r.URL.Query().Get("q"), "name"))
	switch r.URL.Query().Get("archived") {
	case "true":
		filters = append(filters, "archived = true")
	case "false":
		filters = append(filters, "archived = false")
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	rows, total, _, err := s.PB.List(ctx, "apps", listFilter(filters...), "-updated", page, perPage)
	if err != nil {
		writeError(w, 503, "应用目录暂时不可用")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		tenant, _ := s.PB.Get(ctx, "tenants", stringValue(row["tenant_id"]))
		tables, _ := s.adminCount(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(row["tenant_id"])), "app_id = "+pbFilterString(stringValue(row["id"]))))
		var tenantView any
		if len(tenant) > 0 {
			tenantView = map[string]any{"id": tenant["id"], "name": tenant["name"]}
		}
		items = append(items, map[string]any{"id": row["id"], "name": row["name"], "tenant": tenantView, "archived": boolValue(row["archived"]), "restricted": boolValue(row["restricted"]), "table_count": tables, "created_at": row["created"], "updated_at": row["updated"]})
	}
	writeJSON(w, 200, adminPageJSON(items, page, perPage, total))
}

func (s *Server) aggregateAdminUsage(ctx context.Context, from, to time.Time) (map[string]any, error) {
	return s.aggregateUsage(ctx, from, to, "", "")
}

func (s *Server) adminUsage(w http.ResponseWriter, r *http.Request) {
	end := time.Now().UTC()
	if raw := r.URL.Query().Get("to"); raw != "" {
		var err error
		end, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, 400, "用量查询时间范围无效")
			return
		}
	}
	start := end.Add(-6 * 24 * time.Hour)
	if raw := r.URL.Query().Get("from"); raw != "" {
		var err error
		start, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, 400, "用量查询时间范围无效")
			return
		}
	}
	if start.After(end) {
		writeError(w, 400, "用量查询时间范围无效")
		return
	}
	if end.Sub(start) > 31*24*time.Hour {
		writeError(w, 400, "单次用量查询范围最多为 31 天")
		return
	}
	page, perPage := adminPage(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	kind := r.URL.Query().Get("kind")
	if !validUsageKind(kind) {
		writeError(w, 400, "用量类型无效")
		return
	}
	aggregate, err := s.aggregateUsage(ctx, start, end, "", kind)
	if err != nil {
		writeError(w, 503, "平台用量暂时不可用")
		return
	}
	rows := asSliceMap(aggregate["byTenant"])
	sort.Slice(rows, func(i, j int) bool { return intValue(rows[i]["requests"]) > intValue(rows[j]["requests"]) })
	total := len(rows)
	from := (page - 1) * perPage
	if from > total {
		from = total
	}
	to := from + perPage
	if to > total {
		to = total
	}
	items := make([]map[string]any, 0, to-from)
	for _, row := range rows[from:to] {
		tenant, _ := s.PB.Get(ctx, "tenants", stringValue(row["tenant_id"]))
		if len(tenant) > 0 {
			row["tenant"] = map[string]any{"id": tenant["id"], "name": tenant["name"]}
		} else {
			row["tenant"] = nil
		}
		items = append(items, row)
	}
	writeJSON(w, 200, map[string]any{"from": start.Format(time.RFC3339Nano), "to": end.Format(time.RFC3339Nano), "totals": aggregate["totals"], "by_kind": aggregate["by_kind"], "items": items, "page": page, "perPage": perPage, "totalItems": total, "totalPages": (total + perPage - 1) / perPage})
}

func (s *Server) adminAudit(w http.ResponseWriter, r *http.Request) {
	page, perPage := adminPage(r)
	filters := appendFilter(nil, func() string {
		if v := r.URL.Query().Get("targetType"); v != "" {
			return "target_type = " + pbFilterString(v)
		}
		return ""
	}())
	filters = appendFilter(filters, func() string {
		if v := r.URL.Query().Get("targetId"); v != "" {
			return "target_id = " + pbFilterString(v)
		}
		return ""
	}())
	ctx, cancel := contextTimeout(r)
	defer cancel()
	rows, total, _, err := s.PB.List(ctx, "platform_audit_logs", listFilter(filters...), "-created", page, perPage)
	if err != nil {
		writeError(w, 503, "平台审计暂时不可用")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row["id"], "actor_id": row["actor_id"], "actor_email": row["actor_email"], "action": row["action"], "target_type": row["target_type"], "target_id": row["target_id"], "reason": row["reason"], "status": row["status"], "created_at": row["created"]})
	}
	writeJSON(w, 200, adminPageJSON(items, page, perPage, total))
}
