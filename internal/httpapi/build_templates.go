package httpapi

import (
	"embed"
	"encoding/json"
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
