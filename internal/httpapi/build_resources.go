package httpapi

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/tans/miao/internal/harness"
)

// A collection script may only read through a connector that governs its
// source host, so declarations either name connectors explicitly or let the
// builder derive one host connector per script. Both resources stay drafts;
// only a review in the configuration views can enable them.
func collectionSourceOrigin(raw string) (string, string, bool) {
	parsed, err := externalURL(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return "", "", false
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return "", "", false
	}
	allowed := parsed.Path
	if allowed == "" || allowed == "/" {
		allowed = "/"
	} else if !collectionScriptPath(allowed) {
		return "", "", false
	}
	return "https://" + parsed.Hostname(), allowed, true
}

// Resolve logical declarations only after their real tables exist. Resource
// creation still uses the shared reviewed BackendPlan, never model authority.
func (r appBuilderRuntime) nextBuildResource(ctx context.Context, run *harness.Run, definition buildDefinition, tables []map[string]any) (string, map[string]any, bool, error) {
	bySlug := map[string]map[string]any{}
	for _, table := range tables {
		bySlug[stringValue(table["slug"])] = table
	}
	connectorRows := []map[string]any{}
	if len(definition.Connectors) > 0 || len(definition.CollectionScripts) > 0 {
		rows, err := r.s.PB.ListAll(ctx, "connectors", listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID), "status != \"archived\""), "created")
		if err != nil {
			return "", nil, false, err
		}
		connectorRows = rows
	}
	connectorForBase := func(baseURL string) map[string]any {
		for _, row := range connectorRows {
			if stringValue(asMap(row["definition"])["base_url"]) == baseURL {
				return row
			}
		}
		return nil
	}
	for _, group := range []struct {
		collection string
		capability string
		resources  []buildResource
	}{{"business_actions", "business_actions.create", definition.Actions}, {"workflows", "workflows.configure", definition.Workflows}, {"connectors", "connectors.configure", definition.Connectors}, {"collection_scripts", "collection_scripts.configure", definition.CollectionScripts}} {
		for _, resource := range group.resources {
			capability := group.capability
			desired := resource.Definition
			input := map[string]any{"name": resource.Name, "description": resource.Description}
			rows, err := r.s.PB.ListAll(ctx, group.collection, listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID), "name = "+pbFilterString(resource.Name), "status != \"archived\""), "created")
			if err != nil {
				return "", nil, false, err
			}
			if len(rows) > 1 {
				return "", nil, false, businessError(409, "同名业务配置不唯一，请明确整理后再继续："+resource.Name)
			}
			if len(rows) == 1 {
				// A stored script is the normalized form of its declaration,
				// so the builder compares that form: injected recipients and
				// the connector binding are not part of the declaration.
				if group.collection == "collection_scripts" && r.declaredScriptMatches(ctx, run, rows[0], resource.Definition) {
					continue
				}
				if equalJSON(rows[0]["definition"], desired) && (group.collection == "collection_scripts" || defaultString(stringValue(rows[0]["description"]), "") == resource.Description) {
					continue
				}
				if group.collection == "collection_scripts" || group.collection == "connectors" {
					return "", nil, false, businessError(409, "同名外部配置已存在且定义不同，请先在配置页审阅修改："+resource.Name)
				}
				capability = "business_actions.update"
				if group.collection == "workflows" {
					capability = "workflows.update"
				}
				input["resource_id"], input["expected_revision"] = rows[0]["id"], rows[0]["revision"]
			}
			data, _ := json.Marshal(desired)
			planned := map[string]any{}
			if err := json.Unmarshal(data, &planned); err != nil {
				return "", nil, false, err
			}
			tableRef := func(slug string) string {
				return backendOpaqueID("ref-", run.TenantID+"\x00"+run.AppID, "table", stringValue(bySlug[slug]["id"]))
			}
			if group.collection == "business_actions" {
				for _, step := range asSliceMap(planned["steps"]) {
					slug := stringValue(step["table"])
					if bySlug[slug] == nil {
						return "", nil, false, businessError(409, "业务动作引用的数据表尚未创建")
					}
					step["table_ref"] = tableRef(slug)
					delete(step, "table")
					fields := map[string]any{}
					for name, value := range asMap(step["data"]) {
						fields[backendOpaqueID("ref-", run.TenantID+"\x00"+run.AppID, "field", stringValue(bySlug[slug]["id"]), name)] = value
					}
					step["data"] = fields
				}
			} else if group.collection == "workflows" {
				slug := stringValue(planned["table"])
				if bySlug[slug] == nil {
					return "", nil, false, businessError(409, "流程引用的数据表尚未创建")
				}
				planned["table_ref"] = tableRef(slug)
				planned["state_field_ref"] = backendOpaqueID("ref-", run.TenantID+"\x00"+run.AppID, "field", stringValue(bySlug[slug]["id"]), stringValue(planned["state_field"]))
				delete(planned, "table")
				delete(planned, "state_field")
			} else if group.collection == "collection_scripts" {
				source := asMap(planned["source"])
				baseURL, allowedPath, ok := collectionSourceOrigin(stringValue(source["url"]))
				if !ok {
					return "", nil, false, businessError(400, "采集脚本「"+resource.Name+"」来源需要 HTTPS 默认端口地址")
				}
				connector := connectorForBase(baseURL)
				if connector == nil {
					return "connectors.configure", map[string]any{
						"name":        "采集来源 " + strings.TrimPrefix(baseURL, "https://"),
						"description": "由采集脚本「" + resource.Name + "」声明的公开来源；启用前请审阅可访问路径",
						"definition":  map[string]any{"type": "https_fetch", "base_url": baseURL, "allowed_paths": []any{allowedPath}},
					}, false, nil
				}
				source["__backend_connector_ref"] = backendOpaqueID("ref-", run.TenantID+"\x00"+run.AppID, "connector", stringValue(connector["id"]))
				planned["source"] = source
				// Notifications need real workspace members; a declaration
				// cannot know them, so the creating member receives the first
				// daily report until the script is reviewed.
				if len(uniqueStrings(planned["recipients"], 10)) == 0 {
					planned["recipients"] = []any{run.UserID}
				}
				target := asMap(planned["target"])
				slug := stringValue(target["table"])
				if bySlug[slug] == nil {
					return "", nil, false, businessError(409, "采集脚本引用的数据表尚未创建")
				}
				target["__backend_table_ref"] = tableRef(slug)
				delete(target, "table")
				planned["target"] = target
			}
			input["definition"] = planned
			return capability, input, false, nil
		}
	}
	return "", nil, true, nil
}

// A declaration cannot carry workspace member ids, so the declaration is
// compared through the same normalization the stored script went through.
func (r appBuilderRuntime) declaredScriptMatches(ctx context.Context, run *harness.Run, stored map[string]any, desired map[string]any) bool {
	candidate := cloneAnyMap(desired)
	if len(uniqueStrings(candidate["recipients"], 10)) == 0 {
		candidate["recipients"] = []any{run.UserID}
	}
	normalized, message := normalizeCollectionScriptDefinitionWith(ctx, r.s.PB, run.TenantID, run.AppID, candidate)
	if message != "" {
		return false
	}
	return equalJSON(asMap(stored["definition"]), normalized)
}

// Only resources named in this reviewed declaration may refresh an existing
// button binding; the resulting UI difference requires its own confirmation.
func (r appBuilderRuntime) syncDeclaredUIResources(ctx context.Context, run *harness.Run, definition buildDefinition, ui map[string]any) error {
	for _, group := range []struct {
		collection, idKey, revisionKey string
		resources                      []buildResource
	}{{"business_actions", "action_id", "action_revision", definition.Actions}, {"workflows", "workflow_id", "workflow_revision", definition.Workflows}} {
		for _, requested := range group.resources {
			rows, err := r.s.PB.ListAll(ctx, group.collection, listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID), "name = "+pbFilterString(requested.Name), "status != \"archived\""), "")
			if err != nil {
				return err
			}
			if len(rows) != 1 || !equalJSON(rows[0]["definition"], requested.Definition) {
				return businessError(409, "声明资源已变化，请重新整理界面绑定")
			}
			for _, page := range appUIPages(ui) {
				for _, source := range asSliceMap(page["data_sources"]) {
					for _, action := range asSliceMap(source["actions"]) {
						if action[group.idKey] == rows[0]["id"] {
							action[group.revisionKey] = rows[0]["revision"]
						}
					}
				}
			}
		}
	}
	return nil
}
