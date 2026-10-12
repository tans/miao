package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/tans/miao/internal/harness"
)

// Template packages share the reviewed application declaration whitelist.
// They carry structure only: tables, fields, actions, workflows, and collection
// script drafts. Records, members, permissions, run history, credentials, and
// published state never enter the package, and an import always creates a new
// isolated app through the same harness chain as built-in templates.
const (
	templateSchemaVersion = 1
	templateMaxBytes      = 512 << 10
)

type templatePackage struct {
	SchemaVersion int            `json:"schema_version"`
	Kind          string         `json:"kind"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Definition    map[string]any `json:"definition"`
}

// exportAppTemplate downloads a sanitized structure-only template for one app.
func (s *Server) exportAppTemplate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canManageAppRole(role) {
		writeError(w, 403, "只有应用管理者可以导出应用模板")
		return
	}
	tables, err := s.appTables(ctx, app, stringValue(who(r).Tenant["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	definition := map[string]any{"schema_version": 1, "name": stringValue(app["name"]), "description": clip(stringValue(app["description"]), 4000)}
	exportTables := []map[string]any{}
	for _, table := range tables {
		fields := []map[string]any{}
		for _, field := range asSliceMap(table["fields"]) {
			item := map[string]any{"name": stringValue(field["name"]), "label": stringValue(field["label"]), "type": stringValue(field["type"]), "required": boolValue(field["required"])}
			if typ := stringValue(field["type"]); typ == "select" {
				item["options"] = anySlice(field["options"])
			} else if typ == "relation" {
				item["target"] = stringValue(field["target"])
			}
			fields = append(fields, item)
		}
		exportTables = append(exportTables, map[string]any{"name": stringValue(table["name"]), "slug": stringValue(table["slug"]), "fields": fields})
	}
	definition["tables"] = exportTables
	for _, group := range []struct{ collection, key string }{{"business_actions", "actions"}, {"workflows", "workflows"}, {"collection_scripts", "collection_scripts"}} {
		rows, _, _, err := s.PB.List(ctx, group.collection, listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "status != \"archived\""), "created", 1, 8)
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
		resources := []map[string]any{}
		for _, row := range rows {
			stored := cloneAnyMap(asMap(row["definition"]))
			// Recipients are workspace member IDs; an import re-binds them.
			stored["recipients"] = []any{}
			resources = append(resources, map[string]any{"name": stringValue(row["name"]), "description": defaultString(stringValue(row["description"]), ""), "definition": stored})
		}
		definition[group.key] = resources
	}
	// Export must satisfy the same whitelist as a declaration; if a stored
	// configuration drifted out of range, fail instead of shipping a broken
	// template.
	validated, err := parseBuildDefinition(definition)
	if err != nil {
		writeError(w, 409, "导出的模板不合法；请核对应用结构后重试")
		return
	}
	packaged := map[string]any{"schema_version": templateSchemaVersion, "kind": "miao_app_template", "name": stringValue(app["name"]), "description": clip(stringValue(app["description"]), 4000), "definition": validated}
	w.Header().Set("Content-Disposition", "attachment; filename=\"miao-template-"+stringValue(app["id"])+".json\"")
	writeJSON(w, 200, packaged)
}

func (s *Server) importAppTemplate(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	raw, err := io.ReadAll(io.LimitReader(r.Body, templateMaxBytes+1))
	if err != nil || len(raw) > templateMaxBytes {
		writeError(w, 400, "应用模板过大")
		return
	}
	var pkg templatePackage
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&pkg); err != nil || decoder.Decode(new(any)) != io.EOF {
		writeError(w, 400, "应用模板格式无效")
		return
	}
	if pkg.SchemaVersion != templateSchemaVersion || pkg.Kind != "miao_app_template" {
		writeError(w, 400, "应用模板版本或类型不受支持")
		return
	}
	pkg.Name = strings.TrimSpace(pkg.Name)
	definition := cloneAnyMap(pkg.Definition)
	if definition == nil {
		writeError(w, 400, "应用模板缺少有效的应用定义")
		return
	}
	definition["name"], definition["description"] = pkg.Name, clip(pkg.Description, 4000)
	validated, err := parseBuildDefinition(definition)
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	if _, err := s.PB.Find(ctx, "apps", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "name = "+pbFilterString(validated.Name))); err == nil {
		writeError(w, 409, "已存在同名应用「"+validated.Name+"」；请先重命名现有应用或修改模板名称")
		return
	}
	prompt := "导入应用模板：" + validated.Name
	context := map[string]any{"mode": "build", "definition": validated}
	run := harness.NewRun(stringValue(id.Tenant["id"]), "", stringValue(id.User["id"]), clip(prompt, 12000), context)
	setDecisionTree(run, "app_build", appDecisionTree(validated))
	engine := s.harnessEngine()
	if err := engine.Store.Create(ctx, run); err != nil {
		writeError(w, 503, "运行创建失败")
		return
	}
	writeJSON(w, 202, map[string]any{"run": run})
}
