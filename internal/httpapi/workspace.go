package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Workspace-level member, usage, audit, and export handlers. Auth and account
// handlers stay in auth.go; these were split out along the route domains.

func (s *Server) requireManager(w http.ResponseWriter, id identity) bool {
	role := stringValue(id.Membership["role"])
	if role != "owner" && role != "admin" {
		writeError(w, 403, "只有工作区所有者或管理员可以管理成员")
		return false
	}
	if role == "owner" && id.Tenant["owner_id"] != id.User["id"] {
		writeError(w, 403, "只有工作区所有者可以管理成员")
		return false
	}
	return true
}

func (s *Server) aiUsage(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	from, to, err := usageDateRange(r, true)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	kind := r.URL.Query().Get("kind")
	if !validUsageKind(kind) {
		writeError(w, 400, "用量类型无效")
		return
	}
	result, err := s.aggregateUsage(ctx, from, to, stringValue(id.Tenant["id"]), kind)
	if err != nil {
		writeError(w, 503, "空间用量读取失败")
		return
	}
	result["from"], result["to"] = from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano)
	result["daily_limit"], result["llm_daily_limit"], result["jev_daily_limit"] = intValue(id.Tenant["ai_daily_limit"]), intValue(id.Tenant["ai_llm_daily_limit"]), intValue(id.Tenant["ai_jev_daily_limit"])
	result["can_manage"] = id.Membership["role"] == "owner"
	writeJSON(w, 200, result)
}

func (s *Server) aiBudget(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	input := mapBody(r)
	update := map[string]any{}
	for _, field := range []string{"daily_limit", "llm_daily_limit", "jev_daily_limit"} {
		raw, ok := input[field].(float64)
		if !ok || raw < 0 || raw > 100000 || raw != float64(int(raw)) {
			writeError(w, 400, "每日请求预算必须是 0 到 100000 的整数；0 表示不限制")
			return
		}
		update["ai_"+field] = int(raw)
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	if _, err := s.PB.Update(ctx, "tenants", stringValue(id.Tenant["id"]), update); err != nil {
		writeError(w, 503, "预算更新失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	page := queryInt(r, "page", 1, 1, 100000)
	filters := []string{"tenant_id = " + pbFilterString(stringValue(id.Tenant["id"]))}
	for key, field := range map[string]string{"actor": "actor_email", "action": "action", "target": "target_id"} {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			filters = append(filters, field+" ~ "+pbFilterString(clip(value, 120)))
		}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("status")); value != "" {
		status := queryInt(r, "status", 0, 200, 599)
		if status == 0 {
			writeError(w, 400, "结果状态无效")
			return
		}
		filters = append(filters, "status = "+strconv.Itoa(status))
	}
	for key, operator := range map[string]string{"from": ">=", "to": "<="} {
		if raw := strings.TrimSpace(r.URL.Query().Get(key)); raw != "" {
			at, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				writeError(w, 400, "时间范围必须使用 RFC3339 格式")
				return
			}
			filters = append(filters, "created "+operator+" "+pbFilterString(at.UTC().Format("2006-01-02 15:04:05.000Z")))
		}
	}
	rows, total, pages, err := s.PB.List(ctx, "audit_logs", listFilter(filters...), "-created", page, 100)
	if err != nil {
		writeError(w, 503, "审计记录暂不可用")
		return
	}
	items := []map[string]any{}
	for _, row := range rows {
		items = append(items, map[string]any{"id": row["id"], "actor_email": row["actor_email"], "action": row["action"], "route": row["route"], "target_id": row["target_id"], "status": row["status"], "created_at": row["created"]})
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "totalItems": total, "totalPages": pages})
}

func (s *Server) exportWorkspace(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	apps, err := s.PB.ListAll(ctx, "apps", "tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "created")
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	exported := []map[string]any{}
	for _, app := range apps {
		role := s.appPermission(ctx, app, id)
		if role == "" {
			continue
		}
		tables, err := s.appTables(ctx, app, stringValue(id.Tenant["id"]))
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
		tableOut := []map[string]any{}
		for _, table := range tables {
			rows, err := s.PB.ListAll(ctx, stringValue(table["pb_collection"]), listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "created")
			if err != nil {
				s.writeBusinessError(w, err)
				return
			}
			records := []map[string]any{}
			for _, row := range rows {
				records = append(records, publicRecord(row))
			}
			tableOut = append(tableOut, map[string]any{"name": table["name"], "slug": table["slug"], "fields": table["fields"], "records": records})
		}
		versions, err := s.PB.ListAll(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "version")
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
		exported = append(exported, map[string]any{"name": app["name"], "description": app["description"], "icon": app["icon"], "archived": app["archived"], "published_version_id": app["published_version_id"], "versions": versions, "tables": tableOut})
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\"miao-workspace-"+stringValue(id.Tenant["id"])+".json\"")
	writeJSON(w, 200, map[string]any{"exported_at": nowISO(), "workspace": publicTenant(id.Tenant, stringValue(id.Membership["role"])), "apps": exported})
}
