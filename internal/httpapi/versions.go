package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/pocketbase"
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
	source := asMap(version["source"])
	submittedBy := asMap(source["submitted_by"])
	return map[string]any{
		"id": version["id"], "version": version["version"],
		"summary":             defaultString(stringValue(version["summary"]), ""),
		"status":              versionStatus(version, publishedID),
		"based_on_version_id": defaultString(stringValue(version["based_on_version_id"]), ""),
		"created_at":          version["created"], "updated_at": version["updated"], "published_at": version["published_at"],
		"created_by": version["created_by"], "created_by_name": defaultString(stringValue(submittedBy["name"]), stringValue(submittedBy["email"])),
		"created_by_email": submittedBy["email"], "change_request": source["request"],
	}
}

func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func validateAppUIDefinition(raw any, tables []map[string]any) (map[string]any, string) {
	definition, ok := raw.(map[string]any)
	if !ok {
		return nil, "界面定义必须是对象"
	}
	if intValue(definition["schema_version"]) != 3 {
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
		if page == nil {
			return nil, "页面配置无效"
		}
		for key := range page {
			if !containsString([]string{"id", "title", "spec", "data_sources"}, key) {
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
		sources, msg := validateUISources(page["data_sources"], tables)
		if msg != "" {
			return nil, msg
		}
		actions := map[string]bool{}
		for _, source := range sources {
			for _, action := range asSliceMap(source["actions"]) {
				id := stringValue(action["id"])
				if actions[id] {
					return nil, "同一页面的数据源动作标识不能重复"
				}
				actions[id] = true
			}
		}
		spec, msg := validateUISpec(page["spec"], sources)
		if msg != "" {
			return nil, msg
		}
		if msg := validateUIRecordContexts(spec, sources, tables); msg != "" {
			return nil, msg
		}
		pages = append(pages, map[string]any{"id": pid, "title": ptitle, "spec": spec, "data_sources": sources})
	}
	return map[string]any{"schema_version": 3, "title": title, "pages": pages}, ""
}

func validateUISources(raw any, tables []map[string]any) ([]map[string]any, string) {
	items := anySlice(raw)
	if len(items) == 0 || len(items) > 12 {
		return nil, "页面需要 1–12 个数据源"
	}
	sources := make([]map[string]any, 0, len(items))
	seen := map[string]bool{}
	for _, value := range items {
		item := asMap(value)
		if item == nil || len(item) < 3 || len(item) > 7 {
			return nil, "数据源配置无效"
		}
		for key := range item {
			if !containsString([]string{"id", "collection", "fields", "actions", "query", "form_fields", "context"}, key) {
				return nil, "数据源配置无效"
			}
		}
		id := stringValue(item["id"])
		if !validSlugID(id, 40) || seen[id] {
			return nil, "数据源标识无效或重复"
		}
		seen[id] = true
		var table map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == item["collection"] {
				table = candidate
				break
			}
		}
		if table == nil {
			return nil, "数据源引用的数据表不存在"
		}
		fields := []string{}
		used := map[string]bool{}
		for _, rawField := range anySlice(item["fields"]) {
			field, ok := rawField.(string)
			if !ok || used[field] || findField(asSliceMap(table["fields"]), field) == nil {
				return nil, "数据源字段无效或重复"
			}
			used[field] = true
			fields = append(fields, field)
		}
		if len(fields) == 0 || len(fields) > 24 {
			return nil, "数据源字段数量无效"
		}
		if !isUIArray(item["fields"]) || (item["actions"] != nil && !isUIArray(item["actions"])) {
			return nil, "字段和动作必须为数组"
		}
		actions := []map[string]any{}
		actionIDs := map[string]bool{}
		if len(anySlice(item["actions"])) > 24 {
			return nil, "每个数据源最多 24 个动作"
		}
		for _, rawAction := range anySlice(item["actions"]) {
			action := asMap(rawAction)
			if action == nil || len(action) < 2 || len(action) > 7 || !validSlugID(stringValue(action["id"]), 40) || strings.TrimSpace(stringValue(action["label"])) == "" {
				return nil, "数据源动作无效"
			}
			for key := range action {
				if !containsString([]string{"id", "label", "set", "action_id", "action_revision", "workflow_id", "workflow_revision", "transition_id"}, key) {
					return nil, "数据源动作无效"
				}
			}
			actionID := stringValue(action["id"])
			if actionIDs[actionID] || len([]rune(stringValue(action["label"]))) > 120 {
				return nil, "动作标识重复或标签过长"
			}
			actionIDs[actionID] = true
			set := asMap(action["set"])
			kinds := 0
			for _, present := range []bool{len(set) > 0, stringValue(action["action_id"]) != "", stringValue(action["workflow_id"]) != ""} {
				if present {
					kinds++
				}
			}
			if kinds != 1 {
				return nil, "动作需要唯一的字段更新、业务动作或流程转换引用"
			}
			if action["action_revision"] != nil && (stringValue(action["action_id"]) == "" || intValue(action["action_revision"]) < 1) {
				return nil, "业务动作修订引用无效"
			}
			if stringValue(action["workflow_id"]) != "" {
				if stringValue(action["transition_id"]) == "" || intValue(action["workflow_revision"]) < 1 {
					return nil, "流程动作需要转换标识与具体修订"
				}
			} else if action["workflow_revision"] != nil || action["transition_id"] != nil {
				return nil, "只有流程动作可以配置转换和流程修订"
			}
			for key, value := range set {
				field := findField(asSliceMap(table["fields"]), key)
				if field == nil || !containsString([]string{"text", "number", "bool", "date", "email", "url", "select"}, stringValue(field["type"])) {
					return nil, "动作只能更新当前表的普通字段"
				}
				if value != nil {
					switch field["type"] {
					case "number":
						if _, ok := value.(float64); !ok {
							if _, ok := value.(int); !ok {
								return nil, "动作数值无效"
							}
						}
					case "bool":
						if _, ok := value.(bool); !ok {
							return nil, "动作布尔值无效"
						}
					default:
						if _, ok := value.(string); !ok {
							return nil, "动作字段值无效"
						}
					}
				}
				if field["type"] == "select" && !containsString(stringSlice(anySlice(field["options"])), stringValue(value)) {
					return nil, "动作选项不在当前字段中"
				}
			}
			actions = append(actions, action)
		}
		source := map[string]any{"id": id, "collection": item["collection"], "fields": fields, "actions": actions}
		if raw, exists := item["context"]; exists {
			binding := asMap(raw)
			if len(binding) != 2 || !validSlugID(stringValue(binding["source"]), 40) || !validSlugID(stringValue(binding["field"]), 60) {
				return nil, "关联上下文需要真实详情数据源与关联字段"
			}
			source["context"] = map[string]any{"source": binding["source"], "field": binding["field"]}
		}
		if raw, exists := item["form_fields"]; exists {
			names := []string{}
			selected := map[string]bool{}
			if !isUIArray(raw) {
				return nil, "表单字段必须为数组"
			}
			for _, value := range anySlice(raw) {
				name, ok := value.(string)
				if !ok || selected[name] || findField(asSliceMap(table["fields"]), name) == nil {
					return nil, "表单字段无效或重复"
				}
				selected[name] = true
				names = append(names, name)
			}
			if len(names) > 24 {
				return nil, "表单字段过多"
			}
			source["form_fields"] = names
		}
		if raw, exists := item["query"]; exists {
			query, msg := validateUIQuery(raw, asSliceMap(table["fields"]))
			if msg != "" {
				return nil, msg
			}
			source["query"] = query
		}
		sources = append(sources, source)
	}
	return sources, ""
}

func isUIArray(raw any) bool {
	switch raw.(type) {
	case []any, []string, []map[string]any:
		return true
	}
	return false
}

func validateUIQuery(raw any, fields []map[string]any) (map[string]any, string) {
	query, ok := raw.(map[string]any)
	if !ok || query == nil {
		return nil, "数据源查询必须为对象"
	}
	for key := range query {
		if !containsString([]string{"filters", "sort"}, key) {
			return nil, "不支持的数据源查询属性"
		}
	}
	filters := []any{}
	if raw, exists := query["filters"]; exists && !isUIArray(raw) {
		return nil, "筛选必须为数组"
	}
	if len(anySlice(query["filters"])) > 8 {
		return nil, "最多 8 个筛选条件"
	}
	for _, raw := range anySlice(query["filters"]) {
		filter := asMap(raw)
		name, op := stringValue(filter["field"]), stringValue(filter["op"])
		if len(filter) != 3 {
			return nil, "筛选需要 field、op 和 value"
		}
		field := findField(fields, name)
		if field == nil || !containsString([]string{"text", "email", "url", "select", "date", "number", "bool"}, stringValue(field["type"])) || !containsString([]string{"eq", "neq", "contains"}, op) {
			return nil, "筛选字段或运算无效"
		}
		value := filter["value"]
		switch field["type"] {
		case "number":
			if _, ok := value.(float64); !ok {
				return nil, "筛选数值无效"
			}
		case "bool":
			if _, ok := value.(bool); !ok {
				return nil, "筛选布尔值无效"
			}
		default:
			text, ok := value.(string)
			if !ok || len([]rune(text)) > 240 {
				return nil, "筛选文本无效或过长"
			}
		}
		if op == "contains" && !containsString([]string{"text", "email", "url"}, stringValue(field["type"])) {
			return nil, "包含筛选只支持文本字段"
		}
		filters = append(filters, cloneAnyMap(filter))
	}
	sort := stringValue(query["sort"])
	if sort != "" && !containsString([]string{"created", "-created", "updated", "-updated"}, sort) {
		field := findField(fields, strings.TrimPrefix(sort, "-"))
		if field == nil || !containsString([]string{"text", "number", "bool", "date", "email", "url", "select"}, stringValue(field["type"])) {
			return nil, "排序字段无效"
		}
	}
	return map[string]any{"filters": filters, "sort": defaultString(sort, "-created")}, ""
}

// Only validated field names/operators reach PocketBase; values are quoted or
// marshalled as scalars rather than accepting an arbitrary filter expression.
func uiQueryFilter(source map[string]any) []string {
	parts := []string{}
	for _, filter := range asSliceMap(asMap(source["query"])["filters"]) {
		literal := ""
		if text, ok := filter["value"].(string); ok {
			literal = pbFilterString(text)
		} else {
			data, _ := json.Marshal(filter["value"])
			literal = string(data)
		}
		operator := map[string]string{"eq": "=", "neq": "!=", "contains": "~"}[stringValue(filter["op"])]
		parts = append(parts, stringValue(filter["field"])+" "+operator+" "+literal)
	}
	return parts
}
func uiQuerySort(source map[string]any) string {
	return defaultString(stringValue(asMap(source["query"])["sort"]), "-created")
}

func validateUISpec(raw any, sources []map[string]any) (map[string]any, string) {
	spec := asMap(raw)
	if len(spec) == 0 || len(spec) > 3 {
		return nil, "界面 Spec 无效"
	}
	for key := range spec {
		if !containsString([]string{"root", "elements"}, key) {
			return nil, "界面 Spec 只支持 root 和 elements"
		}
	}
	elements := asMap(spec["elements"])
	root := stringValue(spec["root"])
	if len(elements) == 0 || len(elements) > 80 || elements[root] == nil {
		return nil, "界面 Spec 根节点或元素数量无效"
	}
	sourceIDs := map[string]bool{}
	for _, source := range sources {
		sourceIDs[stringValue(source["id"])] = true
	}
	references := map[string]bool{}
	for id, rawElement := range elements {
		if !validSlugID(id, 64) {
			return nil, "界面元素标识无效"
		}
		element := asMap(rawElement)
		if len(element) == 0 || len(element) > 3 {
			return nil, "界面元素配置无效"
		}
		for key := range element {
			if !containsString([]string{"type", "props", "children"}, key) {
				return nil, "界面元素配置无效"
			}
		}
		typ := stringValue(element["type"])
		props := asMap(element["props"])
		if props == nil || len(props) > 8 {
			return nil, "界面组件参数无效"
		}
		allowed := map[string][]string{"Page": {"title"}, "Section": {"title"}, "Text": {"text"}, "Metric": {"label", "value"}, "RecordTable": {"source", "title"}, "RecordCards": {"source", "title"}, "RecordDetail": {"source", "title"}, "RecordForm": {"source", "title"}}
		keys, ok := allowed[typ]
		if !ok {
			return nil, "界面组件不在受控目录中"
		}
		for key, value := range props {
			if !containsString(keys, key) {
				return nil, "界面组件参数不在受控目录中"
			}
			if binding, ok := value.(map[string]any); ok {
				path := stringValue(binding["$state"])
				parts := strings.Split(path, "/")
				if len(binding) != 1 || len(parts) != 4 || parts[0] != "" || parts[1] != "sources" || !sourceIDs[parts[2]] || !containsString([]string{"total_items", "title"}, parts[3]) {
					return nil, "界面数据绑定无效"
				}
			} else if _, ok := value.(string); !ok {
				return nil, "界面组件参数必须是文本或受控绑定"
			}
			if key == "source" && (typ == "RecordDetail" || typ == "RecordTable" || typ == "RecordCards" || typ == "RecordForm") && !validSlugID(stringValue(value), 40) {
				return nil, "数据源引用必须使用声明的稳定标识"
			}
		}
		if strings.HasPrefix(typ, "Record") {
			source := stringValue(props["source"])
			if !sourceIDs[source] {
				return nil, "界面组件引用未知数据源"
			}
			references[source] = true
		} else if props["source"] != nil {
			return nil, "界面组件不能引用数据源"
		}
		children := anySlice(element["children"])
		if len(children) > 32 || (element["children"] != nil && !isUIArray(element["children"])) {
			return nil, "界面组件子节点无效"
		}
		if len(children) > 0 && typ != "Page" && typ != "Section" {
			return nil, "界面组件不支持子节点"
		}
		for _, rawChild := range children {
			child, ok := rawChild.(string)
			if !ok || elements[child] == nil {
				return nil, "界面组件引用未知子节点"
			}
		}
	}
	if len(references) != len(sourceIDs) {
		return nil, "页面数据源必须在界面中使用"
	}
	visited, visiting := map[string]bool{}, map[string]bool{}
	var walk func(string) bool
	walk = func(id string) bool {
		if visiting[id] || visited[id] {
			return false
		}
		visiting[id] = true
		for _, child := range anySlice(asMap(elements[id])["children"]) {
			if !walk(stringValue(child)) {
				return false
			}
		}
		visiting[id] = false
		visited[id] = true
		return true
	}
	if !walk(root) || len(visited) != len(elements) {
		return nil, "界面 Spec 包含循环、共享或不可达元素"
	}
	return map[string]any{"root": root, "elements": elements}, ""
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
	return validateUIActionReferences(ctx, s.PB, app, definition, requireEnabled)
}
func validateUIActionReferences(ctx context.Context, pb *pocketbase.Client, app map[string]any, definition map[string]any, requireEnabled bool) string {
	for _, page := range appUIPages(definition) {
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

func appUIPages(definition map[string]any) []map[string]any { return asSliceMap(definition["pages"]) }

// Detail routing is derived from the validated component/source binding, not
// a display title or a naming convention that the user can change.
func uiDetailSource(page map[string]any) string {
	spec := asMap(page["spec"])
	elements := asMap(spec["elements"])
	queue := []string{stringValue(spec["root"])}
	for len(queue) > 0 {
		element := asMap(elements[queue[0]])
		queue = queue[1:]
		if element["type"] == "RecordDetail" {
			return stringValue(asMap(element["props"])["source"])
		}
		queue = append(queue, stringSlice(anySlice(element["children"]))...)
	}
	return ""
}

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
						options = appendContextOption(options, contextRecord)
					}
					copy["relation_options"] = options
				}
				selected = append(selected, copy)
			}
		}
		parts := []string{"tenant_id = " + pbFilterString(tenantID), "app_id = " + pbFilterString(stringValue(app["id"]))}
		parts = append(parts, uiQueryFilter(rawSource)...)
		if contextField != "" {
			value := "__missing_record__"
			if contextRecord != nil {
				value = stringValue(contextRecord["id"])
			}
			parts = append(parts, contextField+" = "+pbFilterString(value))
		}
		if uiDetailSource(page) == sourceID && recordID != "" {
			parts = append(parts, "id = "+pbFilterString(recordID))
		} else if uiDetailSource(page) == sourceID {
			parts = append(parts, "id = \"__missing_record__\"")
		}
		alts := []string{}
		for _, field := range selected {
			if contains([]any{"text", "email", "url"}, field["type"]) {
				alts = append(alts, stringValue(field["name"])+" ~ "+pbFilterString(search))
			}
		}
		if search != "" && len(alts) > 0 && sourceID != uiDetailSource(page) {
			parts = append(parts, "("+strings.Join(alts, " || ")+")")
		}
		sourcePage := pageNum
		if sourceID == uiDetailSource(page) {
			sourcePage = 1
		}
		rows, total, totalPages, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(parts...), uiQuerySort(rawSource), sourcePage, per)
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
					options = appendContextOption(options, contextRecord)
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
		detailSource := uiDetailSource(candidate)
		if detailSource != "" {
			for _, source := range dataSources {
				if source["id"] == detailSource {
					collection = stringValue(source["collection"])
				}
			}
		}
		pageList = append(pageList, map[string]any{"id": candidate["id"], "title": candidate["title"], "collection": collection, "detail_source": detailSource})
	}
	result := map[string]any{"status": "published", "version": publicVersion(version, stringValue(version["id"])), "title": page["title"], "app_title": definition["title"], "ui_page": page["id"], "record_id": recordID, "pages": pageList, "definition": definition, "sources": sources, "members": members, "read_only": false}
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
	if _, msg := validateAppUIDefinition(version["definition"], tables); msg != "" {
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
	runtime["previous_version_id"] = nilIfEmpty(previousID)
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
	changes := diffAppUI(asMap(current["definition"]), asMap(version["definition"]))
	writeJSON(w, 200, map[string]any{"current_version_id": nilIfEmpty(currentID), "target_version_id": version["id"], "changes": changes, "data_changed": false})
}
func diffAppUI(before, after map[string]any) []map[string]any {
	changes := []map[string]any{}
	add := func(kind, page, resource string, old, next any) {
		changes = append(changes, map[string]any{"type": kind, "page": page, "resource": resource, "before": old, "after": next})
	}
	if !equalJSON(before["title"], after["title"]) {
		add("title", "", "", before["title"], after["title"])
	}
	oldPages, newPages := appUIPages(before), appUIPages(after)
	oldIDs, newIDs := []string{}, []string{}
	oldByID, newByID := map[string]map[string]any{}, map[string]map[string]any{}
	for _, page := range oldPages {
		id := stringValue(page["id"])
		oldIDs = append(oldIDs, id)
		oldByID[id] = page
	}
	for _, page := range newPages {
		id := stringValue(page["id"])
		newIDs = append(newIDs, id)
		newByID[id] = page
	}
	for _, page := range oldPages {
		id := stringValue(page["id"])
		if newByID[id] == nil {
			add("remove_page", id, "", page, nil)
		}
	}
	if !equalJSON(oldIDs, newIDs) {
		add("page_order", "", "", oldIDs, newIDs)
	}
	for _, page := range newPages {
		id := stringValue(page["id"])
		old := oldByID[id]
		if old == nil {
			add("add_page", id, "", nil, page)
			continue
		}
		if !equalJSON(old["title"], page["title"]) {
			add("page_title", id, "", old["title"], page["title"])
		}
		oldSources, newSources := map[string]map[string]any{}, map[string]map[string]any{}
		for _, source := range asSliceMap(old["data_sources"]) {
			oldSources[stringValue(source["id"])] = source
		}
		for _, source := range asSliceMap(page["data_sources"]) {
			newSources[stringValue(source["id"])] = source
		}
		for _, source := range asSliceMap(old["data_sources"]) {
			key := stringValue(source["id"])
			if newSources[key] == nil {
				add("remove_source", id, key, source, nil)
			}
		}
		for _, source := range asSliceMap(page["data_sources"]) {
			key := stringValue(source["id"])
			previous := oldSources[key]
			if previous == nil {
				add("add_source", id, key, nil, source)
				continue
			}
			for _, property := range []string{"collection", "fields", "form_fields", "actions", "query", "context"} {
				if !equalJSON(previous[property], source[property]) {
					add("source_"+property, id, key, previous[property], source[property])
				}
			}
		}
		oldSpec, nextSpec := asMap(old["spec"]), asMap(page["spec"])
		if oldSpec["root"] != nextSpec["root"] {
			add("root", id, "", oldSpec["root"], nextSpec["root"])
		}
		oldElements, newElements := asMap(oldSpec["elements"]), asMap(nextSpec["elements"])
		// Traverse both trees for deterministic human-readable differences.
		var walk func(map[string]any, string, func(string))
		walk = func(elements map[string]any, key string, visit func(string)) {
			visited := map[string]bool{}
			var traverse func(string)
			traverse = func(key string) {
				element := asMap(elements[key])
				if element == nil || visited[key] {
					return
				}
				visited[key] = true
				visit(key)
				for _, child := range stringSlice(anySlice(element["children"])) {
					traverse(child)
				}
			}
			traverse(key)
		}
		walk(oldElements, stringValue(oldSpec["root"]), func(key string) {
			if newElements[key] == nil {
				add("remove_component", id, key, oldElements[key], nil)
			}
		})
		walk(newElements, stringValue(nextSpec["root"]), func(key string) {
			old, next := asMap(oldElements[key]), asMap(newElements[key])
			if old == nil {
				add("add_component", id, key, nil, next)
				return
			}
			if old["type"] != next["type"] {
				add("component_type", id, key, old["type"], next["type"])
			}
			if !equalJSON(old["props"], next["props"]) {
				add("component_props", id, key, old["props"], next["props"])
			}
			if !equalJSON(old["children"], next["children"]) {
				add("component_order", id, key, old["children"], next["children"])
			}
		})
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
	preview["current_version_id"] = nilIfEmpty(currentID)
	preview["page_previews"], preview["status"], preview["version"], preview["changes"], preview["note"] = previews, "preview", publicVersion(version, currentID), diffAppUI(asMap(currentVersion["definition"]), definition), "仅界面定义变更，业务记录不回滚"
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
	definition, msg := validateAppUIDefinition(input["definition"], tables)
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
	runtime["status"], runtime["read_only"], runtime["changes"] = "preview", true, diffAppUI(asMap(published["definition"]), definition)
	runtime["current_version_id"] = nilIfEmpty(stringValue(app["published_version_id"]))
	writeJSON(w, 200, runtime)
}

func (s *Server) createUIDraft(ctx context.Context, actor executionActor, input map[string]any, stepID string) (map[string]any, error) {
	var created map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		id, app, _, err := s.buildActor(ctx, tx, actor)
		if err != nil {
			return err
		}
		// A durable receipt wins over a changed baseline when recovering a write.
		if stepID != "" {
			prior, err := tx.Find(ctx, "app_versions", "harness_step_id = "+pbFilterString(stepID))
			if err == nil {
				if prior["tenant_id"] != actor.TenantID || prior["app_id"] != actor.AppID || prior["created_by"] != actor.UserID {
					return businessError(403, "草稿回执不属于当前运行")
				}
				created = prior
				return nil
			}
			if !isMissing(err) {
				return err
			}
		}
		tables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID)), "created")
		if err != nil {
			return err
		}
		definition, msg := validateAppUIDefinition(input["definition"], tables)
		if msg != "" {
			return businessError(400, msg)
		}
		if msg = validateUIActionReferences(ctx, tx, app, definition, false); msg != "" {
			return businessError(400, msg)
		}
		latest, _, _, err := tx.List(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID)), "-version", 1, 1)
		if err != nil {
			return err
		}
		latestID, number := "", 1
		if len(latest) > 0 {
			latestID = stringValue(latest[0]["id"])
			number = intValue(latest[0]["version"]) + 1
		}
		if expected, exists := input["expected_latest_version_id"]; exists {
			if expected != nil {
				if _, valid := expected.(string); !valid {
					return businessError(400, "版本基线必须为字符串或 null")
				}
			}
			if stringValue(expected) != latestID {
				return businessError(409, "已有新界面草稿，请重新载入后审阅；本次编辑和原草稿均未覆盖")
			}
		}
		basedID := stringValue(input["based_on_version_id"])
		if basedID != "" {
			base, err := tx.Get(ctx, "app_versions", basedID)
			if err != nil || base["tenant_id"] != actor.TenantID || base["app_id"] != actor.AppID {
				return businessError(404, "修订来源版本不存在")
			}
			published := stringValue(base["published_at"]) != ""
			current := basedID == stringValue(app["published_version_id"])
			if published && !current {
				return businessError(409, "正式版本已变化，请重新载入；原草稿已保留")
			}
		}
		if expected, exists := input["expected_published_version_id"]; exists && stringValue(expected) != stringValue(app["published_version_id"]) {
			return businessError(409, "正式界面已变化，请重新载入后审阅")
		}
		source := cloneAnyMap(asMap(input["source"]))
		if source == nil {
			source = map[string]any{}
		}
		source["submitted_by"] = map[string]any{"id": id.User["id"], "name": id.User["name"], "email": id.User["email"]}
		created, err = tx.Create(ctx, "app_versions", map[string]any{"tenant_id": actor.TenantID, "app_id": actor.AppID, "version": number, "summary": clip(strings.TrimSpace(stringValue(input["summary"])), 1000), "based_on_version_id": basedID, "created_by": id.User["id"], "definition": definition, "source": source, "harness_step_id": stepID})
		return err
	})
	return created, err
}

// publishHarnessDraft publishes the reviewed draft referenced by input and
// optionally applies the anonymous publication profile in one transaction.
// The dialogue chain's ui.publish candidates are the only caller since the
// manual publish entry was removed with the visual editor.
func (s *Server) publishHarnessDraft(ctx context.Context, actor executionActor, input map[string]any) (map[string]any, error) {
	versionID := stringValue(input["version_id"])
	if versionID == "" || actor.AppID == "" || actor.TenantID == "" {
		return nil, harness.ErrCapability
	}
	expected := ""
	if raw, exists := input["expected_published_version_id"]; exists && raw != nil {
		text, ok := raw.(string)
		if !ok {
			return nil, businessError(400, "发布基线必须为字符串或 null")
		}
		expected = text
	}
	unlock := lockAppVersion(actor.AppID)
	defer unlock()
	app, err := s.PB.Get(ctx, "apps", actor.AppID)
	if err != nil || app["tenant_id"] != actor.TenantID {
		return nil, businessError(404, "应用不存在或你没有访问权限")
	}
	if boolValue(app["archived"]) {
		return nil, businessError(409, "应用已归档，无法发布")
	}
	currentID := stringValue(app["published_version_id"])
	if expected != currentID {
		return nil, businessError(409, "正式界面已变化，请刷新后重新发布")
	}
	version, err := s.PB.Get(ctx, "app_versions", versionID)
	if err != nil || version["tenant_id"] != actor.TenantID || version["app_id"] != actor.AppID {
		return nil, businessError(404, "草稿版本不存在")
	}
	if currentID == versionID {
		runtime, err := s.runtimeForVersion(ctx, app, actor.TenantID, version, map[string]string{}, 0)
		if err != nil {
			return nil, err
		}
		runtime["version"] = publicVersion(version, versionID)
		return runtime, nil
	}
	if stringValue(version["published_at"]) != "" {
		return nil, businessError(409, "此版本已经发布或已被替代；如需回退，请基于当前版本创建新草稿")
	}
	if currentID != "" {
		current, e := s.PB.Get(ctx, "app_versions", currentID)
		if e != nil {
			return nil, e
		}
		if intValue(version["version"]) <= intValue(current["version"]) {
			return nil, businessError(409, "不能发布早于或等于当前版本的草稿；如需回退，请基于当前版本创建新草稿")
		}
	}
	tables, err := s.appTables(ctx, app, actor.TenantID)
	if err != nil {
		return nil, err
	}
	if _, msg := validateAppUIDefinition(version["definition"], tables); msg != "" {
		return nil, businessError(400, "草稿无法发布："+msg)
	}
	if msg := s.validateBusinessActionReferences(ctx, app, asMap(version["definition"]), true); msg != "" {
		return nil, businessError(400, "草稿无法发布："+msg)
	}
	previewApp := map[string]any{}
	for k, v := range app {
		previewApp[k] = v
	}
	previewApp["published_version_id"] = versionID
	runtime, err := s.runtimeForVersion(ctx, previewApp, actor.TenantID, version, map[string]string{}, 0)
	if err != nil || runtime["status"] != "published" {
		return nil, businessError(409, "草稿运行检查失败，当前发布版未更改")
	}
	publication := asMap(input["publication"])
	profile := map[string]any{}
	if publication["enabled"] == true {
		slug := strings.TrimSpace(strings.ToLower(stringValue(publication["slug"])))
		if len(slug) < 3 || len(slug) > 64 || !publicationSlugPattern.MatchString(slug) {
			return nil, businessError(400, "公开链接标识需为 3–64 位小写字母、数字或连字符")
		}
		pages, message := normalizePublicPages(version, publication["pages"], tables)
		if message != "" {
			return nil, businessError(400, message)
		}
		if existing, findErr := s.PB.Find(ctx, "apps", "public_slug = "+pbFilterString(slug)); findErr == nil && existing["id"] != actor.AppID {
			return nil, businessError(409, "公开链接标识已被使用")
		}
		profile = map[string]any{"enabled": true, "slug": slug, "pages": pages}
	}
	err = s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		fresh, err := tx.Get(ctx, "apps", actor.AppID)
		if err != nil {
			return err
		}
		if stringValue(fresh["published_version_id"]) != currentID {
			return businessError(409, "正式界面已变化，请刷新后发布")
		}
		if _, err := s.authorizeWrite(ctx, tx, actor, false); err != nil {
			return err
		}
		tenant, err := tx.Get(ctx, "tenants", actor.TenantID)
		if err != nil {
			return err
		}
		user, err := tx.Get(ctx, "users", actor.UserID)
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
		txTables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID)), "created")
		if err != nil {
			return err
		}
		if _, msg := validateAppUIDefinition(version["definition"], txTables); msg != "" {
			return businessError(409, msg)
		}
		if msg := validateUIActionReferences(ctx, tx, fresh, asMap(version["definition"]), true); msg != "" {
			return businessError(409, msg)
		}
		if _, err := tx.Update(ctx, "app_versions", versionID, map[string]any{"published_at": nowISO()}); err != nil {
			return err
		}
		updates := map[string]any{"published_version_id": versionID}
		if publication["enabled"] == true {
			updates["public_slug"] = stringValue(profile["slug"])
			updates["public_publication"] = profile
		}
		_, err = tx.Update(ctx, "apps", actor.AppID, updates)
		return err
	})
	if err != nil {
		return nil, err
	}
	runtime["version"] = publicVersion(version, versionID)
	receipt := map[string]any{"status": "published", "app_id": actor.AppID, "version": versionID, "version_number": intValue(version["version"]), "published": true, "publication": "private", "runtime": runtime, "message": fmt.Sprintf("正式界面已发布 v%d。", intValue(version["version"]))}
	if publication["enabled"] == true {
		receipt["publication"] = "public"
		receipt["public_slug"] = stringValue(profile["slug"])
		receipt["message"] = fmt.Sprintf("正式界面已发布 v%d，匿名公开地址 /s/%s。", intValue(version["version"]), stringValue(profile["slug"]))
	}
	return receipt, nil
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
