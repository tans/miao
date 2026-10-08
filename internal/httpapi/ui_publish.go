package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/tans/miao/internal/harness"
)

// publishRun marks requests whose goal is publishing the latest reviewed
// draft. They share the ui_edit conversation flow when no publishable draft
// exists, so a single run can revise and then publish.
func publishRun(run *harness.Run) bool { return asMap(run.Context)["mode"] == "publish" }

// publishCandidates offers the deterministic publish choices for the latest
// unpublished draft. The public variant is only offered when every source of
// at least one page can derive a compatible anonymous read policy; Jev picks
// between the two from the request, and the selected publish candidate is
// validated before any write.
func (r appBuilderRuntime) publishCandidates(ctx context.Context, run *harness.Run, observation harness.Observation) ([]harness.CandidateOption, error) {
	app, err := r.s.PB.Get(ctx, "apps", run.AppID)
	if err != nil {
		return nil, err
	}
	publishedID := stringValue(app["published_version_id"])
	rows, _, _, err := r.s.PB.List(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID)), "-version", 1, 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || stringValue(rows[0]["published_at"]) != "" || stringValue(rows[0]["id"]) == publishedID {
		return nil, nil
	}
	draft := rows[0]
	tables := asSliceMap(observation.Values["tables"])
	baseline := map[string]any{}
	if publishedID != "" {
		published, err := r.s.PB.Get(ctx, "app_versions", publishedID)
		if err == nil && stringValue(published["app_id"]) == run.AppID {
			baseline = asMap(published["definition"])
		}
	}
	changes := diffAppUI(baseline, asMap(draft["definition"]))
	input := map[string]any{"version_id": draft["id"], "expected_latest_version_id": draft["id"], "expected_published_version_id": publishedID}
	options := []harness.CandidateOption{{
		Capability: "ui.publish", Description: fmt.Sprintf("发布界面草稿 v%d 为正式版本（不启用匿名公开）", intValue(draft["version"])), Direct: true,
		Write: true, Input: input, Evidence: map[string]any{"changes": changes, "publication": "private"},
	}}
	if profile, ok := publicProfileFromDraft(draft, tables); ok {
		slug := publicationSlugFor(app, run.AppID)
		publicInput := map[string]any{"version_id": draft["id"], "expected_latest_version_id": draft["id"], "expected_published_version_id": publishedID, "publication": map[string]any{"enabled": true, "slug": slug, "pages": profile["pages"]}}
		scope := []string{}
		for _, page := range anySlice(profile["pages"]) {
			fields := []string{}
			for _, read := range anySlice(asMap(page)["reads"]) {
				fields = append(fields, stringSlice(anySlice(asMap(read)["fields"]))...)
			}
			seen := map[string]bool{}
			unique := []string{}
			for _, field := range fields {
				if !seen[field] {
					seen[field] = true
					unique = append(unique, field)
				}
			}
			scope = append(scope, fmt.Sprintf("%s: %s", stringValue(asMap(page)["title"]), strings.Join(unique, "、")))
		}
		options = append(options, harness.CandidateOption{
			Capability: "ui.publish.public", Description: fmt.Sprintf("发布界面草稿 v%d 并开启匿名公开访问（公开范围=草稿绑定的展示字段）", intValue(draft["version"])), Direct: true,
			Write: true, Input: publicInput, Evidence: map[string]any{"changes": changes, "publication": "public", "public_scope": strings.Join(scope, "；")},
		})
	}
	for index := range options {
		data, marshalErr := json.Marshal(options[index].Input)
		if marshalErr != nil {
			return nil, marshalErr
		}
		options[index].ID = backendOpaqueID("uip-", run.TenantID+"\x00"+run.AppID, options[index].Capability, string(data))
	}
	return options, nil
}

// publicProfileFromDraft derives an anonymous read policy from the draft's own
// bindings: display fields become public fields, and slug/status/published
// detection mirrors the previous manual defaults. Pages with relation context,
// incompatible sources or no qualifying reads are skipped entirely.
func publicProfileFromDraft(version map[string]any, tables []map[string]any) (map[string]any, bool) {
	pages := []any{}
	for _, rawPage := range appUIPages(asMap(version["definition"])) {
		reads := []any{}
		compatible := true
		sources := asSliceMap(rawPage["data_sources"])
		if len(sources) == 0 {
			continue
		}
		for _, source := range sources {
			if source["context"] != nil {
				compatible = false
				break
			}
			var table map[string]any
			for _, candidate := range tables {
				if stringValue(candidate["slug"]) == stringValue(source["collection"]) {
					table = candidate
					break
				}
			}
			if table == nil {
				compatible = false
				break
			}
			var slugField, statusField string
			var statusOptions []string
			publicFields := []string{}
			for _, field := range asSliceMap(table["fields"]) {
				name, fieldType := stringValue(field["name"]), stringValue(field["type"])
				switch name {
				case "slug":
					if fieldType == "text" {
						slugField = name
					}
				case "status":
					if fieldType == "select" {
						statusField, statusOptions = name, stringSlice(anySlice(field["options"]))
					}
				}
				if containsString([]string{"text", "number", "bool", "date", "email", "url", "select"}, fieldType) && containsString(stringSlice(anySlice(source["fields"])), name) {
					publicFields = append(publicFields, name)
				}
			}
			if slugField == "" || statusField == "" || len(publicFields) == 0 {
				compatible = false
				break
			}
			published := ""
			for _, candidate := range []string{"published", "live", "已发布"} {
				if containsString(statusOptions, candidate) {
					published = candidate
					break
				}
			}
			if published == "" && len(statusOptions) > 0 {
				published = statusOptions[len(statusOptions)-1]
			}
			if published == "" {
				compatible = false
				break
			}
			read := map[string]any{"source": stringValue(source["id"]), "table": stringValue(source["collection"]), "fields": publicFields, "images": []any{}, "html_fields": []any{}, "status_field": statusField, "published_value": published, "slug_field": slugField}
			if containsString(publicFields, "title") {
				read["seo_title_field"] = "title"
			}
			if containsString(publicFields, "description") {
				read["seo_description_field"] = "description"
			}
			reads = append(reads, read)
		}
		if !compatible || len(reads) != len(sources) {
			continue
		}
		pages = append(pages, map[string]any{"id": stringValue(rawPage["id"]), "title": stringValue(rawPage["title"]), "reads": reads})
	}
	if len(pages) == 0 {
		return nil, false
	}
	return map[string]any{"pages": pages}, true
}

func publicationSlugFor(app map[string]any, appID string) string {
	current := asMap(app["public_publication"])
	if slug := strings.TrimSpace(strings.ToLower(stringValue(current["slug"]))); slug != "" {
		return slug
	}
	if slug := strings.TrimSpace(strings.ToLower(stringValue(app["public_slug"]))); slug != "" {
		return slug
	}
	short := appID
	if len(short) > 8 {
		short = short[:8]
	}
	return "app-" + short
}

func (r appBuilderRuntime) executeUIPublish(ctx context.Context, run *harness.Run, candidate *harness.Candidate) (harness.StepResult, error) {
	value, err := r.s.publishHarnessDraft(ctx, runActor(run), candidate.Input)
	if err != nil {
		outcome := harness.OutcomeFailed
		if errStatus(err) >= 500 && !errors.Is(err, harness.ErrCapability) && !errors.Is(err, harness.ErrStaleVersion) {
			outcome = harness.OutcomeUnknown
		}
		return harness.StepResult{Outcome: outcome, Value: value}, err
	}
	return harness.StepResult{Outcome: harness.OutcomeContinue, Value: value, Receipt: value}, nil
}

func (r appBuilderRuntime) completeUIPublish(ctx context.Context, run *harness.Run, observation harness.Observation) (harness.Completion, error) {
	for index := len(run.Loop.Steps) - 1; index >= 0; index-- {
		step := run.Loop.Steps[index]
		if step.Candidate.Capability != "ui.publish" && step.Candidate.Capability != "ui.publish.public" {
			continue
		}
		if step.CompletedAt.IsZero() || step.Result.Outcome != harness.OutcomeContinue {
			break
		}
		receipt := asMap(step.Result.Value)
		versionID := stringValue(receipt["version"])
		if versionID == "" {
			return harness.Completion{}, harness.ErrCapability
		}
		version, err := r.s.PB.Get(ctx, "app_versions", versionID)
		if err != nil || version["tenant_id"] != run.TenantID || version["app_id"] != run.AppID {
			return harness.Completion{}, harness.ErrCapability
		}
		if stringValue(version["published_at"]) == "" {
			return harness.Completion{}, businessError(409, "发布回执缺少正式版本状态")
		}
		run.Result = step.Result.Value
		return harness.Completion{Satisfied: true, Evidence: []any{map[string]any{"app_id": run.AppID, "version_id": versionID, "tables": observation.Values["tables"], "stage": "published", "published": true, "publication": receipt["publication"], "public_slug": receipt["public_slug"]}}}, nil
	}
	return harness.Completion{Missing: []string{"有效发布回执"}}, nil
}

func (r appBuilderRuntime) reconcileUIPublish(ctx context.Context, run *harness.Run, step harness.Step) (harness.StepResult, error) {
	if step.Candidate.Capability != "ui.publish" && step.Candidate.Capability != "ui.publish.public" {
		return harness.StepResult{Outcome: harness.OutcomeUnknown}, harness.ErrUnknown
	}
	version, err := r.s.PB.Find(ctx, "app_versions", "harness_step_id = "+pbFilterString(step.ID))
	if err != nil || version["tenant_id"] != run.TenantID || version["app_id"] != run.AppID {
		return harness.StepResult{Outcome: harness.OutcomeUnknown}, harness.ErrUnknown
	}
	if stringValue(version["published_at"]) == "" {
		return harness.StepResult{Outcome: harness.OutcomeUnknown}, harness.ErrUnknown
	}
	publication := "private"
	if step.Candidate.Capability == "ui.publish.public" {
		publication = "public"
	}
	value := map[string]any{"status": "published", "app_id": run.AppID, "version": version["id"], "version_number": version["version"], "published": true, "publication": publication}
	return harness.StepResult{Outcome: harness.OutcomeContinue, Value: value, Receipt: value}, nil
}
