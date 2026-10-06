package httpapi

import (
	"embed"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

//go:embed build_templates/*.json
var buildTemplates embed.FS

func applicationTemplates() []map[string]any {
	items := []map[string]any{}
	for _, key := range []string{"crm", "cms", "collection"} {
		data, err := buildTemplates.ReadFile("build_templates/" + key + ".json")
		if err != nil {
			continue
		}
		var item map[string]any
		if json.Unmarshal(data, &item) == nil {
			items = append(items, item)
		}
	}
	return items
}

func (s *Server) listBuildTemplates(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"items": applicationTemplates()})
}

func declarationForRequest(prompt string, input map[string]any) (any, bool, error) {
	if !strings.HasPrefix(strings.TrimSpace(prompt), "{") {
		return templateForRequest(prompt, input)
	}
	if len(prompt) > 200000 {
		return nil, true, businessError(400, "应用声明过大")
	}
	if stringValue(input["template"]) != "" {
		return nil, true, businessError(400, "声明和模板不能混用，请明确选择一种搭建方式")
	}
	var raw any
	decoder := json.NewDecoder(strings.NewReader(prompt))
	if err := decoder.Decode(&raw); err != nil {
		return nil, true, businessError(400, "应用声明 JSON 无效")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, true, businessError(400, "应用声明只能包含一个 JSON 对象")
	}
	if object := asMap(raw); object["definition"] != nil {
		if len(object) != 1 {
			return nil, true, businessError(400, "声明包装只允许 definition；运行权限不能来自声明")
		}
		raw = object["definition"]
	}
	definition, err := parseBuildDefinition(raw)
	return definition, true, err
}

// Only an explicit template command is deterministic. A mention inside an
// open-ended request is not permission to silently substitute a fixed template.
func templateForRequest(prompt string, input map[string]any) (any, bool, error) {
	key, name := stringValue(input["template"]), strings.TrimSpace(stringValue(input["name"]))
	if key == "" {
		for _, item := range applicationTemplates() {
			prefix := stringValue(item["command"]) + "："
			asciiPrefix := stringValue(item["command"]) + ":"
			for _, candidate := range []string{prefix, asciiPrefix} {
				if strings.HasPrefix(prompt, candidate) {
					key, name = stringValue(item["id"]), strings.TrimSpace(strings.TrimPrefix(prompt, candidate))
				}
			}
		}
	}
	if key == "" {
		return nil, false, nil
	}
	for _, item := range applicationTemplates() {
		if item["id"] != key {
			continue
		}
		if name == "" || len([]rune(name)) > 120 {
			return nil, true, businessError(400, "请选择模板并提供 1–120 个字符的应用名称")
		}
		definition := cloneAnyMap(asMap(item["definition"]))
		definition["name"] = name
		return definition, true, nil
	}
	return nil, true, businessError(400, "应用模板不存在")
}
