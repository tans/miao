package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"github.com/tans/miao/internal/harness"
)

// The flat tree edit semantics follow @json-render/core 0.21.0's
// experimental-composition-tree: replacement keeps children/position, removal
// removes a subtree, and moving keeps that subtree intact. Execution stays in
// the existing Go harness; model output is only a validated draft proposal.
type uiEdit struct {
	Op      string         `json:"op"`
	Page    string         `json:"page,omitempty"`
	ID      string         `json:"id,omitempty"`
	Parent  string         `json:"parent,omitempty"`
	Before  string         `json:"before,omitempty"`
	Element map[string]any `json:"element,omitempty"`
	Value   any            `json:"value,omitempty"`
}

func uiEditRun(run *harness.Run) bool { return asMap(run.Context)["mode"] == "ui_edit" }

func (s *Server) prepareUIEdit(ctx context.Context, actor executionActor, input, runContext map[string]any) error {
	_, app, _, err := s.buildActor(ctx, s.PB, actor)
	if err != nil {
		return err
	}
	tables, err := s.appTables(ctx, app, actor.TenantID)
	if err != nil {
		return err
	}
	rows, _, _, err := s.PB.List(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID)), "-version", 1, 1)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return businessError(409, "请先创建一份界面草稿，再描述需要修改的内容")
	}
	base := rows[0]
	if basedID := stringValue(input["based_on_version_id"]); basedID != "" {
		base, err = s.PB.Get(ctx, "app_versions", basedID)
		if err != nil || base["app_id"] != actor.AppID || base["tenant_id"] != actor.TenantID {
			return businessError(404, "界面来源版本不存在")
		}
	} else if stringValue(base["published_at"]) != "" && base["id"] != app["published_version_id"] {
		// The newest version was superseded by a rollback, so continue
		// designing from the current published version.
		published, err := s.PB.Get(ctx, "app_versions", stringValue(app["published_version_id"]))
		if err != nil || published["app_id"] != actor.AppID || published["tenant_id"] != actor.TenantID {
			return businessError(409, "只能从当前正式版或未发布草稿继续设计")
		}
		base = published
	}
	if stringValue(base["published_at"]) != "" && base["id"] != app["published_version_id"] {
		return businessError(409, "只能从当前正式版或未发布草稿继续设计")
	}
	if expected, exists := input["expected_latest_version_id"]; exists && stringValue(expected) != stringValue(rows[0]["id"]) {
		return businessError(409, "界面草稿已变化，请重新载入后再设计")
	}
	if expected, exists := input["expected_published_version_id"]; exists && stringValue(expected) != stringValue(app["published_version_id"]) {
		return businessError(409, "正式界面已变化，请重新载入后再设计")
	}
	raw := base["definition"]
	if proposed, exists := input["initial_definition"]; exists {
		raw = proposed
	}
	definition, message := validateAppUIDefinition(raw, tables)
	if message != "" {
		return businessError(400, message)
	}
	if message = s.validateBusinessActionReferences(ctx, app, definition, false); message != "" {
		return businessError(400, message)
	}
	runContext["ui_initial_definition"] = definition
	runContext["ui_stored_definition"] = base["definition"]
	runContext["ui_base_id"], runContext["ui_latest_id"], runContext["ui_published_id"] = base["id"], rows[0]["id"], stringValue(app["published_version_id"])
	return nil
}

func composeUIEdits(initial map[string]any, edits []uiEdit, tables []map[string]any) (map[string]any, error) {
	if len(edits) == 0 || len(edits) > 32 {
		return nil, businessError(400, "界面修改需要 1–32 个受控编辑")
	}
	data, err := json.Marshal(initial)
	if err != nil {
		return nil, err
	}
	definition := map[string]any{}
	if err := json.Unmarshal(data, &definition); err != nil {
		return nil, err
	}
	for _, edit := range edits {
		if edit.Op == "app_title" {
			definition["title"] = edit.Value
			continue
		}
		if edit.Op == "add_page" {
			definition["pages"] = append(anySlice(definition["pages"]), edit.Value)
			continue
		}
		if edit.Op == "page_order" {
			ids := stringSlice(anySlice(edit.Value))
			pages := appUIPages(definition)
			if !isUIArray(edit.Value) || len(ids) != len(pages) {
				return nil, businessError(400, "页面排序必须包含所有页面且不能重复")
			}
			ordered, seen := []any{}, map[string]bool{}
			for _, id := range ids {
				var selected map[string]any
				for _, page := range pages {
					if page["id"] == id {
						selected = page
					}
				}
				if selected == nil || seen[id] {
					return nil, businessError(400, "页面排序包含未知或重复页面")
				}
				seen[id] = true
				ordered = append(ordered, selected)
			}
			definition["pages"] = ordered
			continue
		}
		var page map[string]any
		for _, candidate := range appUIPages(definition) {
			if candidate["id"] == edit.Page {
				page = candidate
				break
			}
		}
		if page == nil {
			return nil, businessError(400, "界面编辑引用未知页面")
		}
		if edit.Op == "remove_page" {
			pages := []any{}
			for _, candidate := range appUIPages(definition) {
				if candidate["id"] != edit.Page {
					pages = append(pages, candidate)
				}
			}
			definition["pages"] = pages
			continue
		}
		if edit.Op == "page_title" {
			page["title"] = edit.Value
			continue
		}
		if edit.Op == "add_source" {
			page["data_sources"] = append(anySlice(page["data_sources"]), edit.Value)
			continue
		}
		if edit.Op == "remove_source" {
			sources, found := []any{}, false
			for _, source := range asSliceMap(page["data_sources"]) {
				if source["id"] == edit.ID {
					found = true
				} else {
					sources = append(sources, source)
				}
			}
			if !found {
				return nil, businessError(400, "界面编辑引用未知数据源")
			}
			page["data_sources"] = sources
			continue
		}
		if containsString([]string{"fields", "form_fields", "query", "actions", "context"}, edit.Op) {
			found := false
			for _, source := range asSliceMap(page["data_sources"]) {
				if source["id"] == edit.ID {
					source[edit.Op], found = edit.Value, true
					if edit.Op == "context" && edit.Value == nil {
						delete(source, "context")
					}
				}
			}
			if !found {
				return nil, businessError(400, "界面编辑引用未知数据源")
			}
			continue
		}
		spec := asMap(page["spec"])
		elements := asMap(spec["elements"])
		element := asMap(elements[edit.ID])
		parentOf := func(id string) (map[string]any, int) {
			for _, raw := range elements {
				parent := asMap(raw)
				for index, child := range anySlice(parent["children"]) {
					if child == id {
						return parent, index
					}
				}
			}
			return nil, -1
		}
		subtree := map[string]bool{}
		var visit func(string)
		visit = func(id string) {
			if subtree[id] {
				return
			}
			subtree[id] = true
			for _, child := range stringSlice(anySlice(asMap(elements[id])["children"])) {
				visit(child)
			}
		}
		detach := func(parent map[string]any, index int) {
			children := anySlice(parent["children"])
			parent["children"] = append(children[:index:index], children[index+1:]...)
		}
		switch edit.Op {
		case "props":
			if elements[edit.ID] == nil {
				return nil, businessError(400, "界面编辑引用未知组件")
			}
			props, ok := edit.Value.(map[string]any)
			if !ok {
				return nil, businessError(400, "组件属性修改必须为对象")
			}
			for key, value := range props {
				asMap(element["props"])[key] = value
			}
		case "replace":
			if elements[edit.ID] == nil || edit.Element == nil {
				return nil, businessError(400, "替换组件不存在或缺少配方")
			}
			next := cloneAnyMap(edit.Element)
			next["children"] = element["children"]
			elements[edit.ID] = next
		case "remove":
			parent, index := parentOf(edit.ID)
			if parent == nil || edit.ID == spec["root"] {
				return nil, businessError(400, "根组件必须保留，不能删除未知组件")
			}
			visit(edit.ID)
			detach(parent, index)
			for id := range subtree {
				delete(elements, id)
			}
		case "add", "move":
			parent := asMap(elements[edit.Parent])
			if !containsString([]string{"Page", "Section"}, stringValue(parent["type"])) {
				return nil, businessError(400, "组件必须添加到已存在的页面或分组")
			}
			if edit.Op == "add" {
				if !validSlugID(edit.ID, 64) || elements[edit.ID] != nil || edit.Element == nil {
					return nil, businessError(400, "新增组件标识无效、重复或缺少配方")
				}
			} else {
				previous, index := parentOf(edit.ID)
				if previous == nil || edit.ID == spec["root"] {
					return nil, businessError(400, "根组件必须保留，不能移动未知组件")
				}
				visit(edit.ID)
				if subtree[edit.Parent] || edit.Before == edit.ID {
					return nil, businessError(400, "组件不能移到自身子树或自身之前")
				}
				detach(previous, index)
			}
			children := anySlice(parent["children"])
			position := len(children)
			if edit.Before != "" {
				position = -1
				for index, child := range children {
					if child == edit.Before {
						position = index
					}
				}
				if position < 0 {
					return nil, businessError(400, "组件排序目标不属于指定分组")
				}
			}
			if edit.Op == "add" {
				next := cloneAnyMap(edit.Element)
				if len(anySlice(next["children"])) > 0 {
					return nil, businessError(400, "新增组件配方不能携带子树；请逐项添加")
				}
				next["children"], elements[edit.ID] = []any{}, next
			}
			children = append(children[:position:position], append([]any{edit.ID}, children[position:]...)...)
			parent["children"] = children
		default:
			return nil, businessError(400, "不支持的界面编辑操作")
		}
	}
	validated, message := validateAppUIDefinition(definition, tables)
	if message != "" {
		return nil, businessError(400, message+"；原有效草稿已保留")
	}
	return validated, nil
}

func decodeUIEdits(content string) ([]uiEdit, string, error) {
	if len(content) > 200000 {
		return nil, "", businessError(400, "界面修改方案过大；原草稿已保留")
	}
	var response struct {
		Edits    []uiEdit `json:"edits"`
		Question string   `json:"question"`
	}
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return nil, "", businessError(400, "界面修改格式无效；请提供受控 edits 对象，原草稿已保留")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, "", businessError(400, "界面修改包含多个内容片段；原草稿已保留")
	}
	return response.Edits, response.Question, nil
}

func (r appBuilderRuntime) proposeUIEdits(ctx context.Context, run *harness.Run, edits []uiEdit) (harness.StepResult, error) {
	observation, err := r.Observe(ctx, run)
	if err != nil {
		return harness.StepResult{Outcome: harness.OutcomeFailed}, err
	}
	proposal, err := composeUIEdits(asMap(asMap(run.Context)["ui_initial_definition"]), edits, asSliceMap(observation.Values["tables"]))
	if err != nil {
		return harness.StepResult{Outcome: harness.OutcomeFailed}, err
	}
	app, err := r.s.PB.Get(ctx, "apps", run.AppID)
	if err != nil {
		return harness.StepResult{Outcome: harness.OutcomeFailed}, err
	}
	if message := r.s.validateBusinessActionReferences(ctx, app, proposal, false); message != "" {
		return harness.StepResult{Outcome: harness.OutcomeFailed}, businessError(400, message)
	}
	if equalJSON(proposal, asMap(run.Context)["ui_initial_definition"]) {
		return harness.StepResult{Outcome: harness.OutcomeWaiting, Value: map[string]any{"question": TLang(r.s.runLanguage(ctx, run), "方案没有产生有效界面差异，请补充需要修改的页面、内容或布局。原草稿已保留。")}}, nil
	}
	runContext := cloneAnyMap(asMap(run.Context))
	runContext["ui_proposal"], runContext["ui_edits"] = proposal, edits
	run.Context = runContext
	return harness.StepResult{Outcome: harness.OutcomeContinue, Value: map[string]any{"changes": diffAppUI(asMap(runContext["ui_stored_definition"]), proposal)}, Receipt: map[string]any{"validated": true, "base_version_id": runContext["ui_base_id"]}}, nil
}

func (r appBuilderRuntime) collectUIRequirements(ctx context.Context, run *harness.Run) (harness.StepResult, error) {
	prompt := strings.TrimSpace(run.Prompt)
	answers := anySlice(asMap(run.Context)["answers"])
	if len(answers) > 0 {
		prompt = strings.TrimSpace(stringValue(answers[len(answers)-1]))
	}
	if strings.HasPrefix(prompt, "{") {
		edits, question, err := decodeUIEdits(prompt)
		if err != nil {
			return harness.StepResult{Outcome: harness.OutcomeFailed}, err
		}
		if question != "" {
			return harness.StepResult{Outcome: harness.OutcomeWaiting, Value: map[string]any{"question": clip(question, 2000)}}, nil
		}
		return r.proposeUIEdits(ctx, run, edits)
	}
	return harness.StepResult{Outcome: harness.OutcomeWaiting, Value: map[string]any{"question": TLang(r.s.runLanguage(ctx, run), "自然语言界面编辑能力暂时下架。请使用界面编辑器，或补充明确 JSON 编辑方案；原草稿已保留。")}}, nil
}
