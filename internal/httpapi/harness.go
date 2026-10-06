package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/pocketbase"
)

type pocketHarnessStore struct{ s *Server }

func (p pocketHarnessStore) Create(ctx context.Context, run *harness.Run) error {
	if _, err := json.Marshal(run); err != nil {
		return err
	}
	input, _ := json.Marshal(run.Context)
	candidate, _ := json.Marshal(run.Candidate)
	authority, _ := json.Marshal(run.Authority)
	result, _ := json.Marshal(run.Result)
	loop, _ := json.Marshal(run.Loop)
	row, err := p.s.PB.Create(ctx, "miao_harness_runs", map[string]any{"tenant_id": run.TenantID, "app_id": run.AppID, "user_id": run.UserID, "prompt": run.Prompt, "input": json.RawMessage(input), "state": run.State, "phase": run.Phase, "sequence": run.Sequence, "version": run.Version, "candidate": json.RawMessage(candidate), "authority": json.RawMessage(authority), "result": json.RawMessage(result), "loop": json.RawMessage(loop), "storage_revision": run.Revision, "lease_owner": "", "lease_expires_at": "", "cancel_requested": run.CancelRequested})
	if err != nil {
		return err
	}
	run.ID = stringValue(row["id"])
	run.CreatedAt, run.UpdatedAt = parseTime(row["created"]), parseTime(row["updated"])
	return nil
}
func (p pocketHarnessStore) Load(ctx context.Context, id string) (*harness.Run, error) {
	row, err := p.s.PB.Get(ctx, "miao_harness_runs", id)
	if err != nil {
		return nil, harnessLoadError(err)
	}
	if err := validateHarnessRecord(row, ""); err != nil {
		return nil, err
	}
	return harnessRun(row), nil
}
func (p pocketHarnessStore) Commit(ctx context.Context, run *harness.Run, event *harness.Event) error {
	return commitHarnessRecord(ctx, p.s.PB, "miao_harness_runs", run, event, "")
}
func (p pocketHarnessStore) Acquire(ctx context.Context, id, owner string, now, expires time.Time) (*harness.Run, error) {
	return acquireHarnessRecord(ctx, p.s.PB, "miao_harness_runs", id, owner, now, expires, "")
}
func (p pocketHarnessStore) Release(ctx context.Context, id, owner string) error {
	return releaseHarnessRecord(ctx, p.s.PB, "miao_harness_runs", id, owner, "")
}
func (p pocketHarnessStore) RequestCancel(ctx context.Context, id, actor string, now time.Time) (*harness.Run, error) {
	return requestHarnessCancel(ctx, p.s.PB, "miao_harness_runs", id, actor, now, "")
}
func (p pocketHarnessStore) Events(ctx context.Context, id string, after int64, limit int) ([]harness.Event, error) {
	rows, err := p.s.PB.ListAll(ctx, "miao_harness_events", "run_id = "+pbFilterString(id)+" && sequence > "+strconv.FormatInt(after, 10), "sequence")
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 200 {
		limit = 200
	}
	out := make([]harness.Event, 0, min(limit, len(rows)))
	for _, row := range rows[:min(limit, len(rows))] {
		out = append(out, harness.Event{RunID: id, Sequence: int64(intValue(row["sequence"])), Type: stringValue(row["event_type"]), Data: asMap(row["data"]), CreatedAt: parseTime(row["created"])})
	}
	return out, nil
}
func harnessRun(row map[string]any) *harness.Run {
	run := &harness.Run{ID: stringValue(row["id"]), TenantID: stringValue(row["tenant_id"]), AppID: stringValue(row["app_id"]), UserID: stringValue(row["user_id"]), Prompt: stringValue(row["prompt"]), State: harness.State(stringValue(row["state"])), Phase: stringValue(row["phase"]), Sequence: int64(intValue(row["sequence"])), Version: int64(intValue(row["version"])), Error: stringValue(row["error"]), CancelRequested: boolValue(row["cancel_requested"]), CreatedAt: parseTime(row["created"]), UpdatedAt: parseTime(row["updated"])}
	if value := row["input"]; value != nil {
		run.Context = value
	}
	if value := asMap(row["candidate"]); len(value) > 0 {
		run.Candidate = &harness.Candidate{ID: stringValue(value["id"]), Version: int64(intValue(value["version"])), Capability: stringValue(value["capability"]), Input: asMap(value["input"]), Write: boolValue(value["write"]), Evidence: asMap(value["evidence"])}
	}
	if value := asMap(row["authority"]); len(value) > 0 {
		permissions := []string{}
		for _, raw := range anySlice(value["permissions"]) {
			permissions = append(permissions, stringValue(raw))
		}
		run.Authority = &harness.Authority{Version: int64(intValue(value["version"])), TenantID: stringValue(value["tenant_id"]), AppID: stringValue(value["app_id"]), UserID: stringValue(value["user_id"]), Capability: stringValue(value["capability"]), Permissions: permissions, ConfirmedBy: stringValue(value["confirmed_by"]), ConfirmedAt: stringValue(value["confirmed_at"]), ExpiresAt: stringValue(value["expires_at"])}
	}
	run.Result = row["result"]
	run.Loop = decodeLoop(row["loop"])
	run.Revision = int64(intValue(row["storage_revision"]))
	run.Owner = stringValue(row["lease_owner"])
	run.LeaseExpiresAt = parseTime(row["lease_expires_at"])
	run.ActiveStartedAt = parseTime(row["active_started_at"])
	return run
}

func harnessRunFields(run *harness.Run, prefix string) map[string]any {
	candidate, _ := json.Marshal(run.Candidate)
	authority, _ := json.Marshal(run.Authority)
	result, _ := json.Marshal(run.Result)
	loop, _ := json.Marshal(run.Loop)
	field := func(name string) string { return prefix + name }
	fields := map[string]any{field("state"): run.State, field("phase"): run.Phase, field("sequence"): run.Sequence, field("version"): run.Version, field("candidate"): json.RawMessage(candidate), field("authority"): json.RawMessage(authority), field("result"): json.RawMessage(result), field("loop"): json.RawMessage(loop), "error": run.Error, "cancel_requested": run.CancelRequested}
	fields[field("storage_revision")], fields[field("lease_owner")], fields[field("lease_expires_at")] = run.Revision, run.Owner, formatLease(run.LeaseExpiresAt)
	fields[field("active_started_at")] = formatLease(run.ActiveStartedAt)
	if prefix == "" {
		input, _ := json.Marshal(run.Context)
		fields["input"], fields["app_id"] = json.RawMessage(input), run.AppID
	}
	return fields
}

func decodeLoop(raw any) *harness.LoopState {
	data, err := json.Marshal(raw)
	if err != nil || string(data) == "null" || string(data) == `""` {
		return nil
	}
	var loop harness.LoopState
	if json.Unmarshal(data, &loop) != nil {
		return nil
	}
	return &loop
}

func formatLease(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func (s *Server) harnessEngine() *harness.Engine {
	return harness.NewRuntime(pocketHarnessStore{s: s}, appBuilderRuntime{s: s}, harness.Limits{})
}

// planHarness accepts only an opaque candidate ID from the shared backend
// resolver. Capability and inputs are reconstructed from current state.
func (s *Server) planHarness(ctx context.Context, run *harness.Run) (*harness.Candidate, error) {
	options, err := backendHarnessCandidates(ctx, s.PB, run.TenantID, run.AppID, run.UserID)
	if err != nil {
		return nil, harness.ErrChooserUnavailable
	}
	if rawIDs, ok := asMap(run.Context)["candidate_ids"].([]any); ok {
		requested := make(map[string]bool, len(rawIDs))
		for _, rawID := range rawIDs {
			requested[stringValue(rawID)] = true
		}
		filtered := options[:0]
		for _, option := range options {
			if requested[option.ID] {
				filtered = append(filtered, option)
			}
		}
		options = filtered
	}
	if len(options) == 0 {
		return nil, harness.ErrChooserUnavailable
	}
	choices := make([]map[string]any, 0, len(options))
	byID := make(map[string]harness.CandidateOption, len(options))
	criteria := make(map[string]string, len(options))
	for _, option := range options {
		choices = append(choices, map[string]any{"id": option.ID, "capability": option.Capability, "description": option.Description, "write": option.Write})
		byID[option.ID] = option
		criteria[option.ID] = option.Description
	}
	answers, err := s.evaluateJev(ctx, run.TenantID, run.UserID, run.AppID, map[string]any{"prompt": run.Prompt, "context": run.Context, "candidates": choices, "upstream_commit": jevUpstreamCommit}, map[string]jevQuestion{
		"candidate_id": {Type: "choice", Instructions: "Choose exactly one legal candidate for the request. Do not invent an ID.", Criteria: criteria},
	})
	if err != nil {
		return nil, err
	}
	selected, ok := byID[answers["candidate_id"].Choice]
	if !ok {
		return nil, errors.New("Jev did not select an available candidate")
	}
	return &harness.Candidate{ID: selected.ID, Capability: selected.Capability, Write: selected.Write, Input: selected.Input, Evidence: map[string]any{"evaluator": "jev", "upstream_commit": jevUpstreamCommit}}, nil
}
func (s *Server) executeHarness(ctx context.Context, run *harness.Run, candidate *harness.Candidate) (any, error) {
	options, err := backendHarnessCandidates(ctx, s.PB, run.TenantID, run.AppID, run.UserID)
	if err != nil {
		return nil, harness.ErrChooserUnavailable
	}
	allowed := false
	for _, option := range options {
		if option.ID == candidate.ID && option.Capability == candidate.Capability && option.Write == candidate.Write {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, harness.ErrCapability
	}
	input := asMap(candidate.Input)
	switch candidate.Capability {
	case "records.query":
		tableName := stringValue(input["table"])
		if tableName == "" {
			return nil, errors.New("query candidate is missing table")
		}
		table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID), "slug = "+pbFilterString(tableName)))
		if err != nil {
			return nil, harness.ErrCapability
		}
		page := intValue(input["page"])
		if page < 1 {
			page = 1
		}
		rows, total, pages, err := s.PB.List(ctx, stringValue(table["pb_collection"]), "tenant_id = "+pbFilterString(run.TenantID)+" && app_id = "+pbFilterString(run.AppID), "-created", page, 25)
		if err != nil {
			return nil, err
		}
		items := []map[string]any{}
		for _, row := range rows {
			data := map[string]any{}
			for _, field := range asSliceMap(table["fields"]) {
				name := stringValue(field["name"])
				if name != "" && field["type"] != "file" && field["type"] != "relation" {
					data[name] = row[name]
				}
			}
			items = append(items, map[string]any{"id": row["id"], "updated_at": row["updated"], "data": data})
		}
		return map[string]any{"table": tableName, "items": items, "totalItems": total, "page": page, "totalPages": pages}, nil
	case "backend_plan.apply":
		planID := stringValue(input["plan_id"])
		plan, err := s.PB.Get(ctx, "app_backend_plans", planID)
		if err != nil || plan["tenant_id"] != run.TenantID || plan["app_id"] != run.AppID || plan["user_id"] != run.UserID || !containsString([]string{"draft", "applying"}, stringValue(plan["status"])) {
			return nil, harness.ErrCapability
		}
		if intValue(input["expected_revision"]) != intValue(plan["revision"]) {
			return nil, fmt.Errorf("backend plan revision is stale: %w", harness.ErrStaleVersion)
		}
		return s.applyBackendPlanForHarness(ctx, run, plan)
	case "ui.compose":
		definition := asMap(input["definition"])
		app, err := s.PB.Get(ctx, "apps", run.AppID)
		if err != nil {
			return nil, err
		}
		tables, err := s.appTables(ctx, app, run.TenantID)
		if err != nil {
			return nil, err
		}
		validated, msg := validateAppUIDefinition(definition, tables)
		if msg != "" {
			return nil, fmt.Errorf("%s: %w", msg, harness.ErrCapability)
		}
		id := identity{}
		id.User, err = s.PB.Get(ctx, "users", run.UserID)
		if err != nil {
			return nil, err
		}
		id.Tenant, err = s.PB.Get(ctx, "tenants", run.TenantID)
		if err != nil {
			return nil, err
		}
		id.Membership, err = s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(run.TenantID), "user_id = "+pbFilterString(run.UserID)))
		if err != nil {
			return nil, err
		}
		version, err := s.createHarnessUIDraft(ctx, run, app, id, validated)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": "draft", "version": version["id"], "published": false, "upstream_commit": jevUpstreamCommit}, nil
	default:
		return nil, harness.ErrCapability
	}
}
func (s *Server) executeHarnessBusinessAction(ctx context.Context, run *harness.Run, input map[string]any) (any, error) {
	actionID := stringValue(input["action_id"])
	if actionID == "" {
		return nil, errors.New("action_id is required")
	}
	action, err := s.PB.Get(ctx, "business_actions", actionID)
	if err != nil || action["tenant_id"] != run.TenantID || action["app_id"] != run.AppID || action["status"] != "enabled" {
		return nil, errors.New("business action is unavailable")
	}
	app, err := s.PB.Get(ctx, "apps", run.AppID)
	if err != nil {
		return nil, err
	}
	user, err := s.PB.Get(ctx, "users", run.UserID)
	if err != nil {
		return nil, err
	}
	tenant, err := s.PB.Get(ctx, "tenants", run.TenantID)
	if err != nil {
		return nil, err
	}
	membership, err := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(run.TenantID), "user_id = "+pbFilterString(run.UserID)))
	if err != nil {
		return nil, err
	}
	key := run.ID + ":" + actionID
	if prior, err := s.PB.Find(ctx, "business_action_runs", listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID), "action_id = "+pbFilterString(actionID), "idempotency_key = "+pbFilterString(key))); err == nil {
		return prior["result"], nil
	} else if !isMissing(err) {
		return nil, err
	}
	result, err := s.executeActionSteps(ctx, identity{User: user, Tenant: tenant, Membership: membership}, app, action, asMap(action["definition"]), asMap(input["input"]), "harness", func(tx *pocketbase.Client, steps []map[string]any) error {
		payload := map[string]any{"status": "completed", "action": actionID, "revision": action["revision"], "steps": steps}
		_, e := tx.Create(ctx, "business_action_runs", map[string]any{"tenant_id": run.TenantID, "app_id": run.AppID, "action_id": actionID, "revision": action["revision"], "idempotency_key": key, "status": "completed", "result": payload})
		return e
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"status": "completed", "action": actionID, "revision": action["revision"], "steps": result}, nil
}

func (s *Server) createHarnessUIDraft(ctx context.Context, run *harness.Run, app map[string]any, id identity, definition map[string]any) (map[string]any, error) {
	stepID := ""
	if run.Loop != nil && len(run.Loop.Steps) > 0 {
		stepID = run.Loop.Steps[len(run.Loop.Steps)-1].ID
	}
	var created map[string]any
	err := s.PB.Transaction(ctx, func(tx *pocketbase.Client) error {
		user, err := tx.Get(ctx, "users", run.UserID)
		if err != nil || boolValue(user["disabled"]) {
			return harness.ErrCapability
		}
		tenant, err := tx.Get(ctx, "tenants", run.TenantID)
		if err != nil {
			return err
		}
		membership, err := tx.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(run.TenantID), "user_id = "+pbFilterString(run.UserID)))
		if err != nil {
			return harness.ErrCapability
		}
		currentApp, err := tx.Get(ctx, "apps", run.AppID)
		if err != nil || boolValue(currentApp["archived"]) {
			return harness.ErrCapability
		}
		access, err := applicationAccess(ctx, tx, currentApp, identity{User: user, Tenant: tenant, Membership: membership})
		if err != nil || !canManageAppRole(string(access.Role)) {
			return harness.ErrCapability
		}
		tables, err := tx.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID)), "created")
		if err != nil {
			return err
		}
		validated, message := validateAppUIDefinition(definition, tables)
		if message != "" {
			return fmt.Errorf("%s: %w", message, harness.ErrCapability)
		}
		if stepID != "" {
			prior, err := tx.Find(ctx, "app_versions", "harness_step_id = "+pbFilterString(stepID))
			if err == nil {
				if prior["tenant_id"] != run.TenantID || prior["app_id"] != run.AppID || prior["created_by"] != run.UserID {
					return harness.ErrCapability
				}
				created = prior
				return nil
			}
			if !isMissing(err) {
				return err
			}
		}
		latest, _, _, err := tx.List(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID)), "-version", 1, 1)
		if err != nil {
			return err
		}
		version := 1
		if len(latest) > 0 {
			version = intValue(latest[0]["version"]) + 1
		}
		created, err = tx.Create(ctx, "app_versions", map[string]any{"tenant_id": run.TenantID, "app_id": run.AppID, "version": version, "summary": "Jev json-render draft", "created_by": id.User["id"], "definition": validated, "harness_step_id": stepID})
		return err
	})
	return created, err
}

func (s *Server) reconcileHarnessStep(ctx context.Context, run *harness.Run, step harness.Step) (harness.StepResult, error) {
	unknown := harness.StepResult{Outcome: harness.OutcomeUnknown}
	app, err := s.PB.Get(ctx, "apps", run.AppID)
	if err != nil || app["tenant_id"] != run.TenantID {
		return unknown, harness.ErrUnknown
	}
	user, err := s.PB.Get(ctx, "users", run.UserID)
	if err != nil {
		return unknown, err
	}
	if boolValue(user["disabled"]) || boolValue(app["archived"]) {
		return unknown, harness.ErrCapability
	}
	tenant, err := s.PB.Get(ctx, "tenants", run.TenantID)
	if err != nil {
		return unknown, err
	}
	membership, err := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(run.TenantID), "user_id = "+pbFilterString(run.UserID)))
	if err != nil || s.appPermission(ctx, app, identity{User: user, Tenant: tenant, Membership: membership}) == "" {
		return unknown, harness.ErrCapability
	}
	switch step.Candidate.Capability {
	case "ui.compose":
		version, err := s.PB.Find(ctx, "app_versions", "harness_step_id = "+pbFilterString(step.ID))
		if err != nil || version["tenant_id"] != run.TenantID || version["app_id"] != run.AppID || version["created_by"] != run.UserID {
			return unknown, harness.ErrUnknown
		}
		value := map[string]any{"status": "draft", "version": version["id"], "published": false, "upstream_commit": jevUpstreamCommit}
		return harness.StepResult{Outcome: harness.OutcomeContinue, Value: value, Receipt: value}, nil
	case "backend_plan.apply":
		plan, err := s.PB.Get(ctx, "app_backend_plans", stringValue(step.Candidate.Input["plan_id"]))
		if err != nil || plan["tenant_id"] != run.TenantID || plan["app_id"] != run.AppID || plan["user_id"] != run.UserID || plan["status"] != "applied" || intValue(plan["revision"]) != intValue(step.Candidate.Input["expected_revision"])+1 {
			return unknown, harness.ErrUnknown
		}
		receipt := asMap(plan["receipt"])
		if receipt["status"] != "applied" || len(anySlice(receipt["remaining_steps"])) > 0 || len(anySlice(receipt["completed_steps"])) == 0 {
			return unknown, harness.ErrUnknown
		}
		value := publicBackendPlan(plan)
		return harness.StepResult{Outcome: harness.OutcomeContinue, Value: value, Receipt: receipt}, nil
	default:
		return unknown, harness.ErrUnknown
	}
}

func (s *Server) applyBackendPlanForHarness(ctx context.Context, run *harness.Run, plan map[string]any) (any, error) {
	result, err := s.applyBackendPlanForActor(ctx, runActor(run), stringValue(plan["id"]), intValue(plan["revision"]))
	if err != nil {
		return nil, err
	}
	return publicBackendPlan(result), nil
}

func (s *Server) routesHarness() {
	s.Mux.HandleFunc("POST /api/agent/runs", s.auth(s.submitHarnessRun))
	s.Mux.HandleFunc("GET /api/agent/runs/{runId}", s.auth(s.getHarnessRun))
	s.Mux.HandleFunc("GET /api/agent/runs/{runId}/events", s.auth(s.getHarnessEvents))
	s.Mux.HandleFunc("POST /api/agent/runs/{runId}/confirm", s.auth(s.confirmHarnessRun))
	s.Mux.HandleFunc("POST /api/agent/runs/{runId}/cancel", s.auth(s.cancelHarnessRun))
	s.Mux.HandleFunc("POST /api/agent/runs/{runId}/continue", s.auth(s.continueHarnessRun))
	s.Mux.HandleFunc("POST /api/agent/runs/{runId}/resume", s.auth(s.resumeHarnessRun))
}
func (s *Server) ownedHarness(ctx context.Context, r *http.Request) (*harness.Run, bool) {
	id := who(r)
	run, err := s.harnessEngine().Store.Load(ctx, pathID(r, "runId"))
	if err != nil || run.TenantID != stringValue(id.Tenant["id"]) || run.UserID != stringValue(id.User["id"]) {
		return nil, false
	}
	if run.AppID == "" {
		_, err := s.workspaceActor(ctx, s.PB, runActor(run))
		return run, err == nil
	}
	app, err := s.PB.Get(ctx, "apps", run.AppID)
	if err != nil || app["tenant_id"] != run.TenantID || s.appPermission(ctx, app, id) == "" {
		return nil, false
	}
	return run, true
}
func (s *Server) submitHarnessRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	input := mapBody(r)
	prompt := strings.TrimSpace(stringValue(input["prompt"]))
	if prompt == "" || len([]rune(prompt)) > 12000 {
		writeError(w, 400, "prompt is required and must be at most 12000 characters")
		return
	}
	id := who(r)
	appID := stringValue(input["app_id"])
	if appID != "" {
		app, err := s.PB.Get(ctx, "apps", appID)
		if err != nil || app["tenant_id"] != id.Tenant["id"] || s.appPermission(ctx, app, id) == "" {
			writeError(w, 404, "应用不存在或你没有访问权限")
			return
		}
	}
	context := cloneAnyMap(asMap(input["context"]))
	if context["definition"] != nil {
		definition, err := parseBuildDefinition(context["definition"])
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
		data, _ := json.Marshal(definition)
		context["definition"] = json.RawMessage(data)
	}
	if len(anySlice(context["candidate_ids"])) > 0 && appID == "" {
		writeError(w, 400, "显式候选需要已有应用")
		return
	}
	run := harness.NewRun(stringValue(id.Tenant["id"]), appID, stringValue(id.User["id"]), prompt, context)
	engine := s.harnessEngine()
	startErr := engine.Start(ctx, run)
	saved, loadErr := engine.Store.Load(r.Context(), run.ID)
	if loadErr != nil {
		writeError(w, 503, "运行创建失败")
		return
	}
	if startErr != nil && saved.State != harness.StateFailed && saved.State != harness.StateWaiting && saved.State != harness.StateQueued && saved.State != harness.StateUnknown && saved.State != harness.StateBudgetExhausted {
		writeError(w, 503, "运行暂时无法推进")
		return
	}
	writeJSON(w, 202, map[string]any{"run": saved})
}
func (s *Server) getHarnessRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	run, ok := s.ownedHarness(ctx, r)
	if !ok {
		writeError(w, 404, "运行不存在")
		return
	}
	writeJSON(w, 200, map[string]any{"run": run})
}
func (s *Server) getHarnessEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	run, ok := s.ownedHarness(ctx, r)
	if !ok {
		writeError(w, 404, "运行不存在")
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	events, err := pocketHarnessStore{s}.Events(ctx, run.ID, after, 200)
	if err != nil {
		writeError(w, 503, "运行事件暂不可用")
		return
	}
	next := after
	if len(events) > 0 {
		next = events[len(events)-1].Sequence
	}
	writeJSON(w, 200, map[string]any{"events": events, "next_sequence": next})
}
func (s *Server) confirmHarnessRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	run, ok := s.ownedHarness(ctx, r)
	if !ok {
		writeError(w, 404, "运行不存在")
		return
	}
	input := mapBody(r)
	expected := int64(intValue(input["expected_version"]))
	decision := stringValue(input["decision"])
	if decision == "confirm" {
		decision = "approve"
	}
	if decision == "approve" {
		if run.Candidate == nil || run.State != harness.StateWaiting || run.Phase != "confirmation" {
			writeError(w, 409, "当前运行不等待确认")
			return
		}
		if err := s.harnessEngine().Runtime.Validate(ctx, run, harness.Observation{}, run.Candidate); err != nil {
			writeError(w, 403, "运行权限已变化")
			return
		}
	}
	saved, err := s.harnessEngine().Confirm(ctx, run.ID, expected, decision, stringValue(who(r).User["id"]))
	if err != nil {
		if errors.Is(err, harness.ErrStaleVersion) || errors.Is(err, harness.ErrNotConfirmable) || errors.Is(err, harness.ErrBusy) || errors.Is(err, harness.ErrConflict) || errors.Is(err, harness.ErrCancelled) {
			writeError(w, 409, err.Error())
		} else {
			writeError(w, 503, "确认保存失败")
		}
		return
	}
	writeJSON(w, 200, map[string]any{"run": saved})
}
func (s *Server) cancelHarnessRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	run, ok := s.ownedHarness(ctx, r)
	if !ok {
		writeError(w, 404, "运行不存在")
		return
	}
	saved, err := s.harnessEngine().Cancel(ctx, run.ID, stringValue(who(r).User["id"]))
	if err != nil {
		writeError(w, 503, "取消保存失败")
		return
	}
	writeJSON(w, 200, map[string]any{"run": saved})
}
