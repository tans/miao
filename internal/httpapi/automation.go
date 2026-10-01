package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *Server) routesAutomations() {
	s.Mux.HandleFunc("GET /api/apps/{id}/automations", s.auth(s.listAutomations))
	s.Mux.HandleFunc("POST /api/apps/{id}/automations", s.auth(s.createAutomation))
	s.Mux.HandleFunc("POST /api/apps/{id}/automations/{ruleId}/enable", s.auth(s.enableAutomation))
	s.Mux.HandleFunc("GET /api/notifications", s.auth(s.listNotifications))
	s.Mux.HandleFunc("POST /api/notifications/{notificationId}/read", s.auth(s.readNotification))
}

func automationPublic(rule map[string]any) map[string]any {
	return map[string]any{"id": rule["id"], "name": rule["name"], "definition": rule["definition"], "enabled": boolValue(rule["enabled"]), "pause_reason": defaultString(stringValue(rule["pause_reason"]), "")}
}

func (s *Server) listAutomations(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	rows, err := s.PB.ListAll(ctx, "automation_rules", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "-created")
	if err != nil {
		writeError(w, 503, "自动化规则暂不可用")
		return
	}
	out := []map[string]any{}
	for _, row := range rows {
		out = append(out, automationPublic(row))
	}
	writeJSON(w, 200, out)
}

func (s *Server) createAutomation(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canPublishAppRole(role) {
		writeError(w, 403, "没有配置自动化的权限")
		return
	}
	input := mapBody(r)
	definition := asMap(input["definition"])
	trigger := stringValue(definition["trigger"])
	action := asMap(definition["action"])
	if len(action) == 0 {
		action = map[string]any{"type": "notify"}
	}
	actionType := stringValue(action["type"])
	if !containsString([]string{"record_created", "status_changed", "due"}, trigger) || !containsString([]string{"notify", "set_field"}, actionType) || (actionType == "notify" && stringValue(definition["recipient_id"]) == "") || (actionType == "set_field" && trigger != "record_created") {
		writeError(w, 400, "触发条件或动作无效")
		return
	}
	id := who(r)
	table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(stringValue(definition["table"]))))
	if err != nil {
		writeError(w, 400, "目标表不存在")
		return
	}
	fields := asSliceMap(table["fields"])
	conditionField := findField(fields, stringValue(definition["field"]))
	if actionType == "notify" {
		member, err := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "user_id = "+pbFilterString(stringValue(definition["recipient_id"]))))
		if err != nil {
			writeError(w, 400, "接收人不是工作区成员")
			return
		}
		recipient := identity{User: map[string]any{"id": member["user_id"]}, Tenant: id.Tenant, Membership: member}
		if app["restricted"] == true && s.appPermission(ctx, app, recipient) == "" {
			writeError(w, 400, "接收人没有此应用的访问权限")
			return
		}
	}
	if trigger == "status_changed" && (conditionField == nil || conditionField["type"] != "select" || !contains(conditionField["options"], definition["from"]) || !contains(conditionField["options"], definition["to"])) {
		writeError(w, 400, "状态条件无效")
		return
	}
	if trigger == "due" && (conditionField == nil || conditionField["type"] != "date" || intValue(definition["offset_days"]) < 0 || intValue(definition["offset_days"]) > 30) {
		writeError(w, 400, "到期条件无效")
		return
	}
	timezone := defaultString(stringValue(definition["timezone"]), "Asia/Shanghai")
	if trigger == "due" {
		if _, err := time.LoadLocation(timezone); err != nil {
			writeError(w, 400, "时区无效")
			return
		}
	}
	safeAction := map[string]any{"type": "notify"}
	if actionType == "set_field" {
		field := findField(fields, stringValue(action["field"]))
		if field == nil || !validAutomationValue(field, action["value"]) {
			writeError(w, 400, "表单提交后的动作字段或值无效")
			return
		}
		safeAction = map[string]any{"type": "set_field", "field": field["name"], "value": action["value"]}
	}
	safe := map[string]any{"trigger": trigger, "table": table["slug"], "action": safeAction}
	if actionType == "notify" {
		safe["recipient_id"] = definition["recipient_id"]
	}
	if conditionField != nil {
		safe["field"] = conditionField["name"]
	}
	if trigger == "status_changed" {
		safe["from"], safe["to"] = definition["from"], definition["to"]
	}
	if trigger == "due" {
		safe["offset_days"], safe["timezone"] = intValue(definition["offset_days"]), timezone
	}
	name := clip(strings.TrimSpace(stringValue(input["name"])), 160)
	if name == "" {
		name = "提醒"
	}
	rule, err := s.PB.Create(ctx, "automation_rules", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "created_by": id.User["id"], "name": name, "definition": safe, "enabled": false})
	if err != nil {
		writeError(w, 503, "自动化规则创建失败")
		return
	}
	writeJSON(w, 201, automationPublic(rule))
}

func validAutomationValue(field map[string]any, value any) bool {
	if value == nil || boolValue(field["required"]) && value == "" {
		return false
	}
	switch field["type"] {
	case "bool":
		_, ok := value.(bool)
		return ok
	case "number":
		_, ok := numeric(value)
		return ok
	case "text", "select", "email", "url", "date":
		text, ok := value.(string)
		return ok && (field["type"] != "select" || contains(field["options"], text))
	}
	return false
}

func (s *Server) ownAutomation(ctx context.Context, r *http.Request) (map[string]any, map[string]any, bool) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, nil, false
	}
	rule, err := s.PB.Get(ctx, "automation_rules", pathID(r, "ruleId"))
	if err != nil || rule["tenant_id"] != who(r).Tenant["id"] || rule["app_id"] != app["id"] {
		return app, nil, false
	}
	return app, rule, true
}
func (s *Server) enableAutomation(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, rule, ok := s.ownAutomation(ctx, r)
	if !ok {
		writeError(w, 404, "规则不存在")
		return
	}
	if !canPublishAppRole(s.appPermission(ctx, app, who(r))) {
		writeError(w, 403, "需要有权限的用户确认启用")
		return
	}
	input := mapBody(r)
	if input["confirm"] != true {
		writeError(w, 403, "需要有权限的用户确认启用")
		return
	}
	enabled := boolValue(input["enabled"])
	updated, err := s.PB.Update(ctx, "automation_rules", stringValue(rule["id"]), map[string]any{"enabled": enabled, "pause_reason": ""})
	if err != nil {
		writeError(w, 503, "自动化设置暂不可用")
		return
	}
	writeJSON(w, 200, map[string]any{"id": updated["id"], "enabled": updated["enabled"]})
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	id := who(r)
	page := queryInt(r, "page", 1, 1, 10000)
	rows, total, pages, err := s.PB.List(ctx, "automation_notifications", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "user_id = "+pbFilterString(stringValue(id.User["id"]))), "-created", page, 30)
	if err != nil {
		writeError(w, 503, "通知暂不可用")
		return
	}
	items := []map[string]any{}
	for _, row := range rows {
		app, e := s.PB.Get(ctx, "apps", stringValue(row["app_id"]))
		if e != nil || app["tenant_id"] != id.Tenant["id"] || s.appPermission(ctx, app, id) == "" {
			continue
		}
		items = append(items, map[string]any{"id": row["id"], "app_id": row["app_id"], "run_id": defaultString(stringValue(row["run_id"]), ""), "message": row["message"], "read": boolValue(row["read"]), "created_at": row["created"]})
	}
	if r.URL.Query().Has("page") {
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "perPage": 30, "totalItems": total, "totalPages": pages})
	} else {
		writeJSON(w, 200, items)
	}
}

func (s *Server) readNotification(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	id := who(r)
	item, err := s.PB.Get(ctx, "automation_notifications", pathID(r, "notificationId"))
	if err != nil || item["tenant_id"] != id.Tenant["id"] || item["user_id"] != id.User["id"] {
		writeError(w, 404, "通知不存在")
		return
	}
	app, err := s.PB.Get(ctx, "apps", stringValue(item["app_id"]))
	if err != nil || app["tenant_id"] != id.Tenant["id"] || s.appPermission(ctx, app, id) == "" {
		writeError(w, 404, "通知不存在或权限已变化")
		return
	}
	_, err = s.PB.Update(ctx, "automation_notifications", stringValue(item["id"]), map[string]any{"read": true})
	if err != nil {
		writeError(w, 503, "通知更新失败")
		return
	}
	writeJSON(w, 200, map[string]any{"id": item["id"], "read": true})
}

func (s *Server) processRecordAutomation(ctx context.Context, tenantID, appID, slug, event string, before, after map[string]any) {
	s.processTaskRecordEvent(ctx, tenantID, appID, slug, event, before, after)
	rules, err := s.PB.ListAll(ctx, "automation_rules", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID), "enabled = true"), "")
	if err != nil {
		return
	}
	for _, rule := range rules {
		definition := asMap(rule["definition"])
		if definition["table"] != slug {
			continue
		}
		deliver := false
		message := ""
		switch definition["trigger"] {
		case "record_created":
			deliver = event == "created"
			message = stringValue(rule["name"]) + "：新增了一条记录"
		case "status_changed":
			field := stringValue(definition["field"])
			deliver = event == "updated" && before != nil && before[field] == definition["from"] && after[field] == definition["to"]
			message = stringValue(rule["name"]) + "：记录状态已更新"
		}
		if !deliver {
			continue
		}
		key := clip(event+":"+stringValue(after["id"])+":"+stringValue(after["updated"]), 160)
		_ = s.deliverAutomation(ctx, rule, key, message, after)
	}
}

func (s *Server) deliverAutomation(ctx context.Context, rule map[string]any, key, message string, source map[string]any) error {
	definition := asMap(rule["definition"])
	action := asMap(definition["action"])
	recipientID := stringValue(definition["recipient_id"])
	tenantID, appID := stringValue(rule["tenant_id"]), stringValue(rule["app_id"])
	creatorMembership, creatorErr := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(tenantID), "user_id = "+pbFilterString(stringValue(rule["created_by"]))))
	app, appErr := s.PB.Get(ctx, "apps", appID)
	if creatorErr != nil || appErr != nil || boolValue(app["archived"]) {
		_, _ = s.PB.Update(ctx, "automation_rules", stringValue(rule["id"]), map[string]any{"enabled": false, "pause_reason": "规则创建者已失去权限或应用已归档"})
		return nil
	}
	creator, err := s.PB.Get(ctx, "users", stringValue(rule["created_by"]))
	if err != nil || boolValue(creator["disabled"]) {
		_, _ = s.PB.Update(ctx, "automation_rules", stringValue(rule["id"]), map[string]any{"enabled": false, "pause_reason": "规则创建者已离开工作区"})
		return nil
	}
	creatorIdentity := identity{User: creator, Tenant: map[string]any{"id": tenantID, "owner_id": app["owner_id"]}, Membership: creatorMembership}
	tenant, _ := s.PB.Get(ctx, "tenants", tenantID)
	creatorIdentity.Tenant = tenant
	if !canPublishAppRole(s.appPermission(ctx, app, creatorIdentity)) {
		_, _ = s.PB.Update(ctx, "automation_rules", stringValue(rule["id"]), map[string]any{"enabled": false, "pause_reason": "创建者已失去规则管理权限"})
		return nil
	}
	if action["type"] != "set_field" {
		member, err := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(tenantID), "user_id = "+pbFilterString(recipientID)))
		if err != nil {
			_, _ = s.PB.Update(ctx, "automation_rules", stringValue(rule["id"]), map[string]any{"enabled": false, "pause_reason": "接收人已离开工作区"})
			return nil
		}
		recipient, err := s.PB.Get(ctx, "users", recipientID)
		if err != nil || boolValue(recipient["disabled"]) {
			return nil
		}
		recipientIdentity := identity{User: recipient, Tenant: tenant, Membership: member}
		if s.appPermission(ctx, app, recipientIdentity) == "" {
			_, _ = s.PB.Update(ctx, "automation_rules", stringValue(rule["id"]), map[string]any{"enabled": false, "pause_reason": "接收人已失去应用权限"})
			return nil
		}
	}
	existing, err := s.PB.Find(ctx, "automation_runs", listFilter("rule_id = "+pbFilterString(stringValue(rule["id"])), "event_key = "+pbFilterString(key)))
	if err == nil && existing["status"] == "delivered" {
		return nil
	}
	run := existing
	if err != nil {
		run, err = s.PB.Create(ctx, "automation_runs", map[string]any{"tenant_id": tenantID, "app_id": appID, "rule_id": rule["id"], "event_key": key, "status": "pending"})
		if err != nil {
			return nil
		}
	}
	if action["type"] == "set_field" {
		meta, e := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID), "slug = "+pbFilterString(stringValue(definition["table"]))))
		if e != nil {
			return e
		}
		row, e := s.PB.Get(ctx, stringValue(meta["pb_collection"]), stringValue(source["id"]))
		if e != nil || row["tenant_id"] != tenantID || row["app_id"] != appID {
			return fmt.Errorf("source record missing")
		}
		if stringValue(rule["created_by"]) != stringValue(creator["id"]) {
			return fmt.Errorf("creator changed")
		}
		_, e = s.PB.UpdateBusiness(ctx, stringValue(meta["pb_collection"]), stringValue(row["id"]), map[string]any{stringValue(action["field"]): action["value"]}, stringValue(row["updated"]), stringValue(rule["created_by"]), "background")
		if e != nil {
			return e
		}
		_, e = s.PB.Update(ctx, "automation_runs", stringValue(run["id"]), map[string]any{"status": "delivered", "result": map[string]any{"record_id": row["id"], "action": "set_field"}})
		return e
	}
	_, err = s.PB.Create(ctx, "automation_notifications", map[string]any{"tenant_id": tenantID, "app_id": appID, "rule_id": rule["id"], "event_key": key, "user_id": recipientID, "message": message})
	if err != nil {
		duplicate, _ := s.PB.Find(ctx, "automation_notifications", listFilter("rule_id = "+pbFilterString(stringValue(rule["id"])), "event_key = "+pbFilterString(key), "user_id = "+pbFilterString(recipientID)))
		if duplicate == nil {
			return err
		}
	}
	_, err = s.PB.Update(ctx, "automation_runs", stringValue(run["id"]), map[string]any{"status": "delivered", "result": map[string]any{"recipient_id": recipientID}})
	return err
}

func (s *Server) scanDueAutomation(ctx context.Context) {
	rules, err := s.PB.ListAll(ctx, "automation_rules", "enabled = true", "")
	if err != nil {
		return
	}
	for _, rule := range rules {
		definition := asMap(rule["definition"])
		if definition["trigger"] != "due" {
			continue
		}
		table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(rule["tenant_id"])), "app_id = "+pbFilterString(stringValue(rule["app_id"])), "slug = "+pbFilterString(stringValue(definition["table"]))))
		if err != nil {
			_, _ = s.PB.Update(ctx, "automation_rules", stringValue(rule["id"]), map[string]any{"enabled": false, "pause_reason": "目标字段或数据表已失效"})
			continue
		}
		field := findField(asSliceMap(table["fields"]), stringValue(definition["field"]))
		if field == nil || field["type"] != "date" {
			_, _ = s.PB.Update(ctx, "automation_rules", stringValue(rule["id"]), map[string]any{"enabled": false, "pause_reason": "目标字段或数据表已失效"})
			continue
		}
		timezone := defaultString(stringValue(definition["timezone"]), "Asia/Shanghai")
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			_, _ = s.PB.Update(ctx, "automation_rules", stringValue(rule["id"]), map[string]any{"enabled": false, "pause_reason": "时区无效"})
			continue
		}
		now := time.Now().In(loc)
		start := now.Format("2006-01-02")
		end := now.AddDate(0, 0, intValue(definition["offset_days"])+1).Format("2006-01-02")
		filter := listFilter("tenant_id = "+pbFilterString(stringValue(rule["tenant_id"])), "app_id = "+pbFilterString(stringValue(rule["app_id"])), stringValue(definition["field"])+" >= "+pbFilterString(start), stringValue(definition["field"])+" < "+pbFilterString(end))
		for page := 1; page <= 100; page++ {
			rows, _, pages, err := s.PB.List(ctx, stringValue(table["pb_collection"]), filter, stringValue(definition["field"]), page, 100)
			if err != nil {
				break
			}
			for _, row := range rows {
				key := "due:" + stringValue(row["id"]) + ":" + stringValue(row[stringValue(definition["field"])])
				_ = s.deliverAutomation(ctx, rule, clip(key, 160), stringValue(rule["name"])+"：有一项任务即将到期", row)
			}
			if page >= pages {
				break
			}
		}
	}
}
