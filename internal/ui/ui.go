// Package ui is the pure UI-definition module: it validates, renders, diffs
// and merges application interface definitions over map[string]any values,
// with no HTTP, PocketBase or Server dependency. Callers and tests cross the
// same small seam; the httpapi handlers are thin adapters behind it.
package ui

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tans/miao/internal/pocketbase"
)

// Translator localizes a UI template for one language. httpapi passes its
// catalog-backed TLang; tests pass an identity function.
type Translator func(lang, template string, params ...map[string]string) string

type BuildDefinition struct {
	SchemaVersion     int             `json:"schema_version"`
	Name              string          `json:"name"`
	Description       string          `json:"description,omitempty"`
	Tables            []BuildTable    `json:"tables"`
	Connectors        []BuildResource `json:"connectors,omitempty"`
	Actions           []BuildResource `json:"actions,omitempty"`
	Workflows         []BuildResource `json:"workflows,omitempty"`
	CollectionScripts []BuildResource `json:"collection_scripts,omitempty"`
}

type BuildResource struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Definition  map[string]any `json:"definition"`
}

type BuildTable struct {
	Name   string           `json:"name"`
	Slug   string           `json:"slug"`
	Fields []map[string]any `json:"fields"`
}

func businessError(status int, message string) error {
	return &pocketbase.Error{Status: status, Message: message}
}

// ValidateDefinition checks a raw interface definition against the app's data
// tables and returns the normalized definition, or an empty definition with a
// user-readable reason.
func ValidateDefinition(raw any, tables []map[string]any) (map[string]any, string) {
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
		if !ValidID(pid, 40) || ids[pid] {
			return nil, "页面标识无效或重复"
		}
		ids[pid] = true
		ptitle := strings.TrimSpace(stringValue(page["title"]))
		if ptitle == "" || len([]rune(ptitle)) > 120 {
			return nil, "页面标题无效"
		}
		sources, msg := ValidateSources(page["data_sources"], tables)
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
		spec, msg := ValidateSpec(page["spec"], sources)
		if msg != "" {
			return nil, msg
		}
		if msg := ValidateRecordContexts(spec, sources, tables); msg != "" {
			return nil, msg
		}
		pages = append(pages, map[string]any{"id": pid, "title": ptitle, "spec": spec, "data_sources": sources})
	}
	return map[string]any{"schema_version": 3, "title": title, "pages": pages}, ""
}

// ValidateSources normalizes a page's data-source list against the tables.
func ValidateSources(raw any, tables []map[string]any) ([]map[string]any, string) {
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
		if !ValidID(id, 40) || seen[id] {
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
		if !IsUIArray(item["fields"]) || (item["actions"] != nil && !IsUIArray(item["actions"])) {
			return nil, "字段和动作必须为数组"
		}
		actions := []map[string]any{}
		actionIDs := map[string]bool{}
		if len(anySlice(item["actions"])) > 24 {
			return nil, "每个数据源最多 24 个动作"
		}
		for _, rawAction := range anySlice(item["actions"]) {
			action := asMap(rawAction)
			if action == nil || len(action) < 2 || len(action) > 7 || !ValidID(stringValue(action["id"]), 40) || strings.TrimSpace(stringValue(action["label"])) == "" {
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
			if len(binding) != 2 || !ValidID(stringValue(binding["source"]), 40) || !ValidID(stringValue(binding["field"]), 60) {
				return nil, "关联上下文需要真实详情数据源与关联字段"
			}
			source["context"] = map[string]any{"source": binding["source"], "field": binding["field"]}
		}
		if raw, exists := item["form_fields"]; exists {
			names := []string{}
			selected := map[string]bool{}
			if !IsUIArray(raw) {
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
			query, msg := ValidateQuery(raw, asSliceMap(table["fields"]))
			if msg != "" {
				return nil, msg
			}
			source["query"] = query
		}
		sources = append(sources, source)
	}
	return sources, ""
}

func IsUIArray(raw any) bool {
	switch raw.(type) {
	case []any, []string, []map[string]any:
		return true
	}
	return false
}

// ValidateQuery normalizes a data-source query (filters + sort) against the
// table's fields.
func ValidateQuery(raw any, fields []map[string]any) (map[string]any, string) {
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
	if raw, exists := query["filters"]; exists && !IsUIArray(raw) {
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

// QueryFilter renders a validated source query into PocketBase filter parts.
// Only validated field names/operators reach PocketBase; values are quoted or
// marshalled as scalars rather than accepting an arbitrary filter expression.
func QueryFilter(source map[string]any) []string {
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

// QuerySort returns the validated sort for a source, defaulting to "-created".
func QuerySort(source map[string]any) string {
	return defaultString(stringValue(asMap(source["query"])["sort"]), "-created")
}

// ValidateSpec checks the controlled component tree of one page.
func ValidateSpec(raw any, sources []map[string]any) (map[string]any, string) {
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
		if !ValidID(id, 64) {
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
			if key == "source" && (typ == "RecordDetail" || typ == "RecordTable" || typ == "RecordCards" || typ == "RecordForm") && !ValidID(stringValue(value), 40) {
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
		if len(children) > 32 || (element["children"] != nil && !IsUIArray(element["children"])) {
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

// ValidateRecordContexts enforces that a relation context only references the
// page's own primary detail, with no cycles or cross-page references.
func ValidateRecordContexts(spec map[string]any, sources, tables []map[string]any) string {
	primaryID := DetailSource(map[string]any{"spec": spec})
	var primary map[string]any
	for _, source := range sources {
		if source["id"] == primaryID {
			primary = source
		}
	}
	for _, source := range sources {
		if source["context"] == nil {
			continue
		}
		binding := asMap(source["context"])
		if primary == nil || source["id"] == primaryID || binding["source"] != primaryID || primary["context"] != nil {
			return "关联上下文只能引用当前页的主记录详情，不支持循环或跨页引用"
		}
		var table map[string]any
		for _, candidate := range tables {
			if candidate["slug"] == source["collection"] {
				table = candidate
			}
		}
		field := findField(asSliceMap(table["fields"]), stringValue(binding["field"]))
		if field == nil || field["type"] != "relation" || field["target"] != primary["collection"] {
			return "关联上下文字段必须引用主记录详情的数据表"
		}
	}
	return ""
}

func ValidID(value string, max int) bool {
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

func Pages(definition map[string]any) []map[string]any { return asSliceMap(definition["pages"]) }

// DetailSource walks the page spec and returns the id of the RecordDetail
// source that anchors the page, or "".
func DetailSource(page map[string]any) string {
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

func Status(version map[string]any, publishedID string) string {
	if stringValue(version["id"]) == publishedID {
		return "published"
	}
	if stringValue(version["published_at"]) != "" {
		return "superseded"
	}
	return "draft"
}

func Public(version map[string]any, publishedID string) map[string]any {
	source := asMap(version["source"])
	submittedBy := asMap(source["submitted_by"])
	return map[string]any{
		"id": version["id"], "version": version["version"],
		"summary":             defaultString(stringValue(version["summary"]), ""),
		"status":              Status(version, publishedID),
		"based_on_version_id": defaultString(stringValue(version["based_on_version_id"]), ""),
		"created_at":          version["created"], "updated_at": version["updated"], "published_at": version["published_at"],
		"created_by": version["created_by"], "created_by_name": defaultString(stringValue(submittedBy["name"]), stringValue(submittedBy["email"])),
		"created_by_email": submittedBy["email"], "change_request": source["request"],
	}
}

func NilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// AppendContextOption adds the context record to the relation options once.
func AppendContextOption(options []map[string]any, contextRecord map[string]any) []map[string]any {
	if contextRecord == nil {
		return options
	}
	for _, option := range options {
		if option["id"] == contextRecord["id"] {
			return options
		}
	}
	return append(options, contextRecord)
}

// AddRelatedDetailSources decorates a generated detail page with RecordCards
// sources for each related table of the primary table.
func AddRelatedDetailSources(page, primaryTable map[string]any, tables []map[string]any) {
	elements := asMap(asMap(page["spec"])["elements"])
	section := asMap(elements["section"])
	for _, table := range tables {
		if table["slug"] == primaryTable["slug"] || len(anySlice(page["data_sources"])) >= 12 {
			continue
		}
		for _, field := range asSliceMap(table["fields"]) {
			if field["type"] != "relation" || field["target"] != primaryTable["slug"] {
				continue
			}
			id := backendOpaqueID("related_", stringValue(table["slug"]), stringValue(field["name"]))
			fields := []any{}
			for _, field := range asSliceMap(table["fields"]) {
				fields = append(fields, field["name"])
			}
			source := map[string]any{"id": id, "collection": table["slug"], "fields": fields, "actions": []any{}, "context": map[string]any{"source": "records", "field": field["name"]}}
			page["data_sources"] = append(anySlice(page["data_sources"]), source)
			elements[id] = map[string]any{"type": "RecordCards", "props": map[string]any{"source": id, "title": table["name"]}, "children": []any{}}
			section["children"] = append(anySlice(section["children"]), id)
			break
		}
	}
}

// FieldMatches reports whether a stored field satisfies the requested one.
func FieldMatches(stored, requested map[string]any) bool {
	if stored == nil {
		return false
	}
	for _, key := range []string{"name", "label", "type", "required"} {
		if stored[key] != requested[key] {
			return false
		}
	}
	if requested["type"] == "relation" && stored["target"] != requested["target"] {
		return false
	}
	return requested["type"] != "select" || equalJSON(stored["options"], requested["options"])
}

// NextOperation computes the next backend build step for a declaration: the
// first missing table to create, the first drifted table to update, or the
// complete flag when every declared table already matches.
func NextOperation(definition BuildDefinition, tables []map[string]any, tenantID, appID string) (string, map[string]any, bool, error) {
	bySlug := map[string]map[string]any{}
	for _, table := range tables {
		bySlug[stringValue(table["slug"])] = table
	}
	// Create available tables first; dependent relation fields are added after
	// their targets exist, including mutual references between new tables.
	for _, requested := range definition.Tables {
		if bySlug[requested.Slug] != nil {
			continue
		}
		fields := []map[string]any{}
		for _, field := range requested.Fields {
			if field["type"] != "relation" || bySlug[stringValue(field["target"])] != nil {
				fields = append(fields, cloneAnyMap(field))
			}
		}
		if len(fields) > 0 {
			return "collections.create", map[string]any{"name": requested.Name, "slug": requested.Slug, "fields": mapSliceAny(fields)}, false, nil
		}
	}
	for _, requested := range definition.Tables {
		table := bySlug[requested.Slug]
		if table == nil {
			return "", nil, false, businessError(400, "关联依赖无法解析；新表需要一个可先创建的字段")
		}
		fields := asSliceMap(table["fields"])
		changed := stringValue(table["name"]) != requested.Name
		for _, desired := range requested.Fields {
			current := findField(fields, stringValue(desired["name"]))
			if FieldMatches(current, desired) {
				continue
			}
			if desired["type"] == "relation" && bySlug[stringValue(desired["target"])] == nil {
				return "", nil, false, businessError(400, "关联目标不在当前应用或声明中")
			}
			replaced := false
			for index, field := range fields {
				if field["name"] == desired["name"] {
					fields[index], replaced = cloneAnyMap(desired), true
					break
				}
			}
			if !replaced {
				fields = append(fields, cloneAnyMap(desired))
			}
			changed = true
		}
		if changed {
			return "collections.update", map[string]any{"table_id": table["id"], "name": requested.Name, "slug": table["slug"], "fields": mapSliceAny(fields)}, false, nil
		}
	}
	return "", nil, true, nil
}

// MergeDefinition rebuilds a UI definition draft on top of the current tables:
// existing pages and their bindings are preserved, fields added since the run's
// snapshot are appended, and capacity is capped at 12 pages.
func MergeDefinition(name, lang string, tables []map[string]any, base map[string]any, previousTables []map[string]any, translate Translator) (map[string]any, error) {
	pages := []any{}
	knownCollections, usedIDs := map[string]bool{}, map[string]bool{}
	bySlug, previous := map[string]map[string]any{}, map[string]map[string]any{}
	for _, table := range tables {
		bySlug[stringValue(table["slug"])] = table
	}
	for _, table := range previousTables {
		previous[stringValue(table["slug"])] = table
	}
	for _, original := range Pages(base) {
		data, err := json.Marshal(original)
		if err != nil {
			return nil, err
		}
		page := map[string]any{}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, err
		}
		usedIDs[stringValue(page["id"])] = true
		for _, source := range asSliceMap(page["data_sources"]) {
			collection := stringValue(source["collection"])
			table := bySlug[collection]
			if table == nil {
				return nil, businessError(409, "现有界面的数据表已失效；原草稿已保留")
			}
			knownCollections[collection] = true
			fields := anySlice(source["fields"])
			selected := stringSet(source["fields"])
			if old := previous[collection]; old != nil {
				for _, field := range asSliceMap(table["fields"]) {
					key := stringValue(field["name"])
					if findField(asSliceMap(old["fields"]), key) == nil {
						if !selected[key] {
							fields = append(fields, key)
							selected[key] = true
						}
						if names, exists := source["form_fields"]; exists && !stringSet(names)[key] {
							source["form_fields"] = append(anySlice(names), key)
						}
					}
				}
				// Rename generated labels only when they still equal the original label.
				oldName, nextName := stringValue(old["name"]), stringValue(table["name"])
				if oldName != nextName {
					renamed := map[string]string{oldName: nextName,
						translate(lang, "{title}详情", map[string]string{"title": oldName}): translate(lang, "{title}详情", map[string]string{"title": nextName}),
						translate(lang, "新增{title}", map[string]string{"title": oldName}): translate(lang, "新增{title}", map[string]string{"title": nextName})}
					if replacement := renamed[stringValue(page["title"])]; replacement != "" {
						page["title"] = replacement
					}
					for _, raw := range asMap(asMap(page["spec"])["elements"]) {
						props := asMap(asMap(raw)["props"])
						if replacement := renamed[stringValue(props["title"])]; replacement != "" {
							props["title"] = replacement
						}
					}
				}
			}
			source["fields"] = fields
		}
		pages = append(pages, page)
	}
	missing := []map[string]any{}
	for _, table := range tables {
		if !knownCollections[stringValue(table["slug"])] {
			missing = append(missing, table)
		}
	}
	if len(pages)+len(missing) > 12 {
		return nil, businessError(409, "页面容量不足（最多 12 页）；请先调整现有页面，原草稿已保留")
	}
	// Reserve one list per table, then add detail pages while capacity permits.
	detailSlots := 12 - len(pages) - len(missing)
	nextID := func(slug, kind string) string {
		id := slug
		if kind == "detail" {
			id = slug + "_detail"
		}
		if !ValidID(id, 40) || usedIDs[id] {
			id = backendOpaqueID("page_", slug, kind)
		}
		usedIDs[id] = true
		return id
	}
	for _, table := range missing {
		slug, title := stringValue(table["slug"]), stringValue(table["name"])
		fields := []any{}
		for _, field := range asSliceMap(table["fields"]) {
			fields = append(fields, stringValue(field["name"]))
		}
		actions := []any{}
		for _, field := range asSliceMap(table["fields"]) {
			if field["type"] != "select" {
				continue
			}
			for index, option := range stringSlice(anySlice(field["options"])) {
				if len(actions) == 24 {
					break
				}
				actions = append(actions, map[string]any{"id": backendOpaqueID("set_", slug, stringValue(field["name"]), fmt.Sprint(index)), "label": translate(lang, "设为{value}", map[string]string{"value": option}), "set": map[string]any{stringValue(field["name"]): option}})
			}
		}
		source := map[string]any{"id": "records", "collection": slug, "fields": fields, "actions": actions}
		makePage := func(id, pageTitle, typ string) map[string]any {
			elements := map[string]any{
				"page":    map[string]any{"type": "Page", "props": map[string]any{"title": pageTitle}, "children": []any{"section"}},
				"section": map[string]any{"type": "Section", "props": map[string]any{"title": title}, "children": []any{"records"}},
				"records": map[string]any{"type": typ, "props": map[string]any{"source": "records", "title": pageTitle}, "children": []any{}},
			}
			if typ != "RecordDetail" {
				asMap(elements["section"])["children"] = []any{"records", "form"}
				elements["form"] = map[string]any{"type": "RecordForm", "props": map[string]any{"source": "records", "title": translate(lang, "新增{title}", map[string]string{"title": title})}, "children": []any{}}
			}
			return map[string]any{"id": id, "title": pageTitle, "data_sources": []any{cloneAnyMap(source)}, "spec": map[string]any{"root": "page", "elements": elements}}
		}
		pages = append(pages, makePage(nextID(slug, "list"), title, "RecordCards"))
		if detailSlots > 0 {
			detail := makePage(nextID(slug, "detail"), translate(lang, "{title}详情", map[string]string{"title": title}), "RecordDetail")
			AddRelatedDetailSources(detail, table, tables)
			pages = append(pages, detail)
			detailSlots--
		}
	}
	if base != nil {
		name = stringValue(base["title"])
	}
	definition, message := ValidateDefinition(map[string]any{"schema_version": 3, "title": defaultString(name, translate(lang, "应用")), "pages": pages}, tables)
	if message != "" {
		return nil, businessError(409, message+"；原草稿已保留")
	}
	return definition, nil
}

// Diff renders the human-readable change set between two UI definitions.
func Diff(before, after map[string]any) []map[string]any {
	changes := []map[string]any{}
	add := func(kind, page, resource string, old, next any) {
		changes = append(changes, map[string]any{"type": kind, "page": page, "resource": resource, "before": old, "after": next})
	}
	if !equalJSON(before["title"], after["title"]) {
		add("title", "", "", before["title"], after["title"])
	}
	oldPages, newPages := Pages(before), Pages(after)
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

func asMap(raw any) map[string]any {
	if value, ok := raw.(map[string]any); ok && value != nil {
		return value
	}
	return map[string]any{}
}

func anySlice(raw any) []any {
	switch value := raw.(type) {
	case []any:
		return value
	case []map[string]any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = item
		}
		return out
	case []string:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = item
		}
		return out
	}
	return []any{}
}

func asSliceMap(raw any) []map[string]any {
	out := []map[string]any{}
	for _, v := range anySlice(raw) {
		if item, ok := v.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out
}

func mapSliceAny(values []map[string]any) []any {
	out := make([]any, len(values))
	for i, v := range values {
		out[i] = v
	}
	return out
}

func stringSlice(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, stringValue(value))
	}
	return result
}

func stringValue(raw any) string { value, _ := raw.(string); return value }

func intValue(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	}
	return 0
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func findField(fields []map[string]any, name string) map[string]any {
	for _, field := range fields {
		if field["name"] == name {
			return field
		}
	}
	return nil
}

func cloneAnyMap(value map[string]any) map[string]any {
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func equalJSON(a, b any) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(aa, bb)
}

func pbFilterString(value string) string { encoded, _ := json.Marshal(value); return string(encoded) }

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func stringSet(raw any) map[string]bool {
	values := map[string]bool{}
	for _, item := range anySlice(raw) {
		if value, ok := item.(string); ok && value != "" {
			values[value] = true
		}
	}
	return values
}

func backendOpaqueID(prefix, scope string, parts ...string) string {
	value := scope
	for _, part := range parts {
		value += "\x00" + part
	}
	sum := sha256.Sum256([]byte(value))
	return prefix + hex.EncodeToString(sum[:12])
}
