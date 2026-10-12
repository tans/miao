package httpapi

import (
	"context"
		"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tans/miao/internal/pocketbase"
	uidoc "github.com/tans/miao/internal/ui"
)

var appVersionLocks sync.Map

func lockAppVersion(id string) func() {
	value, _ := appVersionLocks.LoadOrStore(id, &sync.Mutex{})
	m := value.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}

func (s *Server) validateVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, version, ok := s.loadVersion(ctx, r)
	if !ok {
		writeError(w, 404, "应用界面版本不存在")
		return
	}
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	_, msg := uidoc.ValidateDefinition(version["definition"], tables)
	writeJSON(w, 200, map[string]any{"valid": msg == "", "errors": func() []string {
		if msg == "" {
			return []string{}
		}
		return []string{msg}
	}()})
}





















// Only validated field names/operators reach PocketBase; values are quoted or
// marshalled as scalars rather than accepting an arbitrary filter expression.










func (s *Server) validateBusinessActionReferences(ctx context.Context, app map[string]any, definition map[string]any, requireEnabled bool) string {
	return validateUIActionReferences(ctx, s.PB, app, definition, requireEnabled)
}
func validateUIActionReferences(ctx context.Context, pb *pocketbase.Client, app map[string]any, definition map[string]any, requireEnabled bool) string {
	for _, page := range uidoc.Pages(definition) {
		for _, source := range asSliceMap(page["data_sources"]) {
			for _, action := range asSliceMap(source["actions"]) {
				if workflowID := stringValue(action["workflow_id"]); workflowID != "" {
					row, err := pb.Get(ctx, "workflows", workflowID)
					if err != nil || row["tenant_id"] != app["tenant_id"] || row["app_id"] != app["id"] || row["status"] == "archived" {
						return "页面引用的状态流程不存在或不属于当前应用"
					}
					if requireEnabled && row["status"] != "enabled" {
						return "页面引用的状态流程尚未启用"
					}
					if intValue(action["workflow_revision"]) != intValue(row["revision"]) {
						return "状态流程修订已变化，请重新绑定具体转换后审阅"
					}
					definition := asMap(row["definition"])
					if definition["table"] != source["collection"] || workflowTransition(definition, stringValue(action["transition_id"])) == nil {
						return "状态流程的数据表或转换与页面绑定不匹配"
					}
					continue
				}
				actionID := stringValue(action["action_id"])
				if actionID == "" {
					continue
				}
				row, err := pb.Get(ctx, "business_actions", actionID)
				if err != nil || row["tenant_id"] != app["tenant_id"] || row["app_id"] != app["id"] || row["status"] == "archived" {
					return "页面引用的通用业务动作不存在或不属于当前应用"
				}
				if requireEnabled && row["status"] != "enabled" {
					return "页面引用的通用业务动作尚未启用"
				}
				if action["action_revision"] == nil {
					if requireEnabled {
						return "业务动作缺少具体修订，请保存新的界面草稿后审阅发布"
					}
					action["action_revision"] = intValue(row["revision"])
				}
				if action["action_revision"] != nil && intValue(action["action_revision"]) != intValue(row["revision"]) {
					return "业务动作修订已变化，请重新绑定后审阅"
				}
			}
		}
	}
	return ""
}




// Detail routing is derived from the validated component/source binding, not
// a display title or a naming convention that the user can change.



func (s *Server) runtimeForVersion(ctx context.Context, app map[string]any, tenantID string, version map[string]any, query map[string]string, perPageDefault int) (map[string]any, error) {
	tables, err := s.appTables(ctx, app, tenantID)
	if err != nil {
		return nil, err
	}
	definition, errText := uidoc.ValidateDefinition(version["definition"], tables)
	if errText != "" {
		return map[string]any{"status": "unavailable"}, nil
	}
	pages := uidoc.Pages(definition)
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
	return s.runtimeForSpec(ctx, app, tenantID, version, definition, pages, page, query, perPageDefault, tables)
}

func (s *Server) runtimeForSpec(ctx context.Context, app map[string]any, tenantID string, version map[string]any, definition map[string]any, pages []map[string]any, page map[string]any, query map[string]string, perPageDefault int, tables []map[string]any) (map[string]any, error) {
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
	search := clip(strings.TrimSpace(query["search"]), 120)
	recordID := strings.TrimSpace(query["record_id"])
	sources := map[string]any{}
	tenant, err := s.PB.Get(ctx, "tenants", tenantID)
	if err != nil {
		return nil, err
	}
	memberRows, err := s.PB.ListAll(ctx, "tenant_members", "tenant_id = "+pbFilterString(tenantID), "created")
	if err != nil {
		return nil, err
	}
	members, err := s.appMemberChoices(ctx, app, tenant, memberRows)
	if err != nil {
		return nil, err
	}
	relationOptions := func(field map[string]any) []map[string]any {
		if stringValue(field["type"]) != "relation" {
			return nil
		}
		var target map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == field["target"] {
				target = candidate
				break
			}
		}
		if target == nil {
			return nil
		}
		labelField := ""
		for _, candidate := range asSliceMap(target["fields"]) {
			if candidate["type"] == "text" {
				labelField = stringValue(candidate["name"])
				break
			}
		}
		rows, _, _, listErr := s.PB.List(ctx, stringValue(target["pb_collection"]), listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(stringValue(app["id"]))), "-created", 1, 100)
		if listErr != nil {
			return nil
		}
		options := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			label := stringValue(row[labelField])
			if label == "" {
				label = stringValue(row["id"])
			}
			options = append(options, map[string]any{"id": row["id"], "label": label})
		}
		return options
	}
	var contextRecord map[string]any
	for _, source := range asSliceMap(page["data_sources"]) {
		if source["context"] != nil {
			var err error
			contextRecord, err = uiContextRecord(ctx, s.PB, app, page, tables, recordID)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	for _, rawSource := range asSliceMap(page["data_sources"]) {
		sourceID := stringValue(rawSource["id"])
		collection := stringValue(rawSource["collection"])
		contextField := stringValue(asMap(rawSource["context"])["field"])
		contextAvailable := contextField == "" || contextRecord != nil
		defaults := map[string]any{}
		if contextField != "" && contextRecord != nil {
			defaults[contextField] = contextRecord["id"]
		}
		var table map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == collection {
				table = candidate
				break
			}
		}
		if table == nil {
			return map[string]any{"status": "unavailable"}, nil
		}
		allFields := asSliceMap(table["fields"])
		selected := []map[string]any{}
		for _, rawName := range anySlice(rawSource["fields"]) {
			if field := findField(allFields, stringValue(rawName)); field != nil {
				copy := map[string]any{}
				for k, v := range field {
					copy[k] = v
				}
				copy["label"] = defaultString(stringValue(field["label"]), stringValue(field["name"]))
				if options := relationOptions(copy); options != nil {
					if copy["name"] == contextField {
						options = uidoc.AppendContextOption(options, contextRecord)
					}
					copy["relation_options"] = options
				}
				selected = append(selected, copy)
			}
		}
		parts := []string{"tenant_id = " + pbFilterString(tenantID), "app_id = " + pbFilterString(stringValue(app["id"]))}
		parts = append(parts, uidoc.QueryFilter(rawSource)...)
		if contextField != "" {
			value := "__missing_record__"
			if contextRecord != nil {
				value = stringValue(contextRecord["id"])
			}
			parts = append(parts, contextField+" = "+pbFilterString(value))
		}
		if uidoc.DetailSource(page) == sourceID && recordID != "" {
			parts = append(parts, "id = "+pbFilterString(recordID))
		} else if uidoc.DetailSource(page) == sourceID {
			parts = append(parts, "id = \"__missing_record__\"")
		}
		alts := []string{}
		for _, field := range selected {
			if contains([]any{"text", "email", "url"}, field["type"]) {
				alts = append(alts, stringValue(field["name"])+" ~ "+pbFilterString(search))
			}
		}
		if search != "" && len(alts) > 0 && sourceID != uidoc.DetailSource(page) {
			parts = append(parts, "("+strings.Join(alts, " || ")+")")
		}
		sourcePage := pageNum
		if sourceID == uidoc.DetailSource(page) {
			sourcePage = 1
		}
		rows, total, totalPages, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(parts...), uidoc.QuerySort(rawSource), sourcePage, per)
		if err != nil {
			return nil, err
		}
		items := []map[string]any{}
		relationLabels := map[string]map[string]string{}
		for _, field := range selected {
			if field["type"] == "member" {
				labels := map[string]string{}
				for _, member := range members {
					labels[stringValue(member["id"])] = stringValue(member["label"])
				}
				relationLabels[stringValue(field["name"])] = labels
			}
		}
		for _, row := range rows {
			data := map[string]any{}
			for _, field := range selected {
				name := stringValue(field["name"])
				value := row[name]
				data[name] = value
				if field["type"] != "relation" || value == nil || stringValue(value) == "" {
					continue
				}
				var target map[string]any
				for _, candidate := range tables {
					if candidate["slug"] == field["target"] {
						target = candidate
						break
					}
				}
				if target == nil {
					continue
				}
				related, getErr := s.PB.Get(ctx, stringValue(target["pb_collection"]), stringValue(value))
				if getErr != nil || related["tenant_id"] != tenantID || related["app_id"] != app["id"] {
					continue
				}
				label := stringValue(value)
				for _, targetField := range asSliceMap(target["fields"]) {
					if targetField["type"] == "text" && stringValue(related[stringValue(targetField["name"])]) != "" {
						label = stringValue(related[stringValue(targetField["name"])])
						break
					}
				}
				if relationLabels[name] == nil {
					relationLabels[name] = map[string]string{}
				}
				relationLabels[name][stringValue(value)] = label
			}
			items = append(items, map[string]any{"id": row["id"], "data": data, "created_at": row["created"], "updated_at": row["updated"]})
		}
		form := []map[string]any{}
		requiredPresent := true
		formFields := allFields
		if names, exists := rawSource["form_fields"]; exists {
			formFields = []map[string]any{}
			for _, name := range anySlice(names) {
				if field := findField(allFields, stringValue(name)); field != nil {
					formFields = append(formFields, field)
				}
			}
		}
		for _, field := range formFields {
			typ := stringValue(field["type"])
			if !contains([]any{"text", "number", "bool", "date", "email", "url", "select", "relation", "file", "member"}, typ) {
				continue
			}
			copy := map[string]any{}
			for k, v := range field {
				copy[k] = v
			}
			if options := relationOptions(copy); options != nil {
				if copy["name"] == contextField {
					options = uidoc.AppendContextOption(options, contextRecord)
				}
				copy["relation_options"] = options
			}
			form = append(form, copy)
		}
		for _, field := range allFields {
			if boolValue(field["required"]) && findField(form, stringValue(field["name"])) == nil && defaults[stringValue(field["name"])] == nil {
				requiredPresent = false
			}
		}
		actions, err := s.runtimeUIActions(ctx, app, rawSource)
		if err != nil {
			return nil, err
		}
		sources[sourceID] = map[string]any{"id": sourceID, "collection": collection, "fields": selected, "actions": actions, "items": items, "total_items": total, "total_pages": totalPages, "page": sourcePage, "per_page": per, "search_supported": len(alts) > 0, "create_form_available": len(form) > 0 && requiredPresent && contextAvailable, "create_form_fields": form, "create_defaults": defaults, "context_available": contextAvailable, "relation_labels": relationLabels}
	}
	pageList := []map[string]any{}
	for _, candidate := range pages {
		dataSources := asSliceMap(candidate["data_sources"])
		collection := ""
		if len(dataSources) > 0 {
			collection = stringValue(dataSources[0]["collection"])
		}
		detailSource := uidoc.DetailSource(candidate)
		if detailSource != "" {
			for _, source := range dataSources {
				if source["id"] == detailSource {
					collection = stringValue(source["collection"])
				}
			}
		}
		pageList = append(pageList, map[string]any{"id": candidate["id"], "title": candidate["title"], "collection": collection, "detail_source": detailSource})
	}
	result := map[string]any{"status": "published", "version": uidoc.Public(version, stringValue(version["id"])), "title": page["title"], "app_title": definition["title"], "ui_page": page["id"], "record_id": recordID, "pages": pageList, "definition": definition, "sources": sources, "members": members, "read_only": false}
	if len(asSliceMap(page["data_sources"])) > 0 {
		first := stringValue(asSliceMap(page["data_sources"])[0]["id"])
		if firstSource, ok := sources[first].(map[string]any); ok {
			for _, key := range []string{"collection", "fields", "actions", "items", "total_items", "total_pages", "page", "per_page", "search_supported", "create_form_available", "create_form_fields"} {
				result[key] = firstSource[key]
			}
		}
	}
	return result, nil
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
	for _, key := range []string{"ui_page", "page", "perPage", "search", "record_id"} {
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
		items = append(items, uidoc.Public(row, stringValue(app["published_version_id"])))
	}
	writeJSON(w, 200, map[string]any{"published_version_id": uidoc.NilIfEmpty(stringValue(app["published_version_id"])), "page": page, "per_page": 50, "total_items": total, "items": items})
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
	result := uidoc.Public(version, stringValue(app["published_version_id"]))
	result["definition"] = version["definition"]
	writeJSON(w, 200, result)
}
func (s *Server) activateVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canPublishAppRole(role) || boolValue(app["archived"]) {
		writeError(w, 403, "没有发布权限")
		return
	}
	versionID := pathID(r, "versionId")
	version, err := s.PB.Get(ctx, "app_versions", versionID)
	if err != nil || version["tenant_id"] != who(r).Tenant["id"] || version["app_id"] != app["id"] {
		writeError(w, 404, "应用界面版本不存在")
		return
	}
	if stringValue(version["published_at"]) == "" {
		writeError(w, 409, "只能切换到已经发布的版本")
		return
	}
	input := mapBody(r)
	expected := stringValue(input["expected_published_version_id"])
	actor := who(r).actor(stringValue(app["id"]), "interactive")
	unlock := lockAppVersion(stringValue(app["id"]))
	defer unlock()

	tables, err := s.appTables(ctx, app, actor.TenantID)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	if _, msg := uidoc.ValidateDefinition(version["definition"], tables); msg != "" {
		writeError(w, 409, "目标版本无法运行："+msg)
		return
	}
	if msg := s.validateBusinessActionReferences(ctx, app, asMap(version["definition"]), true); msg != "" {
		writeError(w, 409, "目标版本无法运行："+msg)
		return
	}
	previewApp := cloneAnyMap(app)
	previewApp["published_version_id"] = versionID
	runtime, err := s.runtimeForVersion(ctx, previewApp, actor.TenantID, version, map[string]string{}, 0)
	if err != nil || runtime["status"] != "published" {
		writeError(w, 409, "目标版本运行检查失败，当前发布版未更改")
		return
	}
	var previousID string
	err = s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		fresh, err := tx.Get(ctx, "apps", actor.AppID)
		if err != nil {
			return err
		}
		previousID = stringValue(fresh["published_version_id"])
		if expected != "" && expected != previousID {
			return businessError(409, "正式界面已变化，请刷新后再切换版本")
		}
		if previousID == versionID {
			return nil
		}
		if _, err := s.authorizeWrite(ctx, tx, actor, false); err != nil {
			return err
		}
		user, err := tx.Get(ctx, "users", actor.UserID)
		if err != nil {
			return err
		}
		tenant, err := tx.Get(ctx, "tenants", actor.TenantID)
		if err != nil {
			return err
		}
		member, err := tx.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "user_id = "+pbFilterString(actor.UserID)))
		if err != nil {
			return err
		}
		access, err := applicationAccess(ctx, tx, fresh, identity{User: user, Tenant: tenant, Membership: member})
		if err != nil {
			return err
		}
		if !access.Role.canPublish() {
			return businessError(403, "没有发布权限")
		}
		if publication := asMap(fresh["public_publication"]); boolValue(publication["enabled"]) {
			txTables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID)), "created")
			if err != nil {
				return err
			}
			if _, message := normalizePublicPages(version, publication["pages"], txTables); message != "" {
				return businessError(409, "目标版本与当前公开配置不兼容："+message)
			}
		}
		_, err = tx.Update(ctx, "apps", actor.AppID, map[string]any{"published_version_id": versionID})
		return err
	})
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	app["published_version_id"] = versionID
	runtime["switched"] = previousID != versionID
	runtime["previous_version_id"] = uidoc.NilIfEmpty(previousID)
	writeJSON(w, 200, runtime)
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
	changes := uidoc.Diff(asMap(current["definition"]), asMap(version["definition"]))
	writeJSON(w, 200, map[string]any{"current_version_id": uidoc.NilIfEmpty(currentID), "target_version_id": version["id"], "changes": changes, "data_changed": false})
}



func (s *Server) previewVersion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	app, version, ok := s.loadVersion(ctx, r)
	if !ok {
		writeError(w, 404, "应用界面版本不存在")
		return
	}
	preview, err := s.runtimeForVersion(ctx, app, stringValue(who(r).Tenant["id"]), version, map[string]string{"perPage": "5", "ui_page": r.URL.Query().Get("ui_page"), "record_id": r.URL.Query().Get("record_id")}, 5)
	if err != nil || preview["status"] != "published" {
		writeError(w, 409, "此版本当前无法预览")
		return
	}
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	definition, msg := uidoc.ValidateDefinition(version["definition"], tables)
	if msg != "" {
		writeError(w, 409, msg)
		return
	}
	if msg = s.validateBusinessActionReferences(ctx, app, definition, false); msg != "" {
		writeError(w, 409, msg)
		return
	}
	pages := uidoc.Pages(definition)
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
		previews = append(previews, map[string]any{"id": page["id"], "title": page["title"], "collection": current["collection"], "fields": current["fields"], "actions": current["actions"], "items": current["items"], "total_items": current["total_items"], "relation_labels": current["relation_labels"], "sources": current["sources"]})
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
	preview["current_version_id"] = uidoc.NilIfEmpty(currentID)
	preview["page_previews"], preview["status"], preview["version"], preview["changes"], preview["note"] = previews, "preview", uidoc.Public(version, currentID), uidoc.Diff(asMap(currentVersion["definition"]), definition), "仅界面定义变更，业务记录不回滚"
	writeJSON(w, 200, preview)
}

func (s *Server) previewUIDefinition(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(role) {
		writeError(w, 403, "你没有编辑界面的权限")
		return
	}
	input := mapBody(r)
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	definition, msg := uidoc.ValidateDefinition(input["definition"], tables)
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	if msg = s.validateBusinessActionReferences(ctx, app, definition, false); msg != "" {
		writeError(w, 400, msg)
		return
	}
	version := map[string]any{"definition": definition, "version": 0}
	runtime, err := s.runtimeForVersion(ctx, app, stringValue(who(r).Tenant["id"]), version, map[string]string{"ui_page": stringValue(input["ui_page"]), "record_id": stringValue(input["record_id"]), "perPage": "5"}, 5)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	var published map[string]any
	if current := stringValue(app["published_version_id"]); current != "" {
		published, err = s.PB.Get(ctx, "app_versions", current)
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
	}
	runtime["status"], runtime["read_only"], runtime["changes"] = "preview", true, uidoc.Diff(asMap(published["definition"]), definition)
	runtime["current_version_id"] = uidoc.NilIfEmpty(stringValue(app["published_version_id"]))
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
	definition, msg := uidoc.ValidateDefinition(version["definition"], tables)
	if msg != "" {
		writeError(w, 409, msg)
		return
	}
	var page map[string]any
	for _, p := range uidoc.Pages(definition) {
		if p["id"] == input["ui_page"] {
			page = p
			break
		}
	}
	if page == nil {
		writeError(w, 404, "业务动作不存在")
		return
	}
	var action, actionSource map[string]any
	for _, source := range asSliceMap(page["data_sources"]) {
		for _, candidate := range asSliceMap(source["actions"]) {
			if candidate["id"] == pathID(r, "actionId") {
				action, actionSource = candidate, source
				break
			}
		}
		if action != nil {
			break
		}
	}
	if action == nil {
		writeError(w, 404, "业务动作不存在")
		return
	}
	guard := s.runtimeActionGuard(ctx, who(r), app, page, actionSource, tables, expectedVersion, stringValue(input["record_id"]), expectedUpdated, stringValue(input["context_record_id"]))
	if workflowID := stringValue(action["workflow_id"]); workflowID != "" {
		workflow, err := s.PB.Get(ctx, "workflows", workflowID)
		if err != nil || workflow["tenant_id"] != who(r).Tenant["id"] || workflow["app_id"] != app["id"] || intValue(workflow["revision"]) != intValue(action["workflow_revision"]) || asMap(workflow["definition"])["table"] != actionSource["collection"] {
			writeError(w, 409, "流程绑定或修订已变化，请重新审阅发布")
			return
		}
		payload := cloneAnyMap(input)
		payload["transition_id"] = action["transition_id"]
		payload["idempotency_key"] = runtimeActionKey(who(r), expectedVersion, action, input)
		result, err := s.executeWorkflowTransition(ctx, who(r), app, workflow, payload, "interactive", guard)
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
		writeJSON(w, 200, result)
		return
	}
	if actionID := stringValue(action["action_id"]); actionID != "" {
		businessAction, err := s.PB.Get(ctx, "business_actions", actionID)
		if err != nil || businessAction["tenant_id"] != who(r).Tenant["id"] || businessAction["app_id"] != app["id"] || businessAction["status"] != "enabled" {
			writeError(w, 409, "引用的通用业务动作不存在、未启用或已失效")
			return
		}
		if intValue(action["action_revision"]) != intValue(businessAction["revision"]) {
			writeError(w, 409, "业务动作修订已变化，请重新绑定后审阅发布")
			return
		}
		inputValues := cloneAnyMap(asMap(input["input"]))
		if inputValues == nil {
			inputValues = map[string]any{}
		}
		inputValues["record_id"] = input["record_id"]
		inputValues["record_updated_at"] = expectedUpdated
		key := runtimeActionKey(who(r), expectedVersion, action, input)
		if previous, findErr := s.PB.Find(ctx, "business_action_runs", listFilter("action_id = "+pbFilterString(actionID), "idempotency_key = "+pbFilterString(key))); findErr == nil {
			writeJSON(w, 200, previous["result"])
			return
		} else if !isMissing(findErr) {
			s.writeBusinessError(w, findErr)
			return
		}
		result, err := s.executeActionSteps(ctx, who(r), app, businessAction, asMap(businessAction["definition"]), inputValues, "interactive", func(tx *pocketbase.Client, steps []map[string]any) error {
			payload := map[string]any{"status": "completed", "action": businessAction["id"], "revision": businessAction["revision"], "steps": steps}
			_, err := tx.Create(ctx, "business_action_runs", map[string]any{"tenant_id": who(r).Tenant["id"], "app_id": app["id"], "action_id": businessAction["id"], "revision": businessAction["revision"], "idempotency_key": key, "status": "completed", "result": payload})
			return err
		}, guard)
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
		writeJSON(w, 200, map[string]any{"action": action["id"], "business_action": businessAction["id"], "status": "completed", "steps": result})
		return
	}
	var table map[string]any
	tableSlug := stringValue(page["collection"])
	if actionSource != nil {
		tableSlug = stringValue(actionSource["collection"])
	}
	for _, candidate := range tables {
		if candidate["slug"] == tableSlug {
			table = candidate
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
	saved, err := s.saveBusinessRecord(ctx, recordWrite{Actor: id.actor(stringValue(app["id"]), "interactive"), Table: stringValue(table["slug"]), RecordID: stringValue(row["id"]), ExpectedUpdated: expectedUpdated, PublishedVersion: expectedVersion, Data: asMap(action["set"])}, nil, guard)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"action": action["id"], "record": publicRecord(saved), "status": "completed"})
}
