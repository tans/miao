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
			capability := group.capability
			input := map[string]any{"name": resource.Name, "description": resource.Description}
			rows, err := r.s.PB.ListAll(ctx, group.collection, listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID), "name = "+pbFilterString(resource.Name), "status != \"archived\""), "created")
			if err != nil {
				return "", nil, false, err
			}
			if len(rows) > 1 {
				return "", nil, false, businessError(409, "同名业务配置不唯一，请明确整理后再继续："+resource.Name)
			}
			if len(rows) == 1 {
				if equalJSON(rows[0]["definition"], resource.Definition) && defaultString(stringValue(rows[0]["description"]), "") == resource.Description {
					continue
				}
				capability = "business_actions.update"
				if group.collection == "workflows" {
					capability = "workflows.update"
				}
				input["resource_id"], input["expected_revision"] = rows[0]["id"], rows[0]["revision"]
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
			input["definition"] = planned
			return capability, input, false, nil
		}
	}
	return "", nil, true, nil
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
