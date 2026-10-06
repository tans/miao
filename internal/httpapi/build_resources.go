package httpapi

import (
	"context"
	"encoding/json"

	"github.com/tans/miao/internal/harness"
)

// Resolve logical declarations only after their real tables exist. Resource
// creation still uses the shared reviewed BackendPlan, never model authority.
func (r appBuilderRuntime) nextBuildResource(ctx context.Context, run *harness.Run, definition buildDefinition, tables []map[string]any) (string, map[string]any, bool, error) {
	bySlug := map[string]map[string]any{}
	for _, table := range tables {
		bySlug[stringValue(table["slug"])] = table
	}
	for _, group := range []struct {
		collection string
		capability string
		resources  []buildResource
	}{{"business_actions", "business_actions.create", definition.Actions}, {"workflows", "workflows.configure", definition.Workflows}} {
		for _, resource := range group.resources {
			rows, err := r.s.PB.ListAll(ctx, group.collection, listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID), "name = "+pbFilterString(resource.Name), "status != \"archived\""), "created")
			if err != nil {
				return "", nil, false, err
			}
			if len(rows) > 1 {
				return "", nil, false, businessError(409, "同名业务配置不唯一，请明确整理后再继续："+resource.Name)
			}
			if len(rows) == 1 {
				if !equalJSON(rows[0]["definition"], resource.Definition) || defaultString(stringValue(rows[0]["description"]), "") != resource.Description {
					return "", nil, false, businessError(409, "同名业务配置已有不同定义，请从业务配置编辑具体修订，不会重复创建："+resource.Name)
				}
				continue
			}
			data, _ := json.Marshal(resource.Definition)
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
			} else {
				slug := stringValue(planned["table"])
				if bySlug[slug] == nil {
					return "", nil, false, businessError(409, "流程引用的数据表尚未创建")
				}
				planned["table_ref"] = tableRef(slug)
				planned["state_field_ref"] = backendOpaqueID("ref-", run.TenantID+"\x00"+run.AppID, "field", stringValue(bySlug[slug]["id"]), stringValue(planned["state_field"]))
				delete(planned, "table")
				delete(planned, "state_field")
			}
			return group.capability, map[string]any{"name": resource.Name, "description": resource.Description, "definition": planned}, false, nil
		}
	}
	return "", nil, true, nil
}
