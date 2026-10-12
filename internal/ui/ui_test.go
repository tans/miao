package ui

import "testing"

// validDefinition builds a minimal definition that passes every rule:
// one page with a data source bound to a real table, and a RecordCards
// component that uses the source so the "all sources must be used" check holds.
func validDefinition(pages int) map[string]any {
	tables := []map[string]any{{"slug": "users", "fields": []map[string]any{{"name": "name", "type": "text"}}}}
	list := make([]any, 0, pages)
	for i := 0; i < pages; i++ {
		list = append(list, map[string]any{
			"id":    "page_" + string(rune('a'+i)),
			"title": "页面",
			"spec": map[string]any{
				"root": "page",
				"elements": map[string]any{
					"page":  map[string]any{"type": "Page", "props": map[string]any{"title": "页面"}, "children": []any{"cards"}},
					"cards": map[string]any{"type": "RecordCards", "props": map[string]any{"source": "src_a", "title": "卡片"}, "children": []any{}},
				},
			},
			"data_sources": []any{
				map[string]any{"id": "src_a", "collection": "users", "fields": []any{"name"}},
			},
		})
	}
	_ = tables
	return map[string]any{"schema_version": 3, "title": "测试应用", "pages": list}
}

func TestValidateDefinitionAcceptsValid(t *testing.T) {
	tables := []map[string]any{{"slug": "users", "fields": []map[string]any{{"name": "name", "type": "text"}}}}
	if _, msg := ValidateDefinition(validDefinition(1), tables); msg != "" {
		t.Fatalf("合法定义应通过，得到：%s", msg)
	}
}

func TestValidateDefinitionRejectsTooManyPages(t *testing.T) {
	tables := []map[string]any{{"slug": "users", "fields": []map[string]any{{"name": "name", "type": "text"}}}}
	if _, msg := ValidateDefinition(validDefinition(13), tables); msg == "" {
		t.Fatal("13 页应被拒绝")
	}
}

func TestValidateDefinitionRejectsUnknownComponent(t *testing.T) {
	tables := []map[string]any{{"slug": "users", "fields": []map[string]any{{"name": "name", "type": "text"}}}}
	def := validDefinition(1)
	page := def["pages"].([]any)[0].(map[string]any)
	spec := page["spec"].(map[string]any)
	spec["elements"].(map[string]any)["cards"].(map[string]any)["type"] = "Unknown"
	if _, msg := ValidateDefinition(def, tables); msg == "" {
		t.Fatal("未知组件类型应被拒绝")
	}
}

func TestDiffReportsTitleChange(t *testing.T) {
	tables := []map[string]any{{"slug": "users", "fields": []map[string]any{{"name": "name", "type": "text"}}}}
	before := validDefinition(1)
	after := validDefinition(1)
	after["title"] = "新标题"
	ValidateDefinition(before, tables)
	ValidateDefinition(after, tables)
	changes := Diff(before, after)
	found := false
	for _, c := range changes {
		if c["type"] == "title" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Diff 应报告标题变更，得到：%v", changes)
	}
}

func TestFieldMatches(t *testing.T) {
	stored := map[string]any{"name": "f", "type": "text"}
	if !FieldMatches(stored, map[string]any{"name": "f", "type": "text"}) {
		t.Fatal("完全一致的字段应匹配")
	}
	if FieldMatches(stored, map[string]any{"name": "f", "type": "number"}) {
		t.Fatal("类型不同的字段不应匹配")
	}
}
