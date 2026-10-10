package httpapi

import "github.com/tans/miao/internal/harness"

// decisionTree is persisted in the run context. It records the bounded
// decomposition proposed by the language model; every node still becomes a
// server candidate and must pass the normal Harness validation before it can
// execute. Keeping the tree in the run makes retries and resumed runs use the
// same proposal instead of asking the model to rediscover prior work.
func setDecisionTree(run *harness.Run, kind string, nodes []map[string]any) {
	ctx := cloneAnyMap(asMap(run.Context))
	ctx["decision_tree"] = map[string]any{"kind": kind, "cursor": 0, "nodes": nodes}
	run.Context = ctx
}

func advanceDecisionTree(run *harness.Run, capability string) {
	ctx := cloneAnyMap(asMap(run.Context))
	tree := asMap(ctx["decision_tree"])
	if len(tree) == 0 {
		return
	}
	nodes := asSliceMap(tree["nodes"])
	cursor := intValue(tree["cursor"])
	for cursor < len(nodes) && stringValue(nodes[cursor]["capability"]) != capability {
		cursor++
	}
	if cursor < len(nodes) {
		nodes[cursor]["status"] = "completed"
		cursor++
		tree["cursor"] = cursor
		tree["nodes"] = nodes
		ctx["decision_tree"] = tree
		run.Context = ctx
	}
}

func appDecisionTree(definition buildDefinition) []map[string]any {
	nodes := []map[string]any{{"id": "requirements", "capability": "requirements.collect", "status": "completed"}, {"id": "app", "capability": "apps.create"}}
	for _, table := range definition.Tables {
		nodes = append(nodes, map[string]any{"id": "table_" + table.Slug, "capability": "backend_plan.apply", "table": table.Slug})
	}
	nodes = append(nodes, map[string]any{"id": "ui", "capability": "ui.compose"}, map[string]any{"id": "publish", "capability": "ui.publish", "optional_capability": "ui.publish.public"})
	return nodes
}

func recordDecisionTree(request recordRequest) []map[string]any {
	return []map[string]any{{"id": "classify", "capability": "requirements.collect", "status": "completed"}, {"id": "record", "capability": "records." + request.Operation, "table": request.Table}}
}

func uiDecisionTree(edits []uiEdit) []map[string]any {
	return []map[string]any{{"id": "classify", "capability": "requirements.collect", "status": "completed"}, {"id": "patch", "capability": "ui.compose", "edit_count": len(edits)}, {"id": "publish", "capability": "ui.publish", "optional_capability": "ui.publish.public"}}
}
