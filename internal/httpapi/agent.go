package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

var errConversationNotFound = fmt.Errorf("conversation not found")

var sessionLocks sync.Map

func sessionLock(key string) *sync.Mutex {
	v, _ := sessionLocks.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func (s *Server) threadApp(ctx context.Context, r *http.Request, appID string) bool {
	if appID == "" {
		return true
	}
	app, err := s.PB.Get(ctx, "apps", appID)
	if err != nil || app["tenant_id"] != who(r).Tenant["id"] {
		return false
	}
	return s.appPermission(ctx, app, who(r)) != ""
}
func (s *Server) listThreads(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	id := who(r)
	appID := r.URL.Query().Get("app_id")
	if appID != "" {
		app, err := s.PB.Get(ctx, "apps", appID)
		if err != nil || app["tenant_id"] != id.Tenant["id"] || s.appPermission(ctx, app, id) == "" {
			writeError(w, 404, "应用不存在或你没有访问权限")
			return
		}
	}
	page := queryInt(r, "page", 1, 1, 10000)
	perPage := queryInt(r, "perPage", 20, 1, 100)
	rows, total, pages, err := s.PB.List(ctx, "agent_threads", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "user_id = "+pbFilterString(stringValue(id.User["id"])), "app_id = "+pbFilterString(appID)), "-updated", page, perPage)
	if err != nil {
		writeError(w, 503, "对话列表暂不可用")
		return
	}
	items := []map[string]any{}
	for _, row := range rows {
		items = append(items, map[string]any{"id": row["id"], "title": row["title"], "app_id": row["app_id"], "updated_at": row["updated"]})
	}
	if r.URL.Query().Has("page") || r.URL.Query().Has("perPage") {
		writeJSON(w, 200, map[string]any{"items": items, "page": page, "perPage": perPage, "totalItems": total, "totalPages": pages})
		return
	}
	writeJSON(w, 200, items)
}
func (s *Server) createThread(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	id := who(r)
	input := mapBody(r)
	appID := stringValue(input["app_id"])
	if appID != "" {
		app, err := s.PB.Get(ctx, "apps", appID)
		if err != nil || app["tenant_id"] != id.Tenant["id"] || s.appPermission(ctx, app, id) == "" {
			writeError(w, 404, "应用不存在或你没有访问权限")
			return
		}
	}
	title := clip(defaultString(stringValue(input["title"]), T(r, "新对话")), 160)
	row, err := s.PB.Create(ctx, "agent_threads", map[string]any{"tenant_id": id.Tenant["id"], "app_id": appID, "user_id": id.User["id"], "title": title})
	if err != nil {
		writeError(w, 503, "对话创建失败")
		return
	}
	writeJSON(w, 201, map[string]any{"id": row["id"], "app_id": appID, "title": row["title"]})
}
func (s *Server) ownedThread(ctx context.Context, r *http.Request) (map[string]any, bool) {
	id := who(r)
	thread, err := s.PB.Get(ctx, "agent_threads", pathID(r, "threadId"))
	if err != nil || thread["tenant_id"] != id.Tenant["id"] || thread["user_id"] != id.User["id"] {
		return nil, false
	}
	appID := stringValue(thread["app_id"])
	if appID != "" {
		app, e := s.PB.Get(ctx, "apps", appID)
		if e != nil || s.appPermission(ctx, app, id) == "" {
			return nil, false
		}
	}
	return thread, true
}
func (s *Server) listThreadMessages(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	if _, ok := s.ownedThread(ctx, r); !ok {
		writeError(w, 404, "对话不存在")
		return
	}
	id := who(r)
	page := queryInt(r, "page", 1, 1, 100000)
	rows, total, pages, err := s.PB.List(ctx, "agent_messages", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "thread_id = "+pbFilterString(pathID(r, "threadId")), "user_id = "+pbFilterString(stringValue(id.User["id"]))), "created", page, 50)
	if err != nil {
		writeError(w, 503, "对话记录暂不可用")
		return
	}
	out := []map[string]any{}
	for _, row := range rows {
		out = append(out, map[string]any{"id": row["id"], "role": row["role"], "content": row["content"], "created_at": row["created"]})
	}
	writeJSON(w, 200, map[string]any{"items": out, "page": page, "totalPages": pages, "totalItems": total})
}
func (s *Server) createThreadMessage(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	thread, ok := s.ownedThread(ctx, r)
	if !ok {
		writeError(w, 404, "对话不存在")
		return
	}
	id := who(r)
	input := mapBody(r)
	role, content := stringValue(input["role"]), strings.TrimSpace(stringValue(input["content"]))
	if role != "user" || content == "" || len(content) > 30000 {
		writeError(w, 400, "消息内容无效")
		return
	}
	row, err := s.PB.Create(ctx, "agent_messages", map[string]any{"tenant_id": id.Tenant["id"], "thread_id": thread["id"], "user_id": id.User["id"], "role": role, "content": content})
	if err != nil {
		writeError(w, 503, "消息保存失败")
		return
	}
	if role == "user" {
		_, _ = s.PB.Update(ctx, "agent_threads", stringValue(thread["id"]), map[string]any{"title": clip(content, 80)})
	}
	writeJSON(w, 201, map[string]any{"id": row["id"], "role": role, "content": row["content"]})
}

func (s *Server) conversationScope(ctx context.Context, id identity, appID string) (string, error) {
	permissions := [][]string{}
	if appID != "" {
		app, err := s.PB.Get(ctx, "apps", appID)
		if err != nil || app["tenant_id"] != id.Tenant["id"] {
			return "", errConversationNotFound
		}
		role := s.appPermission(ctx, app, id)
		if role == "" {
			return "", errConversationNotFound
		}
		permissions = append(permissions, []string{appID, role})
	} else {
		apps, err := s.PB.ListAll(ctx, "apps", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "archived = false"), "")
		if err != nil {
			return "", err
		}
		for _, app := range apps {
			if role := s.appPermission(ctx, app, id); role != "" {
				permissions = append(permissions, []string{stringValue(app["id"]), role})
			}
		}
		sort.Slice(permissions, func(i, j int) bool { return permissions[i][0] < permissions[j][0] })
	}
	data, _ := json.Marshal(struct {
		TenantID string     `json:"tenant_id"`
		UserID   string     `json:"user_id"`
		Role     string     `json:"role"`
		AppID    string     `json:"app_id"`
		Apps     [][]string `json:"apps"`
	}{stringValue(id.Tenant["id"]), stringValue(id.User["id"]), stringValue(id.Membership["role"]), appID, permissions})
	return string(data), nil
}

func compatibleConversationScope(saved, current string, appID string) bool {
	var previous, next struct {
		TenantID string     `json:"tenant_id"`
		UserID   string     `json:"user_id"`
		Role     string     `json:"role"`
		AppID    string     `json:"app_id"`
		Apps     [][]string `json:"apps"`
	}
	if json.Unmarshal([]byte(saved), &previous) != nil || json.Unmarshal([]byte(current), &next) != nil {
		return false
	}
	if previous.TenantID != next.TenantID || previous.UserID != next.UserID || previous.Role != next.Role || previous.AppID != next.AppID || previous.AppID != appID {
		return false
	}
	if appID != "" {
		return len(previous.Apps) == 1 && len(next.Apps) == 1 && previous.Apps[0][0] == next.Apps[0][0] && previous.Apps[0][1] == next.Apps[0][1]
	}
	permissions := make(map[string]string, len(next.Apps))
	for _, permission := range next.Apps {
		if len(permission) == 2 {
			permissions[permission[0]] = permission[1]
		}
	}
	for _, permission := range previous.Apps {
		if len(permission) != 2 || permissions[permission[0]] != permission[1] {
			return false
		}
	}
	return true
}

func (s *Server) conversationApp(ctx context.Context, id identity, appID string) bool {
	if appID == "" {
		return true
	}
	app, err := s.PB.Get(ctx, "apps", appID)
	return err == nil && app["tenant_id"] == id.Tenant["id"] && s.appPermission(ctx, app, id) != ""
}

func (s *Server) session(ctx context.Context, id identity, conversationID, appID string) (map[string]any, error) {
	filter := listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "user_id = "+pbFilterString(stringValue(id.User["id"])), "app_id = "+pbFilterString(appID))
	if conversationID == "" {
		return s.PB.Find(ctx, "agent_sessions", filter)
	}
	saved, err := s.PB.Get(ctx, "agent_sessions", conversationID)
	if err != nil || saved["tenant_id"] != id.Tenant["id"] || saved["user_id"] != id.User["id"] || stringValue(saved["app_id"]) != appID {
		return nil, errConversationNotFound
	}
	return saved, nil
}

func (s *Server) listConversations(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	appID := r.URL.Query().Get("app_id")
	if !s.conversationApp(ctx, id, appID) {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	rows, _, _, err := s.PB.List(ctx, "agent_sessions", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "user_id = "+pbFilterString(stringValue(id.User["id"])), "app_id = "+pbFilterString(appID)), "-updated", 1, 100)
	if err != nil {
		writeError(w, 503, "对话列表暂不可用")
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"id": row["id"], "title": row["title"], "app_id": row["app_id"], "revision": row["revision"], "updated_at": row["updated"]})
	}
	writeJSON(w, 200, map[string]any{"items": items})
}

func (s *Server) createConversation(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	input := mapBody(r)
	appID := stringValue(input["app_id"])
	if !s.conversationApp(ctx, id, appID) {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	scope, err := s.conversationScope(ctx, id, appID)
	if err != nil {
		writeError(w, 503, "对话权限范围暂不可用")
		return
	}
	row, err := s.PB.Create(ctx, "agent_sessions", map[string]any{"tenant_id": id.Tenant["id"], "user_id": id.User["id"], "app_id": appID, "title": "", "scope": scope, "revision": 0, "checkpoint": "", "messages": []any{}})
	if err != nil {
		writeError(w, 503, "对话创建失败")
		return
	}
	writeJSON(w, 201, map[string]any{"id": row["id"], "title": row["title"], "app_id": appID, "revision": row["revision"], "updated_at": row["updated"]})
}

func (s *Server) getConversation(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	conversationID, appID := r.URL.Query().Get("conversation_id"), r.URL.Query().Get("app_id")
	if !s.conversationApp(ctx, id, appID) {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	lock := sessionLock("session:" + stringValue(id.Tenant["id"]) + ":" + stringValue(id.User["id"]) + ":" + conversationID)
	lock.Lock()
	defer lock.Unlock()
	saved, err := s.session(ctx, id, conversationID, appID)
	if isMissing(err) || err == errConversationNotFound {
		writeJSON(w, 200, map[string]any{"conversation": nil})
		return
	}
	if err != nil {
		writeError(w, 503, "对话读取暂不可用")
		return
	}
	scope, err := s.conversationScope(ctx, id, appID)
	if err != nil {
		writeError(w, 503, "对话权限范围暂不可用")
		return
	}
	if !compatibleConversationScope(stringValue(saved["scope"]), scope, appID) {
		if err := s.PB.Delete(ctx, "agent_sessions", stringValue(saved["id"])); err != nil {
			writeError(w, 503, "对话清理失败")
			return
		}
		writeJSON(w, 200, map[string]any{"conversation": nil, "changed": true})
		return
	}
	if stringValue(saved["scope"]) != scope {
		if _, err := s.PB.Update(ctx, "agent_sessions", stringValue(saved["id"]), map[string]any{"scope": scope}); err != nil {
			writeError(w, 503, "对话权限范围暂不可用")
			return
		}
		saved["scope"] = scope
	}
	writeJSON(w, 200, map[string]any{"conversation": map[string]any{"id": saved["id"], "app_id": saved["app_id"], "title": saved["title"], "scope": saved["scope"], "revision": saved["revision"], "messages": saved["messages"]}})
}

func (s *Server) saveConversation(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	input := mapBody(r)
	conversationID, appID := stringValue(input["conversation_id"]), stringValue(input["app_id"])
	if !s.conversationApp(ctx, id, appID) {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	lock := sessionLock("session:" + stringValue(id.Tenant["id"]) + ":" + stringValue(id.User["id"]) + ":" + conversationID)
	lock.Lock()
	defer lock.Unlock()
	scope, err := s.conversationScope(ctx, id, appID)
	if err != nil {
		writeError(w, 503, "对话权限范围暂不可用")
		return
	}
	if stringValue(input["scope"]) != scope {
		writeJSON(w, 200, map[string]any{"changed": true, "saved": false})
		return
	}
	saved, err := s.session(ctx, id, conversationID, appID)
	if err != nil {
		writeJSON(w, 200, map[string]any{"changed": true, "saved": false})
		return
	}
	if !compatibleConversationScope(stringValue(saved["scope"]), scope, appID) {
		writeJSON(w, 200, map[string]any{"changed": true, "saved": false})
		return
	}
	revision := intValue(saved["revision"])
	if intValue(input["expected_revision"]) != revision {
		writeJSON(w, 200, map[string]any{"conflict": true, "saved": false})
		return
	}
	messages, ok := input["messages"].([]any)
	if !ok || len(messages) > 240 {
		writeError(w, 400, "会话内容无效或超过保存限制")
		return
	}
	raw, _ := json.Marshal(messages)
	if len(raw) > 262144 {
		writeError(w, 400, "会话内容无效或超过保存限制")
		return
	}
	title := stringValue(saved["title"])
	for _, item := range messages {
		message := asMap(item)
		if message["role"] != "user" && message["role"] != "assistant" {
			writeError(w, 400, "会话内容无效或超过保存限制")
			return
		}
		content, ok := message["content"].(string)
		if !ok {
			writeError(w, 400, "会话内容无效或超过保存限制")
			return
		}
		if title == "" && message["role"] == "user" {
			title = clip(strings.TrimSpace(content), 80)
		}
	}
	_, err = s.PB.Update(ctx, "agent_sessions", stringValue(saved["id"]), map[string]any{"scope": scope, "revision": revision + 1, "checkpoint": "", "messages": messages, "title": title})
	if err != nil {
		writeError(w, 503, "对话保存失败")
		return
	}
	writeJSON(w, 200, map[string]any{"saved": true, "revision": revision + 1, "title": title})
}

func (s *Server) conversationByID(ctx context.Context, r *http.Request) (map[string]any, bool) {
	id := who(r)
	row, err := s.PB.Get(ctx, "agent_sessions", pathID(r, "conversationId"))
	if err != nil || row["tenant_id"] != id.Tenant["id"] || row["user_id"] != id.User["id"] {
		return nil, false
	}
	if !s.conversationApp(ctx, id, stringValue(row["app_id"])) {
		return nil, false
	}
	return row, true
}

func (s *Server) renameConversation(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	row, ok := s.conversationByID(ctx, r)
	if !ok {
		writeError(w, 404, "对话不存在")
		return
	}
	title := strings.TrimSpace(stringValue(mapBody(r)["title"]))
	if title == "" || len([]rune(title)) > 80 {
		writeError(w, 400, "对话名称需为 1–80 个字符")
		return
	}
	if _, err := s.PB.Update(ctx, "agent_sessions", stringValue(row["id"]), map[string]any{"title": title}); err != nil {
		writeError(w, 503, "对话重命名失败")
		return
	}
	writeJSON(w, 200, map[string]any{"id": row["id"], "title": title})
}

func (s *Server) deleteConversation(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	row, ok := s.conversationByID(ctx, r)
	if !ok {
		writeJSON(w, 200, map[string]any{"ok": true})
		return
	}
	var messages []map[string]any
	raw, _ := json.Marshal(row["messages"])
	_ = json.Unmarshal(raw, &messages)
	for _, message := range messages {
		runID := stringValue(message["run_id"])
		if runID == "" {
			continue
		}
		run, err := s.PB.Get(ctx, "miao_harness_runs", runID)
		if err == nil && !containsString([]string{"completed", "failed", "cancelled", "budget_exhausted", "unsupported", "unavailable"}, stringValue(run["state"])) {
			writeError(w, 409, "会话仍有关联运行，结束运行后再删除")
			return
		}
	}
	if err := s.PB.Delete(ctx, "agent_sessions", stringValue(row["id"])); err != nil {
		writeError(w, 503, "对话删除失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) clearConversation(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	conversationID, appID := r.URL.Query().Get("conversation_id"), r.URL.Query().Get("app_id")
	if !s.conversationApp(ctx, id, appID) {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if saved, err := s.session(ctx, id, conversationID, appID); err == nil {
		if err := s.PB.Delete(ctx, "agent_sessions", stringValue(saved["id"])); err != nil {
			writeError(w, 503, "对话清理失败")
			return
		}
	} else if !isMissing(err) && err != errConversationNotFound {
		writeError(w, 503, "对话读取暂不可用")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) getAppContext(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	writeJSON(w, 200, map[string]any{"content": app["business_context"], "revision": app["context_revision"]})
}
func (s *Server) saveAppContext(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(role) || boolValue(app["archived"]) {
		writeError(w, 403, "没有应用管理权限")
		return
	}
	input := mapBody(r)
	content, ok := input["content"].(string)
	if !ok || len(content) > 16000 {
		writeError(w, 400, "业务说明最多 16000 字")
		return
	}
	revision := intValue(app["context_revision"])
	if intValue(input["expected_revision"]) != revision {
		writeError(w, 409, "业务说明已变化，请读取后重试")
		return
	}
	revision++
	_, err = s.PB.Update(ctx, "apps", stringValue(app["id"]), map[string]any{"business_context": content, "context_revision": revision})
	if err != nil {
		writeError(w, 503, "业务说明保存失败")
		return
	}
	writeJSON(w, 200, map[string]any{"content": content, "revision": revision})
}

func (s *Server) recordChanges(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	id := who(r)
	parts := []string{"tenant_id = " + pbFilterString(stringValue(id.Tenant["id"])), "app_id = " + pbFilterString(stringValue(app["id"]))}
	if recordID := r.URL.Query().Get("record_id"); recordID != "" {
		parts = append(parts, "record_id = "+pbFilterString(recordID))
	}
	page := queryInt(r, "page", 1, 1, 100000)
	rows, total, pages, err := s.PB.List(ctx, "miao_record_changes", listFilter(parts...), "-created", page, 25)
	if err != nil {
		writeError(w, 503, "修改历史暂时不可用")
		return
	}
	writeJSON(w, 200, pageResult(rows, page, 25, total))
	_ = pages
}
func (s *Server) restoreRecordChange(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if role == "viewer" || boolValue(app["archived"]) {
		writeError(w, 403, "没有修改权限")
		return
	}
	input := mapBody(r)
	if input["confirm"] != true || stringValue(input["expected_updated_at"]) == "" {
		writeError(w, 400, "先读取当前记录并确认恢复")
		return
	}
	id := who(r)
	change, err := s.PB.Get(ctx, "miao_record_changes", pathID(r, "changeId"))
	if err != nil || change["tenant_id"] != id.Tenant["id"] || change["app_id"] != app["id"] {
		writeError(w, 404, "修改记录不存在")
		return
	}
	table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(stringValue(change["table"]))))
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	row, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), stringValue(change["record_id"]))
	if err != nil {
		writeError(w, 404, "记录不存在")
		return
	}
	before, after := asMap(change["before"]), asMap(change["after"])
	fields := asSliceMap(table["fields"])
	data := map[string]any{}
	for _, field := range fields {
		if field["type"] == "file" {
			continue
		}
		name := stringValue(field["name"])
		if !equalJSON(before[name], after[name]) {
			if !equalJSON(row[name], after[name]) {
				writeError(w, 409, "目标字段已有后续变化或无可恢复字段，请重新检查")
				return
			}
			data[name] = before[name]
		}
	}
	if len(data) == 0 {
		writeError(w, 409, "目标字段已有后续变化或无可恢复字段，请重新检查")
		return
	}
	saved, err := s.saveBusinessRecord(ctx, recordWrite{Actor: id.actor(stringValue(app["id"]), "restore"), Table: stringValue(table["slug"]), RecordID: stringValue(row["id"]), ExpectedUpdated: stringValue(input["expected_updated_at"]), Data: data}, nil)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"status": "restored", "record": publicRecord(saved), "note": "产生新的修改记录；附件、删除与表结构不回滚"})
}

var _ = fmt.Sprintf
var _ = context.Canceled
