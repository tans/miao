package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

var appVersionLocks sync.Map

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
	s.Mux.HandleFunc("GET /api/apps/{id}/versions/{versionId}/diff", s.auth(s.diffVersion))
	s.Mux.HandleFunc("POST /api/apps/{id}/runtime/actions/{actionId}", s.auth(s.runRuntimeAction))
	s.Mux.HandleFunc("POST /api/apps/{id}/versions", s.auth(s.createVersion))
	s.Mux.HandleFunc("POST /api/apps/{id}/versions/{versionId}/restore", s.auth(s.restoreVersion))
	s.Mux.HandleFunc("POST /api/apps/{id}/versions/{versionId}/publish", s.auth(s.publishVersion))
}

func (s *Server) loadAppTables(ctx context.Context, app map[string]any, tenantID string) []map[string]any {
	return s.appTables(ctx, app, tenantID)
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
	return map[string]any{"id": version["id"], "version": version["version"], "summary": defaultString(stringValue(version["summary"]), ""), "status": versionStatus(version, publishedID), "based_on_version_id": defaultString(stringValue(version["based_on_version_id"]), ""), "created_at": version["created"], "updated_at": version["updated"], "published_at": version["published_at"]}
}

func validateAppUIDefinition(raw any, tables []map[string]any) (map[string]any, string) {
	definition, ok := raw.(map[string]any)
	if !ok {
		return nil, "界面定义必须是对象"
	}
	version := intValue(definition["schema_version"])
	if version == 1 {
		for key := range definition {
			if !containsString([]string{"schema_version", "title", "collection", "fields"}, key) {
				return nil, "界面定义不支持「" + key + "」；只接受受限的列表界面配置"
			}
		}
		title := strings.TrimSpace(stringValue(definition["title"]))
		if title == "" || len([]rune(title)) > 120 {
			return nil, "界面标题必须为 1 到 120 个字符"
		}
		slug := strings.TrimSpace(stringValue(definition["collection"]))
		var table map[string]any
		for _, item := range tables {
			if item["slug"] == slug {
				table = item
				break
			}
		}
		if table == nil {
			return nil, "界面引用的数据表不存在于当前应用"
		}
		fields := anySlice(definition["fields"])
		if len(fields) < 1 || len(fields) > 12 {
			return nil, "列表界面需要 1 到 12 个字段"
		}
		selected := []string{}
		seen := map[string]bool{}
		for _, rawName := range fields {
			name := stringValue(rawName)
			if seen[name] {
				return nil, "界面字段「" + name + "」重复"
			}
			field := findField(asSliceMap(table["fields"]), name)
			if field == nil || field["type"] == "file" {
				return nil, "界面字段「" + name + "」不存在或暂不支持"
			}
			seen[name] = true
			selected = append(selected, name)
		}
		return map[string]any{"schema_version": 1, "title": title, "collection": slug, "fields": selected}, ""
	}
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
				if !containsString([]string{"id", "label", "set"}, key) {
					return nil, "业务动作无效或重复"
				}
			}
			aid, label := stringValue(action["id"]), strings.TrimSpace(stringValue(action["label"]))
			if !validSlugID(aid, 40) || actionIDs[aid] || label == "" || len([]rune(label)) > 60 {
				return nil, "业务动作无效或重复"
			}
			actionIDs[aid] = true
			set, ok := action["set"].(map[string]any)
			if !ok || len(set) == 0 {
				return nil, "动作需要具体字段赋值"
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
			actions = append(actions, map[string]any{"id": aid, "label": label, "set": set})
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
func appUIPages(definition map[string]any) []map[string]any {
	if intValue(definition["schema_version"]) == 2 {
		return asSliceMap(definition["pages"])
	}
	return []map[string]any{{"id": "main", "title": definition["title"], "collection": definition["collection"], "fields": definition["fields"], "actions": []any{}}}
}

func (s *Server) runtimeForVersion(ctx context.Context, app map[string]any, tenantID string, version map[string]any, query map[string]string, perPageDefault int) (map[string]any, error) {
	tables := s.loadAppTables(ctx, app, tenantID)
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
	formSource := selected
	if intValue(definition["schema_version"]) == 2 {
		formSource = fields
	}
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
	result["definition"] = version["definition"]
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
		current, _ = s.PB.Get(ctx, "app_versions", currentID)
	}
	writeJSON(w, 200, map[string]any{"current_version_id": nilIfEmpty(currentID), "target_version_id": version["id"], "changes": diffAppUI(asMap(current["definition"]), asMap(version["definition"])), "data_changed": false})
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
	preview, err := s.runtimeForVersion(ctx, app, stringValue(who(r).Tenant["id"]), version, map[string]string{"perPage": "5"}, 5)
	if err != nil || preview["status"] != "published" {
		writeError(w, 409, "此版本当前无法预览")
		return
	}
	definition, _ := validateAppUIDefinition(version["definition"], s.appTables(ctx, app, stringValue(who(r).Tenant["id"])))
	pages := appUIPages(definition)
	previews := []map[string]any{}
	for _, page := range pages {
		current := preview
		if page["id"] != preview["ui_page"] {
			current, err = s.runtimeForVersion(ctx, app, stringValue(who(r).Tenant["id"]), version, map[string]string{"perPage": "5", "ui_page": stringValue(page["id"])}, 5)
			if err != nil {
				continue
			}
		}
		previews = append(previews, map[string]any{"id": page["id"], "title": page["title"], "collection": current["collection"], "fields": current["fields"], "actions": current["actions"], "items": current["items"], "total_items": current["total_items"], "relation_labels": current["relation_labels"]})
	}
	currentID := stringValue(app["published_version_id"])
	var currentVersion map[string]any
	if currentID != "" {
		currentVersion, _ = s.PB.Get(ctx, "app_versions", currentID)
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
	definition, msg := validateAppUIDefinition(input["definition"], s.appTables(ctx, app, stringValue(who(r).Tenant["id"])))
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	basedID := stringValue(input["based_on_version_id"])
	var base map[string]any
	if basedID != "" {
		base, err = s.PB.Get(ctx, "app_versions", basedID)
		if err != nil || base["app_id"] != app["id"] || base["tenant_id"] != who(r).Tenant["id"] {
			writeError(w, 404, "修订来源版本不存在")
			return
		}
		if stringValue(base["published_at"]) != "" || base["id"] == app["published_version_id"] {
			writeError(w, 409, "只能从尚未发布的草稿创建修订")
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
	version, err := s.PB.Create(ctx, "app_versions", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "version": number, "definition": definition, "summary": clip(strings.TrimSpace(stringValue(input["summary"])), 1000), "based_on_version_id": basedID, "created_by": id.User["id"]})
	if err != nil {
		writeError(w, 409, "版本序号刚发生变化，请刷新版本列表后重试；原草稿和已发布界面未更改")
		return
	}
	response := publicVersion(version, stringValue(app["published_version_id"]))
	response["definition"] = definition
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
	definition, msg := validateAppUIDefinition(source["definition"], s.appTables(ctx, app, stringValue(who(r).Tenant["id"])))
	if msg != "" {
		writeError(w, 409, fmt.Sprintf("无法从 v%d 创建恢复草稿：%s。当前正式界面未更改，也没有创建草稿。", intValue(source["version"]), msg))
		return
	}
	input := map[string]any{"definition": definition, "summary": fmt.Sprintf("恢复自 v%d", intValue(source["version"])), "based_on_version_id": source["id"]}
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
	input := mapBody(r)
	if _, ok := input["expected_published_version_id"]; !ok {
		writeError(w, 400, "发布时必须提供当前已发布版本，用于检测并发变更")
		return
	}
	unlock := lockAppVersion(stringValue(app["id"]))
	defer unlock()
	latestApp, err := s.PB.Get(ctx, "apps", stringValue(app["id"]))
	if err != nil {
		writeError(w, 503, "应用暂时不可用")
		return
	}
	currentID := stringValue(latestApp["published_version_id"])
	if stringValue(input["expected_published_version_id"]) != currentID {
		writeJSON(w, 409, map[string]any{"error": "应用已被其他操作发布了新版本，请刷新版本列表后重试", "current_version_id": nilIfEmpty(currentID)})
		return
	}
	version, err := s.PB.Get(ctx, "app_versions", pathID(r, "versionId"))
	if err != nil || version["tenant_id"] != who(r).Tenant["id"] || version["app_id"] != app["id"] {
		writeError(w, 404, "草稿版本不存在")
		return
	}
	if currentID == stringValue(version["id"]) {
		result, _ := s.runtimeForVersion(ctx, latestApp, currentID, version, map[string]string{}, 0)
		writeJSON(w, 200, result)
		return
	}
	if stringValue(version["published_at"]) != "" {
		writeError(w, 409, "此版本已经发布或已被替代；如需恢复，请先创建一个新版本")
		return
	}
	if currentID != "" {
		if current, e := s.PB.Get(ctx, "app_versions", currentID); e == nil && intValue(version["version"]) <= intValue(current["version"]) {
			writeError(w, 409, "不能发布早于或等于当前版本的草稿；如需回退，请基于当前版本创建新草稿")
			return
		}
	}
	if _, msg := validateAppUIDefinition(version["definition"], s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))); msg != "" {
		writeError(w, 400, "草稿无法发布："+msg)
		return
	}
	previewApp := map[string]any{}
	for k, v := range latestApp {
		previewApp[k] = v
	}
	previewApp["published_version_id"] = version["id"]
	runtime, err := s.runtimeForVersion(ctx, previewApp, stringValue(who(r).Tenant["id"]), version, map[string]string{}, 0)
	if err != nil || runtime["status"] != "published" {
		writeError(w, 409, "草稿运行检查失败，当前发布版未更改")
		return
	}
	_, err = s.PB.Update(ctx, "apps", stringValue(app["id"]), map[string]any{"published_version_id": version["id"]})
	if err != nil {
		writeError(w, 409, "正式界面未发布，请刷新版本列表后重试")
		return
	}
	_, _ = s.PB.Update(ctx, "app_versions", stringValue(version["id"]), map[string]any{"published_at": nowISO()})
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
	definition, msg := validateAppUIDefinition(version["definition"], s.appTables(ctx, app, stringValue(who(r).Tenant["id"])))
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
	var table map[string]any
	for _, t := range s.appTables(ctx, app, stringValue(who(r).Tenant["id"])) {
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
	saved, err := s.PB.UpdateBusiness(ctx, stringValue(table["pb_collection"]), stringValue(row["id"]), asMap(action["set"]), expectedUpdated, stringValue(id.User["id"]), "interactive")
	if err != nil {
		writeError(w, 409, "记录已变化或动作校验失败")
		return
	}
	s.processRecordAutomation(ctx, stringValue(id.Tenant["id"]), stringValue(app["id"]), stringValue(table["slug"]), "updated", row, saved)
	writeJSON(w, 200, map[string]any{"action": action["id"], "record": publicRecord(saved), "status": "completed"})
}
