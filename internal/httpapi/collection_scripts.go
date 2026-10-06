package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

const (
	collectionScriptMaxPages    = 20
	collectionScriptMaxItems    = 1000
	collectionScriptMaxRequests = 30
)

// routesCollectionScripts is intentionally separate from routes() so deployments
// can roll out the capability independently of the existing task API.
func (s *Server) routesCollectionScripts() {
	s.Mux.HandleFunc("GET /api/apps/{id}/collection-scripts", s.auth(s.listCollectionScripts))
	s.Mux.HandleFunc("POST /api/apps/{id}/collection-scripts", s.auth(s.createCollectionScript))
	s.Mux.HandleFunc("PATCH /api/apps/{id}/collection-scripts/{scriptId}", s.auth(s.updateCollectionScript))
	for _, action := range []string{"enable", "pause", "preview", "run"} {
		s.Mux.HandleFunc("POST /api/apps/{id}/collection-scripts/{scriptId}/"+action, s.auth(s.collectionScriptAction))
	}
	s.Mux.HandleFunc("GET /api/apps/{id}/collection-scripts/{scriptId}/runs", s.auth(s.listCollectionScriptRuns))
	s.Mux.HandleFunc("GET /api/apps/{id}/collection-scripts/{scriptId}/runs/{runId}", s.auth(s.getCollectionScriptRun))
}

func collectionScriptSchedule(definition map[string]any) map[string]any {
	return asMap(definition["schedule"])
}

func collectionScriptPublic(row map[string]any) map[string]any {
	definition := asMap(row["definition"])
	return map[string]any{
		"id": row["id"], "name": row["name"], "status": row["status"],
		"pause_reason": defaultString(stringValue(row["pause_reason"]), ""),
		"revision":     row["revision"], "version": row["revision"],
		"schedule": collectionScriptSchedule(definition), "source": definition["source"],
		"target": definition["target"], "recipients": definition["recipients"],
		"definition": definition,
	}
}

func collectionScriptRunPublic(row map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range row {
		if k != "snapshot" {
			out[k] = v
		}
	}
	snapshot := asMap(row["snapshot"])
	out["version"] = snapshot["version"]
	out["schedule"] = snapshot["schedule"]
	out["source"] = snapshot["source"]
	out["target"] = snapshot["target"]
	out["counts"] = row["counts"]
	out["errors"] = row["errors"]
	return out
}

func (s *Server) collectionScriptForRequest(ctx context.Context, r *http.Request, manage bool) (map[string]any, map[string]any, bool) {
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, nil, false
	}
	if manage && (!canPublishAppRole(role) || boolValue(app["archived"])) {
		return app, nil, false
	}
	script, err := s.PB.Get(ctx, "collection_scripts", pathID(r, "scriptId"))
	id := who(r)
	if err != nil || script["tenant_id"] != id.Tenant["id"] || script["app_id"] != app["id"] {
		return app, nil, false
	}
	if manage && script["created_by"] != id.User["id"] && s.appPermission(ctx, app, id) != "owner" {
		return app, nil, false
	}
	return app, script, true
}

func (s *Server) listCollectionScripts(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	rows, err := s.PB.ListAll(ctx, "collection_scripts", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "status != \"archived\""), "-updated")
	if err != nil {
		writeError(w, 503, "采集脚本暂不可用")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, collectionScriptPublic(row))
	}
	writeJSON(w, 200, out)
}

// collectionScriptMutationActor re-checks the actor and app inside the same
// transaction that changes the script. The request snapshot is only a hint;
// membership, app role, archive state, and script ownership must still hold at
// the write boundary.
func (s *Server) collectionScriptMutationActor(ctx context.Context, pb *pocketbase.Client, actor executionActor, script map[string]any) (map[string]any, appRole, error) {
	id, err := s.workspaceActor(ctx, pb, actor)
	if err != nil {
		return nil, "", err
	}
	app, err := pb.Get(ctx, "apps", actor.AppID)
	if err != nil {
		return nil, "", err
	}
	access, err := applicationAccess(ctx, pb, app, id)
	if err != nil {
		return nil, "", err
	}
	if boolValue(app["archived"]) || !access.Role.canPublish() {
		return nil, "", businessError(403, "需要当前应用的发布权限，且应用未归档")
	}
	if script != nil && stringValue(script["created_by"]) != actor.UserID && access.Role != "owner" {
		return nil, "", businessError(403, "只有脚本负责人或应用所有者可以修改采集脚本")
	}
	return app, access.Role, nil
}

func (s *Server) createCollectionScript(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canPublishAppRole(role) || boolValue(app["archived"]) {
		writeError(w, 403, "需要当前应用的发布权限，且应用未归档")
		return
	}
	input := mapBody(r)
	name := clip(strings.TrimSpace(stringValue(input["name"])), 160)
	if name == "" {
		writeError(w, 400, "脚本名称不能为空")
		return
	}
	id := who(r)
	var script map[string]any
	err = s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		actor := id.actor(stringValue(app["id"]), "interactive")
		freshApp, _, e := s.collectionScriptMutationActor(ctx, tx, actor, nil)
		if e != nil {
			return e
		}
		definition, msg := normalizeCollectionScriptDefinitionWith(ctx, tx, stringValue(freshApp["tenant_id"]), stringValue(freshApp["id"]), input["definition"])
		if msg != "" {
			return businessError(400, msg)
		}
		script, e = tx.Create(ctx, "collection_scripts", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "created_by": id.User["id"], "name": name, "revision": 1, "definition": definition, "status": "draft", "pause_reason": "", "next_run_at": ""})
		if e != nil {
			return e
		}
		_, e = tx.Create(ctx, "collection_script_versions", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "script_id": script["id"], "version": 1, "created_by": id.User["id"], "definition": definition})
		return e
	})
	if err != nil {
		if errStatus(err) >= 500 {
			writeError(w, 503, "采集脚本创建失败")
		} else {
			writeError(w, errStatus(err), err.Error())
		}
		return
	}
	writeJSON(w, 201, collectionScriptPublic(script))
}

func (s *Server) updateCollectionScript(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, script, ok := s.collectionScriptForRequest(ctx, r, true)
	if !ok {
		writeError(w, 404, "采集脚本不存在或没有权限")
		return
	}
	input := mapBody(r)
	if !containsString([]string{"draft", "paused"}, stringValue(script["status"])) || intValue(input["expected_revision"]) != intValue(script["revision"]) {
		writeError(w, 409, "请先暂停脚本，并读取最新版本后修改")
		return
	}
	name := clip(strings.TrimSpace(defaultString(stringValue(input["name"]), stringValue(script["name"]))), 160)
	if name == "" {
		writeError(w, 400, "脚本名称不能为空")
		return
	}
	var saved map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		fresh, err := tx.Get(ctx, "collection_scripts", stringValue(script["id"]))
		if err != nil || fresh["tenant_id"] != script["tenant_id"] || fresh["app_id"] != app["id"] {
			return businessError(409, "采集脚本已失效，请重新读取")
		}
		actor := who(r).actor(stringValue(app["id"]), "interactive")
		freshApp, _, err := s.collectionScriptMutationActor(ctx, tx, actor, fresh)
		if err != nil {
			return err
		}
		if !containsString([]string{"draft", "paused"}, stringValue(fresh["status"])) || intValue(input["expected_revision"]) != intValue(fresh["revision"]) {
			return businessError(409, "请先暂停脚本，并读取最新版本后修改")
		}
		definition, msg := normalizeCollectionScriptDefinitionWith(ctx, tx, stringValue(freshApp["tenant_id"]), stringValue(freshApp["id"]), input["definition"])
		if msg != "" {
			return businessError(400, msg)
		}
		version := intValue(fresh["revision"]) + 1
		saved, err = tx.Update(ctx, "collection_scripts", stringValue(fresh["id"]), map[string]any{"name": name, "definition": definition, "revision": version, "status": "draft", "pause_reason": "", "next_run_at": ""})
		if err != nil {
			return err
		}
		_, err = tx.Create(ctx, "collection_script_versions", map[string]any{"tenant_id": fresh["tenant_id"], "app_id": fresh["app_id"], "script_id": fresh["id"], "version": version, "created_by": who(r).User["id"], "definition": definition})
		return err
	})
	if err != nil {
		if errStatus(err) >= 500 {
			writeError(w, 503, "采集脚本更新失败")
		} else {
			writeError(w, errStatus(err), err.Error())
		}
		return
	}
	writeJSON(w, 200, collectionScriptPublic(saved))
}

func (s *Server) collectionScriptAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	action := pathAction(r)
	_, script, ok := s.collectionScriptForRequest(ctx, r, true)
	if !ok {
		writeError(w, 404, "采集脚本不存在或没有权限")
		return
	}
	input := mapBody(r)
	switch action {
	case "enable":
		if input["confirm"] != true || intValue(input["expected_revision"]) != intValue(script["revision"]) {
			writeError(w, 409, "请审阅并确认脚本的最新具体版本")
			return
		}
		var saved map[string]any
		err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
			fresh, err := tx.Get(ctx, "collection_scripts", stringValue(script["id"]))
			if err != nil || fresh["tenant_id"] != who(r).Tenant["id"] || fresh["app_id"] != script["app_id"] || intValue(fresh["revision"]) != intValue(input["expected_revision"]) {
				return businessError(409, "采集脚本版本已变化，请重新读取")
			}
			if _, err := s.collectionScriptAuthorityWith(ctx, tx, fresh); err != nil {
				return err
			}
			definition := asMap(fresh["definition"])
			next := nextScheduledRun(collectionScriptSchedule(definition), time.Now())
			if schedule := collectionScriptSchedule(definition); stringValue(schedule["type"]) == "once" && next == "" {
				return businessError(400, "一次性时间已过，请修改后重新确认")
			}
			saved, err = tx.Update(ctx, "collection_scripts", stringValue(fresh["id"]), map[string]any{"status": "enabled", "next_run_at": next, "pause_reason": ""})
			return err
		})
		if err != nil {
			writeError(w, errStatus(err), err.Error())
			return
		}
		writeJSON(w, 200, collectionScriptPublic(saved))
	case "pause":
		if intValue(input["expected_revision"]) != intValue(script["revision"]) {
			writeError(w, 409, "采集脚本版本已变化，请重新读取后操作")
			return
		}
		var saved map[string]any
		err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
			fresh, err := tx.Get(ctx, "collection_scripts", stringValue(script["id"]))
			if err != nil || fresh["tenant_id"] != who(r).Tenant["id"] || fresh["app_id"] != script["app_id"] || intValue(fresh["revision"]) != intValue(input["expected_revision"]) {
				return businessError(409, "采集脚本版本已变化，请重新读取")
			}
			if _, err := s.collectionScriptAuthorityWith(ctx, tx, fresh); err != nil {
				return err
			}
			saved, err = tx.Update(ctx, "collection_scripts", stringValue(fresh["id"]), map[string]any{"status": "paused", "next_run_at": "", "pause_reason": "用户暂停；已创建的运行可单独查看"})
			return err
		})
		if err != nil {
			writeError(w, errStatus(err), err.Error())
			return
		}
		writeJSON(w, 200, collectionScriptPublic(saved))
	case "preview", "run":
		if intValue(input["expected_revision"]) != intValue(script["revision"]) || action == "run" && (script["status"] != "enabled" || input["confirm"] != true) {
			writeError(w, 409, "读取并确认当前脚本版本后再试运行")
			return
		}
		fresh, err := s.PB.Get(ctx, "collection_scripts", stringValue(script["id"]))
		if err != nil || fresh["tenant_id"] != who(r).Tenant["id"] || fresh["app_id"] != script["app_id"] || intValue(fresh["revision"]) != intValue(input["expected_revision"]) {
			writeError(w, 409, "采集脚本版本已变化，请重新读取")
			return
		}
		if _, err := s.collectionScriptAuthority(ctx, fresh); err != nil {
			writeError(w, errStatus(err), err.Error())
			return
		}
		script = fresh
		key := strings.TrimSpace(stringValue(input["request_id"]))
		if key == "" {
			key, _ = randomToken()
		}
		if !requestIDPattern.MatchString(key) {
			writeError(w, 400, "请求标识无效")
			return
		}
		mode := "live"
		if action == "preview" {
			mode = "preview"
		}
		run, err := s.executeCollectionScript(ctx, script, mode, "manual:"+key)
		if err != nil {
			writeError(w, errStatus(err), err.Error())
			return
		}
		writeJSON(w, 202, collectionScriptRunPublic(run))
	default:
		writeError(w, 404, "脚本动作不存在")
	}
}

func (s *Server) listCollectionScriptRuns(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	_, script, ok := s.collectionScriptForRequest(ctx, r, false)
	if !ok {
		writeError(w, 404, "采集脚本不存在或没有权限")
		return
	}
	page := queryInt(r, "page", 1, 1, 10000)
	rows, total, _, err := s.PB.List(ctx, "collection_script_runs", listFilter("tenant_id = "+pbFilterString(stringValue(script["tenant_id"])), "app_id = "+pbFilterString(stringValue(script["app_id"])), "script_id = "+pbFilterString(stringValue(script["id"]))), "-created", page, 25)
	if err != nil {
		writeError(w, 503, "采集脚本运行记录暂不可用")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, collectionScriptRunPublic(row))
	}
	writeJSON(w, 200, pageResult(out, page, 25, total))
}

func (s *Server) getCollectionScriptRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	_, script, ok := s.collectionScriptForRequest(ctx, r, false)
	if !ok {
		writeError(w, 404, "采集脚本不存在或没有权限")
		return
	}
	run, err := s.PB.Get(ctx, "collection_script_runs", pathID(r, "runId"))
	if err != nil || run["tenant_id"] != script["tenant_id"] || run["app_id"] != script["app_id"] || run["script_id"] != script["id"] {
		writeError(w, 404, "运行记录不存在")
		return
	}
	writeJSON(w, 200, collectionScriptRunPublic(run))
}

// collectionScriptScheduleDefinition validates the bounded schedule and returns
// the same trigger shape used by the existing scheduler.
func normalizeCollectionScriptSchedule(raw any) (map[string]any, string) {
	input := asMap(raw)
	if len(input) == 0 {
		return nil, "必须提供 schedule"
	}
	typ := stringValue(input["type"])
	if !containsString([]string{"manual", "once", "daily", "weekly"}, typ) {
		return nil, "schedule.type 无效"
	}
	tz := defaultString(stringValue(input["timezone"]), "Asia/Shanghai")
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, "schedule.timezone 无效"
	}
	out := map[string]any{"type": typ, "timezone": tz}
	switch typ {
	case "once":
		at := stringValue(input["at"])
		if !offsetPattern.MatchString(at) {
			return nil, "一次性 schedule.at 必须带时区"
		}
		parsed, err := time.Parse(time.RFC3339, at)
		if err != nil {
			return nil, "一次性 schedule.at 无效"
		}
		out["at"] = parsed.UTC().Format(time.RFC3339Nano)
	case "daily", "weekly":
		tm := stringValue(input["time"])
		if !hhmmPattern.MatchString(tm) {
			return nil, "schedule.time 格式为 HH:mm"
		}
		out["time"] = tm
		if typ == "weekly" {
			days := []int{}
			seen := map[int]bool{}
			for _, rawDay := range anySlice(input["weekdays"]) {
				day, ok := rawDay.(float64)
				if !ok {
					if integer, integerOK := rawDay.(int); integerOK {
						day, ok = float64(integer), true
					}
				}
				if !ok || day < 0 || day > 6 || day != float64(int(day)) {
					return nil, "schedule.weekdays 使用 0–6 的整数"
				}
				if seen[int(day)] {
					continue
				}
				seen[int(day)] = true
				days = append(days, int(day))
			}
			if len(days) == 0 {
				return nil, "weekly schedule 至少需要一个 weekdays"
			}
			out["weekdays"] = days
		}
	}
	return out, ""
}

func collectionScriptPath(value string) bool {
	return value != "" && strings.HasPrefix(value, "/") && !strings.Contains(value, "..") && !strings.ContainsAny(value, "#") && len(value) <= 1000
}

func normalizeCollectionScriptDefinition(ctx context.Context, s *Server, tenantID, appID string, raw any) (map[string]any, string) {
	return normalizeCollectionScriptDefinitionWith(ctx, s.PB, tenantID, appID, raw)
}

func normalizeCollectionScriptDefinitionWith(ctx context.Context, pb *pocketbase.Client, tenantID, appID string, raw any) (map[string]any, string) {
	input := asMap(raw)
	if len(input) == 0 {
		return nil, "脚本定义必须是对象"
	}
	source := asMap(input["source"])
	connectorID, sourcePath := stringValue(source["connector_id"]), stringValue(source["path"])
	if connectorID == "" || !collectionScriptPath(sourcePath) {
		return nil, "source 必须提供 connector_id 和安全 path"
	}
	connector, err := pb.Get(ctx, "connectors", connectorID)
	if err != nil || connector["tenant_id"] != tenantID || connector["app_id"] != appID || connector["status"] != "enabled" {
		return nil, "source.connector_id 必须是当前应用中已启用的连接器"
	}
	u, err := connectorURL(asMap(connector["definition"]), sourcePath)
	if err != nil || !connectorPathAllowed(asMap(connector["definition"]), u.Path) {
		return nil, "source.path 不在连接器允许范围内"
	}
	safeSource := map[string]any{"connector_id": connectorID, "path": sourcePath}
	pagination := asMap(source["pagination"])
	if len(pagination) > 0 {
		maxPages, maxItems, maxRequests := intValue(pagination["max_pages"]), intValue(pagination["max_items"]), intValue(pagination["max_requests"])
		if maxPages == 0 {
			maxPages = 1
		}
		if maxItems == 0 {
			maxItems = collectionScriptMaxItems
		}
		if maxRequests == 0 {
			maxRequests = collectionScriptMaxRequests
		}
		if maxPages < 1 || maxPages > collectionScriptMaxPages || maxItems < 1 || maxItems > collectionScriptMaxItems || maxRequests < 1 || maxRequests > collectionScriptMaxRequests {
			return nil, "pagination 限制超出范围"
		}
		paths := []string{}
		for _, rawPath := range anySlice(pagination["paths"]) {
			path := stringValue(rawPath)
			if !collectionScriptPath(path) {
				return nil, "pagination.paths 必须是安全路径"
			}
			parsed, e := connectorURL(asMap(connector["definition"]), path)
			if e != nil || !connectorPathAllowed(asMap(connector["definition"]), parsed.Path) {
				return nil, "pagination.paths 不在连接器允许范围内"
			}
			paths = append(paths, path)
		}
		if len(paths) > maxPages {
			return nil, "pagination.paths 超过 max_pages"
		}
		safePagination := map[string]any{"max_pages": maxPages, "max_items": maxItems, "max_requests": maxRequests}
		if len(paths) > 0 {
			safePagination["paths"] = uniqueStrings(paths, maxPages)
		}
		safeSource["pagination"] = safePagination
	}
	if detail := asMap(source["detail"]); len(detail) > 0 {
		pathTemplate := defaultString(stringValue(detail["path_template"]), stringValue(detail["path"]))
		pathField := defaultString(stringValue(detail["path_field"]), "id")
		if !collectionScriptPath(pathTemplate) || !strings.Contains(pathTemplate, "{value}") || !validInputName(pathField) {
			return nil, "detail 需要安全 path_template（含 {value}）和 path_field"
		}
		maxRequests := intValue(detail["max_requests"])
		if maxRequests == 0 {
			maxRequests = collectionScriptMaxRequests
		}
		if maxRequests < 1 || maxRequests > collectionScriptMaxRequests {
			return nil, "detail.max_requests 超出范围"
		}
		safeDetail := map[string]any{"path_template": pathTemplate, "path_field": pathField, "max_requests": maxRequests}
		if rawExtract, ok := detail["extract"]; ok && rawExtract != nil {
			extract, extractMsg := normalizeConnectorExtract(rawExtract)
			if extractMsg != "" {
				return nil, "detail.extract: " + extractMsg
			}
			safeDetail["extract"] = extract
		}
		safeSource["detail"] = safeDetail
	}

	target := asMap(input["target"])
	tableName := stringValue(target["table"])
	if tableName == "" {
		return nil, "target.table 必须存在"
	}
	table, err := pb.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID), "slug = "+pbFilterString(tableName)))
	if err != nil {
		return nil, "target.table 不存在"
	}
	fields := asSliceMap(table["fields"])
	fieldDefs := map[string]map[string]any{}
	for _, field := range fields {
		fieldDefs[stringValue(field["name"])] = field
	}
	mapping := asMap(target["fields"])
	if len(mapping) < 1 || len(mapping) > 24 {
		return nil, "target.fields 需要 1–24 个字段"
	}
	safeMapping := map[string]any{}
	for targetField, rawMapping := range mapping {
		field := fieldDefs[targetField]
		if field == nil || stringValue(field["type"]) == "file" || stringValue(field["type"]) == "relation" {
			return nil, "target.fields 只能写入当前表的普通字段"
		}
		mappingItem := asMap(rawMapping)
		from := stringValue(mappingItem["from"])
		if from == "" {
			from = stringValue(rawMapping)
		}
		if from == "" || !validInputName(from) && !validJSONPointer(from) {
			return nil, "字段映射 from 无效"
		}
		typ := defaultString(stringValue(mappingItem["type"]), stringValue(field["type"]))
		if !containsString([]string{"text", "number", "bool", "date", "email", "url", "select"}, typ) {
			return nil, "字段映射 type 无效"
		}
		safeMapping[targetField] = map[string]any{"from": from, "type": typ}
	}
	safeTarget := map[string]any{"table": tableName, "fields": safeMapping}

	filter := []any{}
	for _, rawFilter := range anySlice(input["filters"]) {
		item := asMap(rawFilter)
		field := stringValue(item["field"])
		op := stringValue(item["operator"])
		if !validInputName(field) || !containsString([]string{"eq", "neq", "contains", "gt", "gte", "lt", "lte", "is_empty", "not_empty"}, op) || (op != "is_empty" && op != "not_empty" && item["value"] == nil) {
			return nil, "filters 只能使用受限字段和比较符"
		}
		filter = append(filter, map[string]any{"field": field, "operator": op, "value": item["value"]})
	}
	if len(filter) > 12 {
		return nil, "filters 最多 12 条"
	}
	dedup := asMap(input["dedup"])
	keyFields := []string{}
	for _, rawField := range anySlice(dedup["fields"]) {
		field := stringValue(rawField)
		if !validInputName(field) && !validJSONPointer(field) {
			return nil, "dedup.fields 无效"
		}
		keyFields = append(keyFields, field)
	}
	if len(keyFields) == 0 {
		if field := stringValue(input["dedup_key"]); field != "" {
			keyFields = []string{field}
		}
	}
	keyFields = uniqueStrings(keyFields, 8)
	if len(keyFields) == 0 {
		return nil, "必须提供 1–8 个稳定 dedup 字段"
	}
	changePolicy := defaultString(stringValue(dedup["on_change"]), "update")
	if !containsString([]string{"update", "skip"}, changePolicy) {
		return nil, "dedup.on_change 必须是 update 或 skip"
	}
	safeDedup := map[string]any{"fields": keyFields, "on_change": changePolicy}
	recipients := uniqueStrings(input["recipients"], 10)
	if len(recipients) == 0 {
		recipients = uniqueStrings(asMap(input["notifications"])["recipient_ids"], 10)
	}
	if len(recipients) == 0 {
		return nil, "必须明确 1–10 个 recipient IDs"
	}
	for _, uid := range recipients {
		if _, err := pb.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(tenantID), "user_id = "+pbFilterString(uid))); err != nil {
			return nil, "recipient 必须是当前工作区成员"
		}
	}
	baseline := defaultString(stringValue(input["baseline"]), "notify")
	if !containsString([]string{"notify", "silent"}, baseline) {
		return nil, "baseline 必须是 notify 或 silent"
	}
	schedule, msg := normalizeCollectionScriptSchedule(input["schedule"])
	if msg != "" {
		return nil, msg
	}
	return map[string]any{"schema_version": 1, "source": safeSource, "target": safeTarget, "filters": filter, "dedup": safeDedup, "recipients": recipients, "baseline": baseline, "schedule": schedule}, ""
}

func collectionScriptValue(row map[string]any, field string) (any, bool) {
	if strings.HasPrefix(field, "/") {
		return jsonPointer(row, field)
	}
	value, ok := row[field]
	return value, ok
}

func collectionScriptFilterMatch(row map[string]any, filters []any) bool {
	for _, raw := range filters {
		item := asMap(raw)
		value, present := collectionScriptValue(row, stringValue(item["field"]))
		op := stringValue(item["operator"])
		want := item["value"]
		switch op {
		case "is_empty":
			if present && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
				return false
			}
		case "not_empty":
			if !present || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
				return false
			}
		case "eq":
			if !present || fmt.Sprint(value) != fmt.Sprint(want) {
				return false
			}
		case "neq":
			if present && fmt.Sprint(value) == fmt.Sprint(want) {
				return false
			}
		case "contains":
			if !strings.Contains(strings.ToLower(fmt.Sprint(value)), strings.ToLower(fmt.Sprint(want))) {
				return false
			}
		case "gt", "gte", "lt", "lte":
			left := floatValue(value)
			right := floatValue(want)
			if left == 0 && fmt.Sprint(value) != "0" {
				left, _ = strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
			}
			if right == 0 && fmt.Sprint(want) != "0" {
				right, _ = strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(want)), 64)
			}
			if (op == "gt" && !(left > right)) || (op == "gte" && !(left >= right)) || (op == "lt" && !(left < right)) || (op == "lte" && !(left <= right)) {
				return false
			}
		}
	}
	return true
}

func collectionScriptConvert(value any, typ string) (any, bool) {

	if value == nil {
		return nil, true
	}
	switch typ {
	case "text", "date", "email", "url", "select":
		return fmt.Sprint(value), true
	case "number":
		if n, ok := value.(float64); ok {
			return n, true
		}
		if n, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64); err == nil {
			return n, true
		}
	case "bool":
		if b, ok := value.(bool); ok {
			return b, true
		}
		switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		}
	}
	return nil, false
}

func collectionScriptShouldSkip(item, row map[string]any) bool {
	return len(item) > 0 && stringValue(item["status"]) == "written" && equalJSON(asMap(item["source"]), row)
}

func collectionScriptDedupKey(row map[string]any, fields []string) string {
	values := make([]any, 0, len(fields))
	for _, field := range fields {
		value, _ := collectionScriptValue(row, field)
		values = append(values, value)
	}
	encoded, _ := json.Marshal(values)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func (s *Server) collectionScriptAuthority(ctx context.Context, script map[string]any) (map[string]any, error) {
	return s.collectionScriptAuthorityWith(ctx, s.PB, script)
}

func (s *Server) collectionScriptAuthorityWith(ctx context.Context, pb *pocketbase.Client, script map[string]any) (map[string]any, error) {
	app, err := pb.Get(ctx, "apps", stringValue(script["app_id"]))
	if err != nil || app["tenant_id"] != script["tenant_id"] || boolValue(app["archived"]) {
		return nil, businessError(403, "应用已归档或不存在")
	}
	user, err := pb.Get(ctx, "users", stringValue(script["created_by"]))
	if err != nil || boolValue(user["disabled"]) {
		return nil, businessError(403, "脚本负责人已失去权限")
	}
	tenant, err := pb.Get(ctx, "tenants", stringValue(script["tenant_id"]))
	if err != nil || tenant["id"] != app["tenant_id"] {
		return nil, businessError(403, "脚本工作区不存在")
	}
	member, err := pb.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(stringValue(script["tenant_id"])), "user_id = "+pbFilterString(stringValue(script["created_by"]))))
	if err != nil {
		return nil, businessError(403, "脚本负责人已离开工作区")
	}
	id := identity{User: user, Tenant: tenant, Membership: member}
	access, err := applicationAccess(ctx, pb, app, id)
	if err != nil || !access.Role.canPublish() {
		return nil, businessError(403, "脚本负责人已失去应用权限")
	}
	return app, nil
}

func (s *Server) executeCollectionScript(ctx context.Context, script map[string]any, mode, eventKey string) (map[string]any, error) {
	if mode == "preview" {
		definition := asMap(script["definition"])
		snapshot := map[string]any{"version": script["revision"], "source": definition["source"], "target": definition["target"], "filters": definition["filters"], "dedup": definition["dedup"], "recipients": definition["recipients"], "baseline": definition["baseline"], "schedule": definition["schedule"], "name": script["name"]}
		run := map[string]any{"id": "preview", "snapshot": snapshot}
		result, err := s.collectCollectionScript(ctx, script, run, mode)
		if err != nil {
			return map[string]any{"status": "failed", "mode": mode, "result": result, "error": err.Error()}, nil
		}
		return map[string]any{"status": "completed", "mode": mode, "snapshot": snapshot, "result": result, "counts": result["counts"]}, nil
	}
	var run map[string]any
	created := false
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		fresh, err := tx.Get(ctx, "collection_scripts", stringValue(script["id"]))
		if err != nil || fresh["tenant_id"] != script["tenant_id"] || fresh["app_id"] != script["app_id"] {
			return businessError(409, "采集脚本已失效，请重新读取")
		}
		if previous, findErr := tx.Find(ctx, "collection_script_runs", listFilter("script_id = "+pbFilterString(stringValue(fresh["id"])), "event_key = "+pbFilterString(eventKey))); findErr == nil {
			run, script = previous, fresh
			return nil
		} else if !isMissing(findErr) {
			return findErr
		}
		if stringValue(fresh["status"]) != "enabled" {
			return businessError(409, "采集脚本已暂停，请重新读取后运行")
		}
		if _, err := s.collectionScriptAuthorityWith(ctx, tx, fresh); err != nil {
			return err
		}
		definition := asMap(fresh["definition"])
		snapshot := map[string]any{"version": fresh["revision"], "source": definition["source"], "target": definition["target"], "filters": definition["filters"], "dedup": definition["dedup"], "recipients": definition["recipients"], "baseline": definition["baseline"], "schedule": definition["schedule"], "name": fresh["name"]}
		run, err = tx.Create(ctx, "collection_script_runs", map[string]any{"tenant_id": fresh["tenant_id"], "app_id": fresh["app_id"], "script_id": fresh["id"], "version": fresh["revision"], "created_by": fresh["created_by"], "event_key": eventKey, "mode": mode, "status": "running", "snapshot": snapshot, "counts": map[string]any{"pages": 0, "items": 0, "filtered": 0, "written": 0, "skipped": 0, "notifications": 0}, "errors": []any{}, "started_at": nowISO()})
		if err != nil {
			return err
		}
		created = true
		script = fresh
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !created {
		return run, nil
	}
	result, runErr := s.collectCollectionScript(ctx, script, run, mode)
	if runErr != nil {
		return s.finishCollectionScriptRun(ctx, run, "failed", result, []string{runErr.Error()})
	}
	return s.finishCollectionScriptRun(ctx, run, "completed", result, nil)

}
func (s *Server) collectCollectionScript(ctx context.Context, script, run map[string]any, mode string) (map[string]any, error) {
	definition := asMap(run["snapshot"])
	source := asMap(definition["source"])
	connector, err := s.PB.Get(ctx, "connectors", stringValue(source["connector_id"]))
	if err != nil || connector["tenant_id"] != script["tenant_id"] || connector["app_id"] != script["app_id"] || connector["status"] != "enabled" {
		return nil, fmt.Errorf("连接器不存在、未启用或权限已变化")
	}
	paths := []string{stringValue(source["path"])}
	pagination := asMap(source["pagination"])
	if explicit := anySlice(pagination["paths"]); len(explicit) > 0 {
		paths = []string{}
		for _, raw := range explicit {
			paths = append(paths, stringValue(raw))
		}
	}
	maxPages, maxItems, maxRequests := intValue(pagination["max_pages"]), intValue(pagination["max_items"]), intValue(pagination["max_requests"])
	if maxPages == 0 {
		maxPages = 1
	}
	if maxItems == 0 {
		maxItems = collectionScriptMaxItems
	}
	if maxRequests == 0 {
		maxRequests = collectionScriptMaxRequests
	}
	rows := []map[string]any{}
	requests := 0
	for i, path := range paths {
		if i >= maxPages || requests >= maxRequests || len(rows) >= maxItems {
			break
		}
		requests++
		key := "collection:" + stringValue(script["id"]) + ":" + stringValue(run["id"]) + ":" + path
		result, fetchErr := s.fetchConnectorResult(ctx, connector, path, clip(key, 160), mode != "preview")
		if fetchErr != nil {
			return map[string]any{"counts": map[string]any{"pages": i, "items": len(rows), "requests": requests}}, fetchErr
		}
		for _, raw := range asSliceMap(asMap(result)["items"]) {
			if len(rows) < maxItems {
				rows = append(rows, raw)
			}
		}
	}
	detail := asMap(source["detail"])
	if len(detail) > 0 {
		template := stringValue(detail["path_template"])
		pathField := stringValue(detail["path_field"])
		maxDetailRequests := intValue(detail["max_requests"])
		for i := range rows {
			if requests >= maxRequests || i >= maxDetailRequests {
				break
			}
			value, ok := collectionScriptValue(rows[i], pathField)
			if !ok || value == nil {
				continue
			}
			path := strings.ReplaceAll(template, "{value}", url.PathEscape(fmt.Sprint(value)))
			if !collectionScriptPath(path) {
				return map[string]any{"counts": map[string]any{"pages": len(paths), "items": len(rows), "requests": requests}}, fmt.Errorf("detail path 无效")
			}
			requests++
			key := "collection:" + stringValue(script["id"]) + ":" + stringValue(run["id"]) + ":detail:" + strconv.Itoa(i)
			result, fetchErr := s.fetchConnectorResult(ctx, connector, path, clip(key, 160), mode != "preview")
			if fetchErr != nil {
				return map[string]any{"counts": map[string]any{"pages": len(paths), "items": len(rows), "requests": requests}}, fetchErr
			}
			items := asSliceMap(asMap(result)["items"])
			if len(items) > 0 {
				for key, itemValue := range items[0] {
					rows[i][key] = itemValue
				}
			}
		}
	}
	if mode != "preview" {
		if err := s.collectionScriptExecutionGuard(ctx, script); err != nil {
			return map[string]any{"counts": map[string]any{"pages": len(paths), "items": len(rows), "requests": requests}}, err
		}
	}
	counts := map[string]any{"pages": len(paths), "requests": requests, "items": len(rows), "filtered": 0, "written": 0, "skipped": 0, "notifications": 0}
	results := []any{}
	target := asMap(definition["target"])
	mapping := asMap(target["fields"])
	dedup := asMap(definition["dedup"])
	for _, row := range rows {
		if mode != "preview" {
			if err := s.collectionScriptExecutionGuard(ctx, script); err != nil {
				return map[string]any{"counts": counts, "items": results}, err
			}
		}
		if !collectionScriptFilterMatch(row, anySlice(definition["filters"])) {
			continue
		}
		counts["filtered"] = intValue(counts["filtered"]) + 1
		data := map[string]any{}
		valid := true
		for targetField, rawMapping := range mapping {
			item := asMap(rawMapping)
			value, present := collectionScriptValue(row, stringValue(item["from"]))
			if !present {
				valid = false
				break
			}
			converted, ok := collectionScriptConvert(value, stringValue(item["type"]))
			if !ok {
				valid = false
				break
			}
			data[targetField] = converted
		}
		if !valid {
			results = append(results, map[string]any{"status": "error", "error": "字段映射值无法转换"})
			continue
		}
		dedupKey := collectionScriptDedupKey(row, collectionScriptStrings(asMap(dedup)["fields"]))
		item, findErr := s.PB.Find(ctx, "collection_script_items", listFilter("script_id = "+pbFilterString(stringValue(script["id"])), "dedup_key = "+pbFilterString(dedupKey)))
		if findErr == nil && stringValue(item["status"]) == "written" && equalJSON(asMap(item["source"]), row) {
			counts["skipped"] = intValue(counts["skipped"]) + 1
			results = append(results, map[string]any{"status": "skipped", "dedup_key": dedupKey})
			continue
		}
		if findErr == nil && stringValue(item["status"]) == "written" && stringValue(asMap(dedup)["on_change"]) == "skip" {
			counts["skipped"] = intValue(counts["skipped"]) + 1
			results = append(results, map[string]any{"status": "skipped", "dedup_key": dedupKey, "reason": "source_changed"})
			continue
		}
		existingItem := item
		if _, authorityErr := s.collectionScriptAuthority(ctx, script); authorityErr != nil {
			results = append(results, map[string]any{"status": "error", "dedup_key": dedupKey, "error": authorityErr.Error()})
			continue
		}
		actor := executionActor{UserID: stringValue(script["created_by"]), TenantID: stringValue(script["tenant_id"]), AppID: stringValue(script["app_id"]), Source: "collection_script"}
		var recordID, expectedUpdated string
		if item != nil {
			recordID = stringValue(item["target_record_id"])
			if recordID != "" {
				if tableMeta, metaErr := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(script["tenant_id"])), "app_id = "+pbFilterString(stringValue(script["app_id"])), "slug = "+pbFilterString(stringValue(target["table"])))); metaErr == nil {
					if current, rowErr := s.PB.Get(ctx, stringValue(tableMeta["pb_collection"]), recordID); rowErr == nil {
						expectedUpdated = stringValue(current["updated"])
					}
				}
			}
		}
		saved, writeErr := s.saveBusinessRecord(ctx, recordWrite{Actor: actor, Table: stringValue(target["table"]), RecordID: recordID, ExpectedUpdated: expectedUpdated, Data: data, AllowedFields: mapKeys(mapping)}, nil)
		if writeErr != nil {
			results = append(results, map[string]any{"status": "error", "dedup_key": dedupKey, "error": writeErr.Error()})
			continue
		}
		if item == nil {
			item, _ = s.PB.Create(ctx, "collection_script_items", map[string]any{"tenant_id": script["tenant_id"], "app_id": script["app_id"], "script_id": script["id"], "dedup_key": dedupKey, "status": "written", "target_record_id": saved["id"], "last_run_id": run["id"], "source": row})
		} else {
			item, _ = s.PB.Update(ctx, "collection_script_items", stringValue(item["id"]), map[string]any{"status": "written", "target_record_id": saved["id"], "last_run_id": run["id"], "source": row, "error": ""})
		}
		counts["written"] = intValue(counts["written"]) + 1
		if existingItem == nil && stringValue(definition["baseline"]) != "silent" {
			for _, recipient := range collectionScriptStrings(definition["recipients"]) {
				if _, notifyErr := s.createCollectionScriptNotification(ctx, script, run, item, recipient); notifyErr == nil {
					counts["notifications"] = intValue(counts["notifications"]) + 1
				}
			}
		}
		results = append(results, map[string]any{"status": "written", "dedup_key": dedupKey, "record_id": saved["id"]})
	}
	return map[string]any{"counts": counts, "items": results}, nil
}

// A live run may spend a long time fetching pages. Re-checking the script just
// before each business write makes pause, revision, archive, and owner revokes
// take effect without pretending that already-written records can be rolled
// back.
func (s *Server) collectionScriptExecutionGuard(ctx context.Context, script map[string]any) error {
	fresh, err := s.PB.Get(ctx, "collection_scripts", stringValue(script["id"]))
	if err != nil {
		return err
	}
	if fresh["tenant_id"] != script["tenant_id"] || fresh["app_id"] != script["app_id"] || intValue(fresh["revision"]) != intValue(script["revision"]) || stringValue(fresh["status"]) != "enabled" {
		return businessError(409, "采集脚本已暂停或版本已变化；已完成的记录保留，后续写入已停止")
	}
	_, err = s.collectionScriptAuthority(ctx, fresh)
	return err
}

func (s *Server) createCollectionScriptNotification(ctx context.Context, script, run, item map[string]any, recipient string) (map[string]any, error) {
	if _, err := s.PB.Get(ctx, "users", recipient); err != nil {
		return nil, err
	}
	if _, err := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(stringValue(script["tenant_id"])), "user_id = "+pbFilterString(recipient))); err != nil {
		return nil, err
	}
	key := stringValue(script["id"]) + ":" + stringValue(item["id"]) + ":" + recipient
	notification, err := s.PB.Find(ctx, "collection_script_notifications", listFilter("script_id = "+pbFilterString(stringValue(script["id"])), "item_id = "+pbFilterString(stringValue(item["id"])), "recipient_id = "+pbFilterString(recipient)))
	if err != nil {
		notification, err = s.PB.Create(ctx, "collection_script_notifications", map[string]any{"tenant_id": script["tenant_id"], "app_id": script["app_id"], "script_id": script["id"], "run_id": run["id"], "item_id": item["id"], "recipient_id": recipient, "event_key": key, "message": stringValue(script["name"]) + "：采集脚本新增了一条记录。", "status": "pending", "error": ""})
		if err != nil {
			return nil, err
		}
	}
	if stringValue(notification["status"]) == "delivered" {
		return notification, nil
	}
	message := stringValue(notification["message"])
	_, err = s.PB.Create(ctx, "automation_notifications", map[string]any{"tenant_id": script["tenant_id"], "app_id": script["app_id"], "rule_id": script["id"], "run_id": run["id"], "event_key": key, "user_id": recipient, "message": message, "read": false})
	if err != nil {
		if _, existingErr := s.PB.Find(ctx, "automation_notifications", listFilter("rule_id = "+pbFilterString(stringValue(script["id"])), "event_key = "+pbFilterString(key), "user_id = "+pbFilterString(recipient))); existingErr != nil {
			_, _ = s.PB.Update(ctx, "collection_script_notifications", stringValue(notification["id"]), map[string]any{"status": "failed", "error": clip(err.Error(), 1000)})
			return notification, err
		}
	}
	saved, err := s.PB.Update(ctx, "collection_script_notifications", stringValue(notification["id"]), map[string]any{"status": "delivered", "error": ""})
	if err != nil {
		return notification, err
	}
	return saved, nil
}

func (s *Server) finishCollectionScriptRun(ctx context.Context, run map[string]any, status string, result map[string]any, runErrors []string) (map[string]any, error) {
	updates := map[string]any{"status": status, "finished_at": nowISO()}
	if result != nil {
		updates["result"] = result
		if counts := result["counts"]; counts != nil {
			updates["counts"] = counts
		}
	}
	if len(runErrors) > 0 {
		errorsJSON := make([]any, len(runErrors))
		for i, value := range runErrors {
			errorsJSON[i] = value
		}
		updates["errors"] = errorsJSON
		updates["error"] = clip(strings.Join(runErrors, "; "), 1000)
	}
	saved, err := s.PB.Update(ctx, "collection_script_runs", stringValue(run["id"]), updates)
	if err != nil {
		return run, err
	}
	return saved, nil
}

// runDueCollectionScripts is called by the existing background scanner.
func (s *Server) runDueCollectionScripts(ctx context.Context) {
	now := time.Now().UTC()
	rows, err := s.PB.ListAll(ctx, "collection_scripts", "status = \"enabled\" && next_run_at != \"\" && next_run_at <= "+pbFilterString(now.Format(time.RFC3339Nano)), "next_run_at")
	if err != nil {
		return
	}
	for _, script := range rows {
		if _, err := s.collectionScriptAuthority(ctx, script); err != nil {
			_, _ = s.pauseCollectionScriptIfCurrent(ctx, script, "脚本负责人已失去权限")
			continue
		}
		eventKey := "schedule:" + strconv.Itoa(intValue(script["revision"])) + ":" + stringValue(script["next_run_at"])
		run, err := s.executeCollectionScript(ctx, script, "live", eventKey)
		if err != nil {
			continue
		}
		if stringValue(run["status"]) == "running" {
			continue
		}
		next := nextScheduledRun(collectionScriptSchedule(asMap(script["definition"])), now)
		_ = s.advanceCollectionScriptSchedule(ctx, script, next)
	}
	s.retryCollectionScriptNotifications(ctx)
}

func (s *Server) pauseCollectionScriptIfCurrent(ctx context.Context, script map[string]any, reason string) (map[string]any, error) {
	var saved map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		fresh, err := tx.Get(ctx, "collection_scripts", stringValue(script["id"]))
		if err != nil {
			return err
		}
		if fresh["tenant_id"] != script["tenant_id"] || fresh["app_id"] != script["app_id"] || intValue(fresh["revision"]) != intValue(script["revision"]) || stringValue(fresh["status"]) != "enabled" || stringValue(fresh["next_run_at"]) != stringValue(script["next_run_at"]) {
			return businessError(409, "采集脚本已变化")
		}
		saved, err = tx.Update(ctx, "collection_scripts", stringValue(fresh["id"]), map[string]any{"status": "paused", "next_run_at": "", "pause_reason": reason})
		return err
	})
	return saved, err
}

func (s *Server) advanceCollectionScriptSchedule(ctx context.Context, script map[string]any, next string) error {
	return s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		fresh, err := tx.Get(ctx, "collection_scripts", stringValue(script["id"]))
		if err != nil {
			return err
		}
		if fresh["tenant_id"] != script["tenant_id"] || fresh["app_id"] != script["app_id"] || intValue(fresh["revision"]) != intValue(script["revision"]) || stringValue(fresh["status"]) != "enabled" || stringValue(fresh["next_run_at"]) != stringValue(script["next_run_at"]) {
			return businessError(409, "采集脚本已变化")
		}
		_, err = tx.Update(ctx, "collection_scripts", stringValue(fresh["id"]), map[string]any{"next_run_at": next})
		return err
	})
}

func (s *Server) retryCollectionScriptNotifications(ctx context.Context) {
	rows, err := s.PB.ListAll(ctx, "collection_script_notifications", "status = \"pending\" || status = \"failed\"", "created")
	if err != nil {
		return
	}
	for _, notification := range rows {
		script, err := s.PB.Get(ctx, "collection_scripts", stringValue(notification["script_id"]))
		if err != nil {
			continue
		}
		if _, err := s.collectionScriptAuthority(ctx, script); err != nil {
			continue
		}
		if _, err := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(stringValue(notification["tenant_id"])), "user_id = "+pbFilterString(stringValue(notification["recipient_id"])))); err != nil {
			continue
		}
		if _, err := s.PB.Create(ctx, "automation_notifications", map[string]any{"tenant_id": notification["tenant_id"], "app_id": notification["app_id"], "rule_id": notification["script_id"], "run_id": notification["run_id"], "event_key": notification["event_key"], "user_id": notification["recipient_id"], "message": notification["message"], "read": false}); err != nil {
			if _, existingErr := s.PB.Find(ctx, "automation_notifications", listFilter("rule_id = "+pbFilterString(stringValue(notification["script_id"])), "event_key = "+pbFilterString(stringValue(notification["event_key"])), "user_id = "+pbFilterString(stringValue(notification["recipient_id"])))); existingErr != nil {
				_, _ = s.PB.Update(ctx, "collection_script_notifications", stringValue(notification["id"]), map[string]any{"status": "failed", "error": clip(err.Error(), 1000)})
				continue
			}
		}
		_, _ = s.PB.Update(ctx, "collection_script_notifications", stringValue(notification["id"]), map[string]any{"status": "delivered", "error": ""})
	}
}

func collectionScriptStrings(raw any) []string {
	values := []string{}
	for _, item := range anySlice(raw) {
		if value := stringValue(item); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
