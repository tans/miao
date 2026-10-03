package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

var appVersionLocks sync.Map

const (
	maxSourceFiles     = 32
	maxSourceBytes     = 512 * 1024
	maxSourceFileBytes = 256 * 1024
)

func lockAppVersion(id string) func() {
	value, _ := appVersionLocks.LoadOrStore(id, &sync.Mutex{})
	m := value.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

func (s *Server) routesVersions() {
	s.Mux.HandleFunc("GET /api/apps/{id}/runtime", s.auth(s.publishedRuntime))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions", s.auth(s.listVersions))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions/{versionId}", s.auth(s.getVersion))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions/{versionId}/preview", s.auth(s.previewVersion))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions/{versionId}/validation", s.auth(s.validateVersion))
	s.Mux.HandleFunc("GET /api/apps/{id}/versions/{versionId}/diff", s.auth(s.diffVersion))
	s.Mux.HandleFunc("POST /api/apps/{id}/runtime/actions/{actionId}", s.auth(s.runRuntimeAction))
	s.Mux.HandleFunc("POST /api/apps/{id}/versions", s.auth(s.createVersion))
	s.Mux.HandleFunc("POST /api/apps/{id}/versions/{versionId}/restore", s.auth(s.restoreVersion))
	s.Mux.HandleFunc("POST /api/apps/{id}/versions/{versionId}/publish", s.auth(s.publishVersion))
}

func (s *Server) validateSourceResources(ctx context.Context, app map[string]any, manifest map[string]any, tenantID string) string {
	raw := anySlice(manifest["resources"])
	if len(raw) > maxSourceFiles {
		return "资源引用超过 32 项"
	}
	seen := map[string]bool{}
	for _, item := range raw {
		id := stringValue(item)
		if object := asMap(item); object != nil {
			id = stringValue(object["id"])
		}
		if id == "" || seen[id] {
			return "资源引用必须是唯一的资源 ID"
		}
		seen[id] = true
		file, err := s.PB.Get(ctx, "app_files", id)
		if err != nil || file["tenant_id"] != tenantID || file["app_id"] != app["id"] {
			return "资源引用不存在或不属于当前应用"
		}
	}
	return ""
}

func (s *Server) validateVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, version, ok := s.loadVersion(ctx, r)
	if !ok {
		writeError(w, 404, "应用界面版本不存在")
		return
	}
	if isSourceVersion(version) {
		_, manifest, capabilities, msg := validateSourceVersion(version["source"], version["manifest"], version["capabilities"])
		if msg == "" {
			msg = s.validateSourceResources(ctx, app, manifest, stringValue(who(r).Tenant["id"]))
		}
		if msg == "" {
			msg = s.validateSourceActionReferences(ctx, app, manifest, true)
		}
		if msg == "" {
			msg = s.validateSourceActionReferences(ctx, app, manifest, true)
		}
		if msg != "" {
			writeJSON(w, 200, map[string]any{"valid": false, "errors": []string{msg}, "capabilities": capabilities})
			return
		}
		writeJSON(w, 200, map[string]any{"valid": true, "errors": []any{}, "format": "html", "capabilities": capabilities})
		return
	}
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	_, msg := validateAppUIDefinition(version["definition"], tables)
	writeJSON(w, 200, map[string]any{"valid": msg == "", "errors": func() []string {
		if msg == "" {
			return []string{}
		}
		return []string{msg}
	}()})
}

func versionStatus(version map[string]any, publishedID string) string {
	if stringValue(version["id"]) == publishedID {
		return "published"
	}
	if stringValue(version["published_at"]) != "" {
		return "superseded"
	}
	return "draft"
}
func publicVersion(version map[string]any, publishedID string) map[string]any {
	result := map[string]any{"id": version["id"], "version": version["version"], "summary": defaultString(stringValue(version["summary"]), ""), "status": versionStatus(version, publishedID), "based_on_version_id": defaultString(stringValue(version["based_on_version_id"]), ""), "created_at": version["created"], "updated_at": version["updated"], "published_at": version["published_at"]}
	if version["source"] != nil {
		result["format"] = "html"
		result["manifest"] = version["manifest"]
		result["capabilities"] = version["capabilities"]
	}
	return result
}

func validateSourceVersion(rawSource, rawManifest, rawCapabilities any) (map[string]string, map[string]any, []any, string) {
	rawFiles, ok := rawSource.(map[string]any)
	if !ok || len(rawFiles) == 0 || len(rawFiles) > maxSourceFiles {
		return nil, nil, nil, "源码文件必须为 1-32 个文件"
	}
	files := map[string]string{}
	total := 0
	for path, raw := range rawFiles {
		if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "\\") || strings.Contains(path, "..") || path != strings.TrimSpace(path) {
			return nil, nil, nil, "源码路径必须是安全的相对路径"
		}
		if path != "index.html" && path != "styles.css" && path != "app.js" && !strings.HasPrefix(path, "pages/") && !strings.HasPrefix(path, "assets/") {
			return nil, nil, nil, "源码路径包含不支持的文件"
		}
		value, ok := raw.(string)
		if !ok || value == "" || len([]byte(value)) > maxSourceFileBytes {
			return nil, nil, nil, "源码文件内容无效或超过大小限制"
		}
		if strings.HasPrefix(path, "assets/") && !strings.HasSuffix(path, ".css") && !strings.HasSuffix(path, ".js") && !strings.HasSuffix(path, ".html") {
			return nil, nil, nil, "资源只能使用受支持的文本文件类型"
		}
		files[path] = value
		total += len([]byte(value))
	}
	if files["index.html"] == "" {
		return nil, nil, nil, "源码必须包含 index.html"
	}
	manifest, ok := rawManifest.(map[string]any)
	if !ok {
		return nil, nil, nil, "源码 manifest 必须是对象"
	}
	for key := range manifest {
		if !containsString([]string{"entry", "routes", "resources", "csp", "actions"}, key) {
			return nil, nil, nil, "manifest 包含不支持的配置"
		}
	}
	entry := stringValue(manifest["entry"])
	if entry == "" || files[entry] == "" || !strings.HasSuffix(entry, ".html") {
		return nil, nil, nil, "manifest.entry 必须引用 HTML 页面"
	}
	routes := anySlice(manifest["routes"])
	if len(routes) == 0 || len(routes) > maxSourceFiles {
		return nil, nil, nil, "manifest.routes 必须包含 1-32 个页面"
	}
	for _, route := range routes {
		item := asMap(route)
		path := stringValue(item["path"])
		file := stringValue(item["file"])
		if path == "" || !strings.HasPrefix(path, "/") || strings.Contains(path, "..") || files[file] == "" || !strings.HasSuffix(file, ".html") {
			return nil, nil, nil, "manifest 路由无效"
		}
	}
	actionIDs := anySlice(manifest["actions"])
	if len(actionIDs) > 32 {
		return nil, nil, nil, "manifest.actions 不能超过 32 项"
	}
	seenActions := map[string]bool{}
	for _, raw := range actionIDs {
		id := stringValue(raw)
		if !validSlugID(id, 64) || seenActions[id] {
			return nil, nil, nil, "manifest.actions 包含无效或重复的动作 ID"
		}
		seenActions[id] = true
	}
	caps := anySlice(rawCapabilities)
	if len(caps) > 64 {
		return nil, nil, nil, "能力清单超过 64 项"
	}
	allowed := map[string]bool{"records.read": true, "records.create": true, "records.update": true, "records.delete": true, "navigation": true, "user.read": true, "files.read": true, "files.upload": true, "actions.execute": true}
	for _, item := range caps {
		name := stringValue(item)
		if !allowed[name] {
			return nil, nil, nil, "能力清单包含未支持或未授权能力"
		}
	}
	if total > maxSourceBytes {
		return nil, nil, nil, "源码总大小超过 512 KB"
	}
	return files, manifest, caps, ""
}

func isSourceVersion(version map[string]any) bool { return version["source"] != nil }

func validateAppUIDefinition(raw any, tables []map[string]any) (map[string]any, string) {
	definition, ok := raw.(map[string]any)
	if !ok {
		return nil, "界面定义必须是对象"
	}
	version := intValue(definition["schema_version"])
	if version != 2 {
		return nil, "界面定义版本不受支持"
	}
	for key := range definition {
		if !containsString([]string{"schema_version", "title", "pages"}, key) {
			return nil, "界面定义包含不支持的配置"
		}
	}
	title := strings.TrimSpace(stringValue(definition["title"]))
	if title == "" || len([]rune(title)) > 120 {
		return nil, "界面标题必须为 1 到 120 个字符"
	}
	rawPages := anySlice(definition["pages"])
	if len(rawPages) < 1 || len(rawPages) > 12 {
		return nil, "应用需要 1–12 个页面"
	}
	pages := []map[string]any{}
	ids := map[string]bool{}
	for _, rawPage := range rawPages {
		page := asMap(rawPage)
		for key := range page {
			if !containsString([]string{"id", "title", "collection", "fields", "actions"}, key) {
				return nil, "页面配置无效"
			}
		}
		pid := stringValue(page["id"])
		if !validSlugID(pid, 40) || ids[pid] {
			return nil, "页面标识无效或重复"
		}
		ids[pid] = true
		ptitle := strings.TrimSpace(stringValue(page["title"]))
		if ptitle == "" || len([]rune(ptitle)) > 120 {
			return nil, "页面标题无效"
		}
		var table map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == page["collection"] {
				table = candidate
				break
			}
		}
		if table == nil {
			return nil, "页面引用的数据表不存在于当前应用"
		}
		selectedRaw := anySlice(page["fields"])
		if len(selectedRaw) < 1 || len(selectedRaw) > 24 {
			return nil, "页面字段无效或重复"
		}
		selected := []string{}
		seenFields := map[string]bool{}
		for _, v := range selectedRaw {
			name := stringValue(v)
			if seenFields[name] || findField(asSliceMap(table["fields"]), name) == nil {
				return nil, "页面字段无效或重复"
			}
			seenFields[name] = true
			selected = append(selected, name)
		}
		actionsRaw := anySlice(page["actions"])
		if len(actionsRaw) > 8 {
			return nil, "页面最多支持 8 个动作"
		}
		actions := []map[string]any{}
		actionIDs := map[string]bool{}
		for _, rawAction := range actionsRaw {
			action := asMap(rawAction)
			for key := range action {
				if !containsString([]string{"id", "label", "set", "action_id"}, key) {
					return nil, "业务动作无效或重复"
				}
			}
			aid, label := stringValue(action["id"]), strings.TrimSpace(stringValue(action["label"]))
			if !validSlugID(aid, 40) || actionIDs[aid] || label == "" || len([]rune(label)) > 60 {
				return nil, "业务动作无效或重复"
			}
			actionIDs[aid] = true
			set, ok := action["set"].(map[string]any)
			actionID := stringValue(action["action_id"])
			if actionID != "" && (len(actionID) > 64 || strings.TrimSpace(actionID) != actionID) {
				return nil, "业务动作引用无效"
			}
			if actionID == "" && (!ok || len(set) == 0) {
				return nil, "动作需要具体字段赋值或引用通用业务动作"
			}
			for name, value := range set {
				field := findField(asSliceMap(table["fields"]), name)
				if field == nil || field["type"] == "file" || field["type"] == "relation" || (boolValue(field["required"]) && value == "") {
					return nil, "动作字段无效"
				}
				typ := stringValue(field["type"])
				valid := false
				switch typ {
				case "number":
					_, valid = numeric(value)
				case "bool":
					_, valid = value.(bool)
				default:
					_, valid = value.(string)
				}
				if !valid {
					return nil, "动作赋值无效"
				}
				if typ == "select" && !contains(field["options"], value) && !(value == "" && !boolValue(field["required"])) {
					return nil, "动作赋值无效"
				}
			}
			safeAction := map[string]any{"id": aid, "label": label, "set": set}
			if actionID != "" {
				safeAction["action_id"] = actionID
				delete(safeAction, "set")
			}
			actions = append(actions, safeAction)
		}
		pages = append(pages, map[string]any{"id": pid, "title": ptitle, "collection": page["collection"], "fields": selected, "actions": actions})
	}
	return map[string]any{"schema_version": 2, "title": title, "pages": pages}, ""
}

func validSlugID(value string, max int) bool {
	if len(value) < 1 || len(value) > max || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, r := range value[1:] {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}

func (s *Server) validateBusinessActionReferences(ctx context.Context, app map[string]any, definition map[string]any, requireEnabled bool) string {
	for _, page := range appUIPages(definition) {
		for _, action := range asSliceMap(page["actions"]) {
			actionID := stringValue(action["action_id"])
			if actionID == "" {
				continue
			}
			businessAction, err := s.PB.Get(ctx, "business_actions", actionID)
			if err != nil || businessAction["tenant_id"] != app["tenant_id"] || businessAction["app_id"] != app["id"] || businessAction["status"] == "archived" {
				return "页面引用的通用业务动作不存在或不属于当前应用"
			}
			if requireEnabled && businessAction["status"] != "enabled" {
				return "页面引用的通用业务动作尚未启用"
			}
		}
	}
	return ""
}

func (s *Server) validateSourceActionReferences(ctx context.Context, app map[string]any, manifest map[string]any, requireEnabled bool) string {
	for _, raw := range anySlice(manifest["actions"]) {
		actionID := stringValue(raw)
		businessAction, err := s.PB.Get(ctx, "business_actions", actionID)
		if err != nil || businessAction["tenant_id"] != app["tenant_id"] || businessAction["app_id"] != app["id"] || businessAction["status"] == "archived" {
			return "源码引用的通用业务动作不存在或不属于当前应用"
		}
		if requireEnabled && businessAction["status"] != "enabled" {
			return "源码引用的通用业务动作尚未启用"
		}
	}
	return ""
}
func appUIPages(definition map[string]any) []map[string]any { return asSliceMap(definition["pages"]) }

func (s *Server) runtimeForVersion(ctx context.Context, app map[string]any, tenantID string, version map[string]any, query map[string]string, perPageDefault int) (map[string]any, error) {
	tables, err := s.appTables(ctx, app, tenantID)
	if err != nil {
		return nil, err
	}
	definition, errText := validateAppUIDefinition(version["definition"], tables)
	if errText != "" {
		return map[string]any{"status": "unavailable"}, nil
	}
	pages := appUIPages(definition)
	if len(pages) == 0 {
		return map[string]any{"status": "unavailable"}, nil
	}
	pageID := query["ui_page"]
	page := pages[0]
	if pageID != "" {
		found := false
		for _, p := range pages {
			if p["id"] == pageID {
				page = p
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("应用页面不存在")
		}
	}
	var table map[string]any
	for _, candidate := range tables {
		if candidate["slug"] == page["collection"] {
			table = candidate
			break
		}
	}
	if table == nil {
		return map[string]any{"status": "unavailable"}, nil
	}
	fields := asSliceMap(table["fields"])
	selected := []map[string]any{}
	for _, name := range anySlice(page["fields"]) {
		if field := findField(fields, stringValue(name)); field != nil {
			selected = append(selected, field)
		}
	}
	formSource := fields
	form := []map[string]any{}
	available := map[string]bool{"text": true, "number": true, "bool": true, "date": true, "email": true, "url": true, "select": true, "relation": true, "file": true}
	formNames := map[string]bool{}
	for _, f := range formSource {
		if !available[stringValue(f["type"])] {
			continue
		}
		copy := map[string]any{}
		for k, v := range f {
			copy[k] = v
		}
		form = append(form, copy)
		formNames[stringValue(f["name"])] = true
	}
	requiredPresent := true
	for _, f := range fields {
		if boolValue(f["required"]) && !formNames[stringValue(f["name"])] {
			requiredPresent = false
		}
	}
	per := 25
	if perPageDefault > 0 {
		per = perPageDefault
	}
	if n, ok := strconvAtoiRange(query["perPage"], 1, 50); ok {
		per = n
	}
	pageNum := 1
	if n, ok := strconvAtoiRange(query["page"], 1, 1000000); ok {
		pageNum = n
	}
	parts := []string{"tenant_id = " + pbFilterString(tenantID), "app_id = " + pbFilterString(stringValue(app["id"]))}
	search := clip(strings.TrimSpace(query["search"]), 120)
	alts := []string{}
	for _, f := range selected {
		if contains([]any{"text", "email", "url"}, f["type"]) {
			alts = append(alts, stringValue(f["name"])+" ~ "+pbFilterString(search))
		}
	}
	if search != "" && len(alts) > 0 {
		parts = append(parts, "("+strings.Join(alts, " || ")+")")
	}
	rows, total, pagesTotal, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(parts...), "-created", pageNum, per)
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	for _, row := range rows {
		data := map[string]any{}
		for _, f := range selected {
			name := stringValue(f["name"])
			v := row[name]
			if v == nil {
				v = nil
			}
			data[name] = v
		}
		items = append(items, map[string]any{"id": row["id"], "data": data, "created_at": row["created"], "updated_at": row["updated"]})
	}
	labels := map[string]map[string]string{}
	for _, f := range selected {
		if f["type"] != "relation" {
			continue
		}
		var target map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == f["target"] {
				target = candidate
				break
			}
		}
		if target == nil {
			continue
		}
		labels[stringValue(f["name"])] = map[string]string{}
		firstText := ""
		for _, tf := range asSliceMap(target["fields"]) {
			if tf["type"] == "text" {
				firstText = stringValue(tf["name"])
				break
			}
		}
		seen := map[string]bool{}
		for _, row := range rows {
			rid := stringValue(row[stringValue(f["name"])])
			if rid == "" || seen[rid] {
				continue
			}
			seen[rid] = true
			related, e := s.PB.Get(ctx, stringValue(target["pb_collection"]), rid)
			if e == nil && related["tenant_id"] == tenantID && related["app_id"] == app["id"] {
				label := stringValue(related[firstText])
				if label == "" {
					label = rid
				}
				labels[stringValue(f["name"])][rid] = label
			}
		}
	}
	publicFields := make([]map[string]any, 0, len(selected))
	searchable := false
	for _, f := range selected {
		copy := map[string]any{}
		for k, v := range f {
			if k != "target_collection_id" && k != "target_name" {
				copy[k] = v
			}
		}
		copy["label"] = defaultString(stringValue(f["label"]), stringValue(f["name"]))
		publicFields = append(publicFields, copy)
		if contains([]any{"text", "email", "url"}, f["type"]) {
			searchable = true
		}
	}
	pageList := []map[string]any{}
	for _, p := range pages {
		pageList = append(pageList, map[string]any{"id": p["id"], "title": p["title"], "collection": p["collection"]})
	}
	return map[string]any{"status": "published", "relation_labels": labels, "version": publicVersion(version, stringValue(version["id"])), "title": page["title"], "app_title": definition["title"], "ui_page": page["id"], "pages": pageList, "actions": anySlice(page["actions"]), "collection": table["slug"], "fields": publicFields, "create_form_available": len(form) > 0 && requiredPresent, "create_form_fields": func() any {
		if len(form) > 0 && requiredPresent {
			return form
		}
		return []any{}
	}(), "search_supported": searchable, "page": pageNum, "per_page": per, "total_items": total, "total_pages": pagesTotal, "items": items}, nil
}

func strconvAtoiRange(value string, min, max int) (int, bool) {
	if value == "" {
		return 0, false
	}
	n, err := strconv.Atoi(value)
	return n, err == nil && n >= min && n <= max
}

func (s *Server) publishedRuntime(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	versionID := stringValue(app["published_version_id"])
	if versionID == "" {
		writeJSON(w, 200, map[string]any{"status": "not_published"})
		return
	}
	version, err := s.PB.Get(ctx, "app_versions", versionID)
	if err != nil || version["tenant_id"] != who(r).Tenant["id"] || version["app_id"] != app["id"] {
		writeJSON(w, 200, map[string]any{"status": "unavailable"})
		return
	}
	if isSourceVersion(version) {
		writeJSON(w, 200, map[string]any{
			"status": "published", "version": publicVersion(version, stringValue(version["id"])),
			"source": version["source"], "manifest": version["manifest"], "capabilities": version["capabilities"],
			"runtime": map[string]any{"isolation": "sandboxed-iframe", "sandbox": "allow-scripts", "protocol": "miao-app-v1", "app_id": app["id"], "version_id": version["id"], "nonce_required": true},
		})
		return
	}
	query := map[string]string{}
	for _, key := range []string{"ui_page", "page", "perPage", "search"} {
		query[key] = r.URL.Query().Get(key)
	}
	result, err := s.runtimeForVersion(ctx, app, stringValue(who(r).Tenant["id"]), version, query, 0)
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) listVersions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	page := queryInt(r, "page", 1, 1, 1000000)
	rows, total, _, err := s.PB.List(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "-version", page, 50)
	if err != nil {
		writeError(w, 503, "版本列表暂不可用")
		return
	}
	items := []map[string]any{}
	for _, row := range rows {
		items = append(items, publicVersion(row, stringValue(app["published_version_id"])))
	}
	writeJSON(w, 200, map[string]any{"published_version_id": nilIfEmpty(stringValue(app["published_version_id"])), "page": page, "per_page": 50, "total_items": total, "items": items})
}
func (s *Server) loadVersion(ctx context.Context, r *http.Request) (map[string]any, map[string]any, bool) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, nil, false
	}
	version, err := s.PB.Get(ctx, "app_versions", pathID(r, "versionId"))
	if err != nil || version["tenant_id"] != who(r).Tenant["id"] || version["app_id"] != app["id"] {
		return app, nil, false
	}
	return app, version, true
}
func (s *Server) getVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, version, ok := s.loadVersion(ctx, r)
	if !ok {
		writeError(w, 404, "应用界面版本不存在")
		return
	}
	result := publicVersion(version, stringValue(app["published_version_id"]))
	if isSourceVersion(version) {
		result["source"], result["manifest"], result["capabilities"] = version["source"], version["manifest"], version["capabilities"]
	} else {
		result["definition"] = version["definition"]
	}
	writeJSON(w, 200, result)
}
func (s *Server) diffVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, version, ok := s.loadVersion(ctx, r)
	if !ok {
		writeError(w, 404, "应用版本不存在")
		return
	}
	currentID := stringValue(app["published_version_id"])
	var current map[string]any
	if currentID != "" {
		var err error
		current, err = s.PB.Get(ctx, "app_versions", currentID)
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
	}
	changes := diffAppUI(asMap(current["definition"]), asMap(version["definition"]))
	if isSourceVersion(current) || isSourceVersion(version) {
		changes = []map[string]any{{"type": "source_version", "before_version_id": nilIfEmpty(currentID), "after_version_id": version["id"], "files": sourceFileNames(asMap(version["source"]))}}
	}
	writeJSON(w, 200, map[string]any{"current_version_id": nilIfEmpty(currentID), "target_version_id": version["id"], "changes": changes, "data_changed": false})
}

func sourceFileNames(files map[string]any) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func diffAppUI(before, after map[string]any) []map[string]any {
	changes := []map[string]any{}
	if !equalJSON(before["title"], after["title"]) {
		changes = append(changes, map[string]any{"type": "title", "before": defaultString(stringValue(before["title"]), ""), "after": after["title"]})
	}
	oldPages := []map[string]any{}
	for _, p := range appUIPages(before) {
		if stringValue(p["collection"]) != "" {
			oldPages = append(oldPages, p)
		}
	}
	newPages := appUIPages(after)
	for _, old := range oldPages {
		found := false
		for _, next := range newPages {
			if next["id"] == old["id"] {
				found = true
				break
			}
		}
		if !found {
			changes = append(changes, map[string]any{"type": "remove_page", "page": old["id"], "before": old})
		}
	}
	for _, next := range newPages {
		var old map[string]any
		for _, candidate := range oldPages {
			if candidate["id"] == next["id"] {
				old = candidate
				break
			}
		}
		if old == nil {
			changes = append(changes, map[string]any{"type": "add_page", "page": next["id"], "after": next})
		} else if !equalJSON(old, next) {
			changes = append(changes, map[string]any{"type": "change_page", "page": next["id"], "before": old, "after": next})
		}
	}
	return changes
}

func (s *Server) previewVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	app, version, ok := s.loadVersion(ctx, r)
	if !ok {
		writeError(w, 404, "应用界面版本不存在")
		return
	}
	if isSourceVersion(version) {
		writeJSON(w, 200, map[string]any{
			"status": "preview", "version": publicVersion(version, stringValue(app["published_version_id"])),
			"source": version["source"], "manifest": version["manifest"], "capabilities": version["capabilities"],
			"changes":      []any{map[string]any{"type": "source_preview", "files": sourceFileNames(asMap(version["source"]))}},
			"runtime":      map[string]any{"isolation": "sandboxed-iframe", "sandbox": "allow-scripts", "protocol": "miao-app-v1", "csp": "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; connect-src 'none'; frame-src 'none'; object-src 'none'; form-action 'none'; base-uri 'none'; navigate-to 'none'"},
			"data_changed": false,
		})
		return
	}
	preview, err := s.runtimeForVersion(ctx, app, stringValue(who(r).Tenant["id"]), version, map[string]string{"perPage": "5"}, 5)
	if err != nil || preview["status"] != "published" {
		writeError(w, 409, "此版本当前无法预览")
		return
	}
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	definition, msg := validateAppUIDefinition(version["definition"], tables)
	if msg != "" {
		writeError(w, 409, msg)
		return
	}
	if msg = s.validateBusinessActionReferences(ctx, app, definition, false); msg != "" {
		writeError(w, 409, msg)
		return
	}
	pages := appUIPages(definition)
	previews := []map[string]any{}
	for _, page := range pages {
		current := preview
		if page["id"] != preview["ui_page"] {
			current, err = s.runtimeForVersion(ctx, app, stringValue(who(r).Tenant["id"]), version, map[string]string{"perPage": "5", "ui_page": stringValue(page["id"])}, 5)
			if err != nil {
				s.writeBusinessError(w, err)
				return
			}
			if current["status"] != "published" {
				writeError(w, 409, "页面无法预览")
				return
			}
		}
		previews = append(previews, map[string]any{"id": page["id"], "title": page["title"], "collection": current["collection"], "fields": current["fields"], "actions": current["actions"], "items": current["items"], "total_items": current["total_items"], "relation_labels": current["relation_labels"]})
	}
	currentID := stringValue(app["published_version_id"])
	var currentVersion map[string]any
	if currentID != "" {
		currentVersion, err = s.PB.Get(ctx, "app_versions", currentID)
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
	}
	preview["page_previews"], preview["status"], preview["version"], preview["changes"], preview["note"] = previews, "preview", publicVersion(version, currentID), diffAppUI(asMap(currentVersion["definition"]), definition), "仅界面定义变更，业务记录不回滚"
	writeJSON(w, 200, preview)
}

func (s *Server) createVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(role) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	input := mapBody(r)
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	var definition map[string]any
	var source map[string]string
	var manifest map[string]any
	var capabilities []any
	var msg string
	format := stringValue(input["format"])
	if format == "html" || input["source"] != nil {
		source, manifest, capabilities, msg = validateSourceVersion(input["source"], input["manifest"], input["capabilities"])
		if msg != "" {
			writeError(w, 400, msg)
			return
		}
		if msg = s.validateSourceResources(ctx, app, manifest, stringValue(who(r).Tenant["id"])); msg != "" {
			writeError(w, 400, msg)
			return
		}
		if msg = s.validateSourceActionReferences(ctx, app, manifest, false); msg != "" {
			writeError(w, 400, msg)
			return
		}
	} else {
		definition, msg = validateAppUIDefinition(input["definition"], tables)
		if msg != "" {
			writeError(w, 400, msg)
			return
		}
		if msg = s.validateBusinessActionReferences(ctx, app, definition, false); msg != "" {
			writeError(w, 400, msg)
			return
		}
	}
	basedID := stringValue(input["based_on_version_id"])
	var base map[string]any
	if basedID != "" {
		base, err = s.PB.Get(ctx, "app_versions", basedID)
		if err != nil || base["app_id"] != app["id"] || base["tenant_id"] != who(r).Tenant["id"] {
			writeError(w, 404, "修订来源版本不存在")
			return
		}
		if input["restore"] != true && stringValue(base["published_at"]) != "" && base["id"] != app["published_version_id"] {
			writeError(w, 409, "只能从当前正式版本或尚未发布的草稿创建修订")
			return
		}
	}
	unlock := lockAppVersion(stringValue(app["id"]))
	defer unlock()
	latest, _, _, err := s.PB.List(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "-version", 1, 1)
	if err != nil {
		writeError(w, 503, "版本序号读取失败")
		return
	}
	number := 1
	if len(latest) > 0 {
		number = intValue(latest[0]["version"]) + 1
	}
	id := who(r)
	versionData := map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "version": number, "summary": clip(strings.TrimSpace(stringValue(input["summary"])), 1000), "based_on_version_id": basedID, "created_by": id.User["id"]}
	if source != nil {
		versionData["source"], versionData["manifest"], versionData["capabilities"] = source, manifest, capabilities
	} else {
		versionData["definition"] = definition
	}
	version, err := s.PB.Create(ctx, "app_versions", versionData)
	if err != nil {
		writeError(w, 409, "版本序号刚发生变化，请刷新版本列表后重试；原草稿和已发布界面未更改")
		return
	}
	response := publicVersion(version, stringValue(app["published_version_id"]))
	if source != nil {
		response["source"], response["manifest"], response["capabilities"] = source, manifest, capabilities
	} else {
		response["definition"] = definition
	}
	writeJSON(w, 201, response)
}

func (s *Server) restoreVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(role) {
		writeError(w, 403, "你没有管理此应用的权限")
		return
	}
	source, err := s.PB.Get(ctx, "app_versions", pathID(r, "versionId"))
	if err != nil || source["tenant_id"] != who(r).Tenant["id"] || source["app_id"] != app["id"] {
		writeError(w, 404, "应用界面版本不存在")
		return
	}
	if source["id"] == app["published_version_id"] || stringValue(source["published_at"]) == "" {
		writeError(w, 409, "只能从已发布过的历史界面创建恢复草稿")
		return
	}
	if current, err := s.PB.Get(ctx, "app_versions", stringValue(app["published_version_id"])); err == nil && intValue(source["version"]) >= intValue(current["version"]) {
		writeError(w, 409, "目标版本不是早于当前正式界面的历史版本")
		return
	}
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	input := map[string]any{"summary": fmt.Sprintf("恢复自 v%d", intValue(source["version"])), "based_on_version_id": source["id"]}
	input["restore"] = true
	if isSourceVersion(source) {
		if _, _, _, msg := validateSourceVersion(source["source"], source["manifest"], source["capabilities"]); msg != "" {
			writeError(w, 409, fmt.Sprintf("无法从 v%d 创建恢复草稿：%s。当前正式界面未更改，也没有创建草稿。", intValue(source["version"]), msg))
			return
		}
		input["format"], input["source"], input["manifest"], input["capabilities"] = "html", source["source"], source["manifest"], source["capabilities"]
	} else {
		definition, msg := validateAppUIDefinition(source["definition"], tables)
		if msg != "" {
			writeError(w, 409, fmt.Sprintf("无法从 v%d 创建恢复草稿：%s。当前正式界面未更改，也没有创建草稿。", intValue(source["version"]), msg))
			return
		}
		input["definition"] = definition
	}
	body, _ := jsonMarshal(input)
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	s.createVersion(w, r)
}

func (s *Server) publishVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canPublishAppRole(role) {
		writeError(w, 403, "你没有发布权限")
		return
	}
	var input struct {
		Expected json.RawMessage `json:"expected_published_version_id"`
	}
	var expectedID *string
	if readJSON(r, &input) != nil || len(input.Expected) == 0 || json.Unmarshal(input.Expected, &expectedID) != nil {
		writeError(w, 400, "发布时必须提供当前已发布版本（字符串或 null），用于检测并发变更")
		return
	}
	expected := ""
	if expectedID != nil {
		expected = *expectedID
	}
	unlock := lockAppVersion(stringValue(app["id"]))
	defer unlock()
	latestApp, err := s.PB.Get(ctx, "apps", stringValue(app["id"]))
	if err != nil {
		writeError(w, 503, "应用暂时不可用")
		return
	}
	currentID := stringValue(latestApp["published_version_id"])
	if expected != currentID {
		writeJSON(w, 409, map[string]any{"error": "应用已被其他操作发布了新版本，请刷新版本列表后重试", "current_version_id": nilIfEmpty(currentID)})
		return
	}
	version, err := s.PB.Get(ctx, "app_versions", pathID(r, "versionId"))
	if err != nil || version["tenant_id"] != who(r).Tenant["id"] || version["app_id"] != app["id"] {
		writeError(w, 404, "草稿版本不存在")
		return
	}
	if currentID == stringValue(version["id"]) {
		result, err := s.runtimeForVersion(ctx, latestApp, stringValue(who(r).Tenant["id"]), version, map[string]string{}, 0)
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if stringValue(version["published_at"]) != "" {
		writeError(w, 409, "此版本已经发布或已被替代；如需恢复，请先创建一个新版本")
		return
	}
	if currentID != "" {
		current, e := s.PB.Get(ctx, "app_versions", currentID)
		if e != nil {
			s.writeBusinessError(w, e)
			return
		}
		if intValue(version["version"]) <= intValue(current["version"]) {
			writeError(w, 409, "不能发布早于或等于当前版本的草稿；如需回退，请基于当前版本创建新草稿")
			return
		}
	}
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	if isSourceVersion(version) {
		_, manifest, _, msg := validateSourceVersion(version["source"], version["manifest"], version["capabilities"])
		if msg == "" {
			msg = s.validateSourceResources(ctx, app, manifest, stringValue(who(r).Tenant["id"]))
		}
		if msg != "" {
			writeError(w, 400, "草稿无法发布："+msg)
			return
		}
	} else if _, msg := validateAppUIDefinition(version["definition"], tables); msg != "" {
		writeError(w, 400, "草稿无法发布："+msg)
		return
	} else if msg := s.validateBusinessActionReferences(ctx, app, asMap(version["definition"]), true); msg != "" {
		writeError(w, 400, "草稿无法发布："+msg)
		return
	}
	previewApp := map[string]any{}
	for k, v := range latestApp {
		previewApp[k] = v
	}
	previewApp["published_version_id"] = version["id"]
	runtime := map[string]any{}
	if isSourceVersion(version) {
		runtime = map[string]any{"status": "published", "source": version["source"], "manifest": version["manifest"], "capabilities": version["capabilities"], "runtime": map[string]any{"isolation": "sandboxed-iframe", "sandbox": "allow-scripts", "protocol": "miao-app-v1", "app_id": app["id"], "version_id": version["id"], "nonce_required": true}}
	} else {
		runtime, err = s.runtimeForVersion(ctx, previewApp, stringValue(who(r).Tenant["id"]), version, map[string]string{}, 0)
		if err != nil || runtime["status"] != "published" {
			writeError(w, 409, "草稿运行检查失败，当前发布版未更改")
			return
		}
	}
	err = s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		fresh, err := s.authorizeWrite(ctx, tx, who(r).actor(stringValue(app["id"]), "interactive"), false)
		if err != nil {
			return err
		}
		if stringValue(fresh["published_version_id"]) != currentID {
			return businessError(409, "正式界面已变化，请刷新后发布")
		}
		tenant, err := tx.Get(ctx, "tenants", stringValue(who(r).Tenant["id"]))
		if err != nil {
			return err
		}
		member, err := tx.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(stringValue(tenant["id"])), "user_id = "+pbFilterString(stringValue(who(r).User["id"]))))
		if err != nil {
			return err
		}
		access, err := applicationAccess(ctx, tx, fresh, identity{User: who(r).User, Tenant: tenant, Membership: member})
		if err != nil {
			return err
		}
		if !access.Role.canPublish() {
			return businessError(403, "没有发布权限")
		}
		tables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "created")
		if err != nil {
			return err
		}
		if isSourceVersion(version) {
			_, manifest, _, msg := validateSourceVersion(version["source"], version["manifest"], version["capabilities"])
			if msg == "" {
				msg = s.validateSourceResources(ctx, app, manifest, stringValue(who(r).Tenant["id"]))
			}
			if msg == "" {
				msg = s.validateSourceActionReferences(ctx, app, manifest, true)
			}
			if msg != "" {
				return businessError(409, msg)
			}
		} else if _, msg := validateAppUIDefinition(version["definition"], tables); msg != "" {
			return businessError(409, msg)
		}
		version, err = tx.Update(ctx, "app_versions", stringValue(version["id"]), map[string]any{"published_at": nowISO()})
		if err != nil {
			return err
		}
		_, err = tx.Update(ctx, "apps", stringValue(app["id"]), map[string]any{"published_version_id": version["id"]})
		return err
	})
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	runtime["version"] = publicVersion(version, stringValue(version["id"]))
	writeJSON(w, 200, runtime)
}

func (s *Server) runRuntimeAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if role == "viewer" || boolValue(app["archived"]) {
		writeError(w, 403, "没有业务修改权限")
		return
	}
	input := mapBody(r)
	expectedVersion := stringValue(input["expected_version_id"])
	expectedUpdated := stringValue(input["expected_updated_at"])
	if input["confirm"] != true || expectedUpdated == "" || expectedVersion != stringValue(app["published_version_id"]) {
		writeError(w, 409, "请确认当前界面、记录及动作，刷新后重试")
		return
	}
	version, err := s.PB.Get(ctx, "app_versions", expectedVersion)
	if err != nil {
		writeError(w, 409, "正式界面已变化")
		return
	}
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	definition, msg := validateAppUIDefinition(version["definition"], tables)
	if msg != "" {
		writeError(w, 409, msg)
		return
	}
	var page map[string]any
	for _, p := range appUIPages(definition) {
		if p["id"] == input["ui_page"] {
			page = p
			break
		}
	}
	if page == nil {
		writeError(w, 404, "业务动作不存在")
		return
	}
	var action map[string]any
	for _, a := range asSliceMap(page["actions"]) {
		if a["id"] == pathID(r, "actionId") {
			action = a
			break
		}
	}
	if action == nil {
		writeError(w, 404, "业务动作不存在")
		return
	}
	if actionID := stringValue(action["action_id"]); actionID != "" {
		businessAction, err := s.PB.Get(ctx, "business_actions", actionID)
		if err != nil || businessAction["tenant_id"] != who(r).Tenant["id"] || businessAction["app_id"] != app["id"] || businessAction["status"] != "enabled" {
			writeError(w, 409, "引用的通用业务动作不存在、未启用或已失效")
			return
		}
		inputValues := asMap(input["input"])
		if inputValues == nil {
			inputValues = map[string]any{}
		}
		inputValues["record_id"] = input["record_id"]
		inputValues["record_updated_at"] = expectedUpdated
		key := defaultString(stringValue(input["idempotency_key"]), fmt.Sprintf("ui:%s:%s:%s:%s", expectedVersion, input["ui_page"], action["id"], input["record_id"]))
		if previous, findErr := s.PB.Find(ctx, "business_action_runs", listFilter("action_id = "+pbFilterString(actionID), "idempotency_key = "+pbFilterString(key))); findErr == nil {
			writeJSON(w, 200, previous["result"])
			return
		}
		result, err := s.executeActionSteps(ctx, who(r), app, businessAction, asMap(businessAction["definition"]), inputValues, func(tx *pocketbase.Client, steps []map[string]any) error {
			payload := map[string]any{"status": "completed", "action": businessAction["id"], "revision": businessAction["revision"], "steps": steps}
			_, err := tx.Create(ctx, "business_action_runs", map[string]any{"tenant_id": who(r).Tenant["id"], "app_id": app["id"], "action_id": businessAction["id"], "revision": businessAction["revision"], "idempotency_key": key, "status": "completed", "result": payload})
			return err
		})
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"action": action["id"], "business_action": businessAction["id"], "status": "completed", "steps": result})
		return
	}
	var table map[string]any
	for _, t := range tables {
		if t["slug"] == page["collection"] {
			table = t
			break
		}
	}
	if table == nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	row, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), stringValue(input["record_id"]))
	id := who(r)
	if err != nil || row["tenant_id"] != id.Tenant["id"] || row["app_id"] != app["id"] {
		writeError(w, 404, "记录不存在")
		return
	}
	if stringValue(row["updated"]) != expectedUpdated {
		writeError(w, 409, "记录已变化，请重新读取后操作")
		return
	}
	fresh, err := s.PB.Get(ctx, "apps", stringValue(app["id"]))
	if err != nil || fresh["published_version_id"] != expectedVersion || boolValue(fresh["archived"]) {
		writeError(w, 409, "权限或正式界面已变化")
		return
	}
	saved, err := s.saveBusinessRecord(ctx, recordWrite{Actor: id.actor(stringValue(app["id"]), "interactive"), Table: stringValue(table["slug"]), RecordID: stringValue(row["id"]), ExpectedUpdated: expectedUpdated, PublishedVersion: expectedVersion, Data: asMap(action["set"])}, nil)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"action": action["id"], "record": publicRecord(saved), "status": "completed"})
}
