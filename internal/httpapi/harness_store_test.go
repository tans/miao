package httpapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/tans/miao/internal/harness"
)

func newStoredHarness(t *testing.T, f *integrationFixture, task bool) (harness.Store, *harness.Run) {
	t.Helper()
	run := harness.NewRun(f.tenantID, strings.TrimPrefix(f.base, "/api/apps/"), f.userID, "durability", nil)
	var store harness.Store = pocketHarnessStore{s: f.api}
	if task {
		row, err := f.api.PB.Create(context.Background(), "miao_runs", map[string]any{"tenant_id": run.TenantID, "app_id": run.AppID, "created_by": run.UserID, "task_id": "test-task", "event_key": run.ID, "snapshot": map[string]any{"goal": run.Prompt}, "status": "queued"})
		if err != nil {
			t.Fatal(err)
		}
		run.ID = stringValue(row["id"])
		store = taskHarnessStore{s: f.api}
	}
	if err := store.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	return store, run
}

func TestHarnessStoreRollsBackRunWhenEventFails(t *testing.T) {
	for _, task := range []bool{false, true} {
		t.Run(map[bool]string{false: "interactive", true: "task"}[task], func(t *testing.T) {
			f := newIntegration(t)
			store, run := newStoredHarness(t, f, task)
			owned, err := store.Acquire(context.Background(), run.ID, "owner", time.Now(), time.Now().Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			revision := owned.Revision
			owned.State, owned.Phase, owned.Sequence = harness.StateRecording, "record", 1
			owned.Loop = &harness.LoopState{ModelRequests: 4, Steps: []harness.Step{{ID: "effect", Result: harness.StepResult{Outcome: harness.OutcomeContinue, Receipt: "stored"}}}}
			hook := f.runtime.App.OnRecordCreate("miao_harness_events").BindFunc(func(*core.RecordEvent) error {
				return errors.New("injected event storage failure")
			})
			event := harness.Event{RunID: run.ID, Sequence: 1, Type: "receipt"}
			err = store.Commit(context.Background(), owned, &event)
			f.runtime.App.OnRecordCreate("miao_harness_events").Unbind(hook)
			if err == nil || owned.Revision != revision {
				t.Fatalf("failed transaction advanced caller revision: err=%v revision=%d", err, owned.Revision)
			}
			saved, err := store.Load(context.Background(), run.ID)
			if err != nil || saved.State != harness.StateQueued || saved.Sequence != 0 || saved.Revision != revision {
				t.Fatalf("state did not roll back: %+v err=%v", saved, err)
			}
			events, err := store.Events(context.Background(), run.ID, 0, 20)
			if err != nil || len(events) != 0 {
				t.Fatalf("failed event persisted: %+v err=%v", events, err)
			}
			if err := store.Commit(context.Background(), owned, &event); err != nil {
				t.Fatal(err)
			}
			if err := store.Release(context.Background(), run.ID, "owner"); err != nil {
				t.Fatal(err)
			}
			f.restart()
			if task {
				store = taskHarnessStore{s: f.api}
			} else {
				store = pocketHarnessStore{s: f.api}
			}
			saved, err = store.Load(context.Background(), run.ID)
			if err != nil || saved.Loop == nil || saved.Loop.ModelRequests != 4 || len(saved.Loop.Steps) != 1 || saved.Owner != "" {
				t.Fatalf("steps/budget/lease did not survive restart: %+v err=%v", saved, err)
			}
			events, err = store.Events(context.Background(), run.ID, 0, 20)
			if err != nil || len(events) != 1 || events[0].Sequence != saved.Sequence {
				t.Fatalf("event sequence did not survive restart: %+v err=%v", events, err)
			}
		})
	}
}

func TestHarnessStoreCancellationKeepsReceiptAndRejectsStaleOwner(t *testing.T) {
	f := newIntegration(t)
	store, run := newStoredHarness(t, f, false)
	ctx := context.Background()
	owned, err := store.Acquire(ctx, run.ID, "owner", time.Now(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	owned.State, owned.Sequence = harness.StateRecording, 1
	owned.Loop = &harness.LoopState{Steps: []harness.Step{{ID: "done", CompletedAt: time.Now(), Result: harness.StepResult{Outcome: harness.OutcomeContinue, Receipt: "real-effect"}}}}
	if err := store.Commit(ctx, owned, &harness.Event{RunID: run.ID, Sequence: 1, Type: "receipt"}); err != nil {
		t.Fatal(err)
	}
	stale := *owned
	cancelled, err := store.RequestCancel(ctx, run.ID, f.userID, time.Now())
	if err != nil || !cancelled.CancelRequested || cancelled.State != harness.StateRecording || len(cancelled.Loop.Steps) != 1 {
		t.Fatalf("cancellation changed execution receipt: %+v err=%v", cancelled, err)
	}
	if err := store.Commit(ctx, &stale, nil); !errors.Is(err, harness.ErrConflict) {
		t.Fatalf("stale executor overwrote cancellation: %v", err)
	}
	if err := store.Release(ctx, run.ID, "owner"); err != nil {
		t.Fatal(err)
	}
	second, err := store.Acquire(ctx, run.ID, "second", time.Now(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Release(ctx, run.ID, "owner"); !errors.Is(err, harness.ErrLeaseLost) {
		t.Fatalf("old owner released a new lease: %v", err)
	}
	if !second.CancelRequested || second.Loop.Steps[0].Result.Receipt != "real-effect" {
		t.Fatalf("lease acquisition lost cancellation or effect: %+v", second)
	}
	if err := store.Release(ctx, run.ID, "second"); err != nil {
		t.Fatal(err)
	}
}

func TestHarnessConcurrentExecutorsOnlyAdvanceOnce(t *testing.T) {
	f := newIntegration(t)
	store, run := newStoredHarness(t, f, false)
	entered, release := make(chan struct{}), make(chan struct{})
	var effects atomic.Int32
	plan := func(ctx context.Context, _ *harness.Run) (*harness.Candidate, error) {
		close(entered)
		select {
		case <-release:
			return &harness.Candidate{ID: "read", Capability: "records.query"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	execute := func(context.Context, *harness.Run, *harness.Candidate) (any, error) {
		effects.Add(1)
		return "receipt", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- harness.New(store, plan, execute).Resume(ctx, run.ID) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	other := pocketHarnessStore{s: New(f.api.PB)}
	err := harness.New(other, plan, execute).Resume(ctx, run.ID)
	close(release)
	if !errors.Is(err, harness.ErrBusy) {
		t.Fatalf("second engine acquired a running lease: %v", err)
	}
	if err := <-done; err != nil || effects.Load() != 1 {
		t.Fatalf("run did not advance exactly once: effects=%d err=%v", effects.Load(), err)
	}
}

func TestHarnessConfirmedDraftReconcilesLostReceiptWithoutReplay(t *testing.T) {
	f := newIntegration(t)
	appID := strings.TrimPrefix(f.base, "/api/apps/")
	options, err := backendHarnessCandidates(context.Background(), f.api.PB, f.tenantID, appID, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	var option harness.CandidateOption
	for _, candidate := range options {
		if candidate.Capability == "ui.compose" {
			option = candidate
			break
		}
	}
	if option.ID == "" {
		t.Fatal("UI candidate is unavailable")
	}
	engine := harness.New(pocketHarnessStore{s: f.api}, func(context.Context, *harness.Run) (*harness.Candidate, error) {
		return &harness.Candidate{ID: option.ID, Capability: option.Capability, Input: option.Input, Write: true}, nil
	}, f.api.executeHarness)
	run := harness.NewRun(f.tenantID, appID, f.userID, "compose", nil)
	if err := engine.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	waiting, err := engine.Store.Load(context.Background(), run.ID)
	if err != nil || waiting.Owner != "" {
		t.Fatalf("confirmation did not release ownership: %+v err=%v", waiting, err)
	}
	var rejected atomic.Bool
	hook := f.runtime.App.OnRecordCreate("miao_harness_events").BindFunc(func(e *core.RecordEvent) error {
		if strings.Contains(e.Record.GetString("data"), `"phase":"record"`) {
			rejected.Store(true)
			return errors.New("receipt event failed after draft creation")
		}
		return e.Next()
	})
	_, err = engine.Confirm(context.Background(), run.ID, waiting.Candidate.Version, "approve", f.userID)
	f.runtime.App.OnRecordCreate("miao_harness_events").Unbind(hook)
	if err == nil || !rejected.Load() {
		t.Fatalf("receipt failure was not injected: err=%v rejected=%v", err, rejected.Load())
	}
	interrupted, err := engine.Store.Load(context.Background(), run.ID)
	if err != nil || len(interrupted.Loop.Steps) != 1 || !interrupted.Loop.Steps[0].CompletedAt.IsZero() {
		t.Fatalf("uncommitted step was marked complete: %+v err=%v", interrupted, err)
	}
	f.restart()
	if err := f.api.harnessEngine().Resume(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	saved, err := (pocketHarnessStore{s: f.api}).Load(context.Background(), run.ID)
	if err != nil || saved.State != harness.StateCompleted || len(saved.Loop.Steps) != 1 || saved.Loop.Steps[0].Result.Receipt == nil {
		t.Fatalf("durable draft did not reconcile: %+v err=%v", saved, err)
	}
	_, count, _, err := f.api.PB.List(context.Background(), "app_versions", "harness_step_id = "+pbFilterString(saved.Loop.Steps[0].ID), "", 1, 20)
	if err != nil || count != 1 {
		t.Fatalf("draft was replayed: count=%d err=%v", count, err)
	}
	if err := f.api.harnessEngine().Resume(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
}

type durableTestRuntime struct {
	api   *Server
	table string
}

func (r durableTestRuntime) Observe(ctx context.Context, run *harness.Run) (harness.Observation, error) {
	_, count, _, err := r.api.PB.List(ctx, r.table, listFilter("tenant_id = "+pbFilterString(run.TenantID), "app_id = "+pbFilterString(run.AppID)), "", 1, 20)
	return harness.Observation{Revision: fmt.Sprint(count), Values: map[string]any{"count": count}}, err
}
func (r durableTestRuntime) Enumerate(_ context.Context, run *harness.Run, observation harness.Observation) ([]harness.CandidateOption, error) {
	return []harness.CandidateOption{{ID: fmt.Sprint("create-", observation.Values["count"]), Capability: "fixture.create", Write: true, Input: map[string]any{"name": fmt.Sprint(run.ID, "-", observation.Values["count"])}}}, nil
}
func (r durableTestRuntime) Decide(ctx context.Context, _ *harness.Run, _ harness.Observation, options []harness.CandidateOption) (harness.Decision, error) {
	if err := harness.ReserveModelRequest(ctx); err != nil {
		return harness.Decision{}, err
	}
	return harness.Decision{CandidateID: options[0].ID}, nil
}
func (r durableTestRuntime) Validate(ctx context.Context, run *harness.Run, _ harness.Observation, _ *harness.Candidate) error {
	_, err := r.api.authorizeWrite(ctx, r.api.PB, executionActor{UserID: run.UserID, TenantID: run.TenantID, AppID: run.AppID, Source: "interactive"}, false)
	return err
}
func (r durableTestRuntime) Execute(ctx context.Context, run *harness.Run, candidate *harness.Candidate) (harness.StepResult, error) {
	row, err := r.api.PB.Create(ctx, r.table, map[string]any{"tenant_id": run.TenantID, "app_id": run.AppID, "name": candidate.Input["name"], "status": "new"})
	return harness.StepResult{Outcome: harness.OutcomeContinue, Value: row, Receipt: row}, err
}
func (r durableTestRuntime) CheckComplete(_ context.Context, _ *harness.Run, observation harness.Observation) (harness.Completion, error) {
	count := intValue(observation.Values["count"])
	if count >= 2 {
		return harness.Completion{Satisfied: true, Evidence: []any{map[string]any{"actual_records": count}}}, nil
	}
	return harness.Completion{Missing: []string{"second record"}}, nil
}

func TestHarnessMultiStepConfirmationRestartsWithoutResettingBudget(t *testing.T) {
	f := newIntegration(t)
	engine := harness.NewRuntime(pocketHarnessStore{s: f.api}, durableTestRuntime{api: f.api, table: f.table}, harness.Limits{MaxModelRequests: 2})
	run := harness.NewRun(f.tenantID, strings.TrimPrefix(f.base, "/api/apps/"), f.userID, "two durable effects", nil)
	ctx := context.Background()
	if err := engine.Start(ctx, run); err != nil {
		t.Fatal(err)
	}
	first, err := engine.Store.Load(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Confirm(ctx, run.ID, first.Candidate.Version, "approve", f.userID)
	if err != nil || second.State != harness.StateWaiting || len(second.Loop.Steps) != 1 || second.Loop.ModelRequests != 2 {
		t.Fatalf("first effect/next confirmation missing: %+v err=%v", second, err)
	}
	f.restart()
	engine = harness.NewRuntime(pocketHarnessStore{s: f.api}, durableTestRuntime{api: f.api, table: f.table}, harness.Limits{MaxModelRequests: 2})
	if _, err := engine.Confirm(ctx, run.ID, first.Candidate.Version, "approve", f.userID); !errors.Is(err, harness.ErrStaleVersion) {
		t.Fatalf("first confirmation applied to second effect: %v", err)
	}
	completed, err := engine.Confirm(ctx, run.ID, second.Candidate.Version, "approve", f.userID)
	if err != nil || completed.State != harness.StateCompleted || len(completed.Loop.Steps) != 2 || completed.Loop.ModelRequests != 2 {
		t.Fatalf("restart lost steps or budget: %+v err=%v", completed, err)
	}
	if _, err := engine.Confirm(ctx, run.ID, second.Candidate.Version, "approve", f.userID); !errors.Is(err, harness.ErrNotConfirmable) {
		t.Fatalf("completed effect accepted another confirmation: %v", err)
	}
	_, count, _, err := f.api.PB.List(ctx, f.table, "", "", 1, 20)
	if err != nil || count != 2 {
		t.Fatalf("confirmation repeated a real database effect: count=%d err=%v", count, err)
	}
}

func TestHarnessExpiredLeaseConsumesBudgetBeforeNewExecution(t *testing.T) {
	f := newIntegration(t)
	store, run := newStoredHarness(t, f, false)
	ctx := context.Background()
	stale, err := store.Acquire(ctx, run.ID, "crashed", time.Now().Add(-2*time.Second), time.Now().Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	engine := harness.NewRuntime(store, durableTestRuntime{api: f.api, table: f.table}, harness.Limits{Timeout: time.Millisecond})
	if err := engine.Resume(ctx, run.ID); !errors.Is(err, harness.ErrBudgetExhausted) {
		t.Fatalf("crash reset accumulated execution time: %v", err)
	}
	if err := store.Commit(ctx, stale, nil); !errors.Is(err, harness.ErrConflict) && !errors.Is(err, harness.ErrLeaseLost) {
		t.Fatalf("expired owner could overwrite resumed run: %v", err)
	}
	saved, err := store.Load(ctx, run.ID)
	if err != nil || saved.State != harness.StateBudgetExhausted || saved.Loop.ActiveDuration < time.Second {
		t.Fatalf("crash duration was not retained: %+v err=%v", saved, err)
	}
	_, count, _, err := f.api.PB.List(ctx, f.table, "", "", 1, 20)
	if err != nil || count != 0 {
		t.Fatalf("expired budget executed an effect: count=%d err=%v", count, err)
	}
}

func TestHarnessRejectsMalformedStoredLoop(t *testing.T) {
	f := newIntegration(t)
	store, run := newStoredHarness(t, f, false)
	if _, err := f.api.PB.Update(context.Background(), "miao_harness_runs", run.ID, map[string]any{"loop": map[string]any{"model_requests": "invalid"}}); err != nil {
		t.Fatal(err)
	}
	engine := harness.NewRuntime(store, durableTestRuntime{api: f.api, table: f.table}, harness.Limits{})
	if err := engine.Resume(context.Background(), run.ID); err == nil {
		t.Fatal("invalid stored loop silently reset its budgets")
	}
	_, count, _, err := f.api.PB.List(context.Background(), f.table, "", "", 1, 20)
	if err != nil || count != 0 {
		t.Fatalf("invalid stored loop executed an effect: count=%d err=%v", count, err)
	}
}

func TestHarnessDisconnectedCallerRetainsModelBudgetAfterRestart(t *testing.T) {
	f := newIntegration(t)
	store, run := newStoredHarness(t, f, false)
	entered := make(chan struct{})
	engine := harness.New(store, func(ctx context.Context, _ *harness.Run) (*harness.Candidate, error) {
		if err := harness.ReserveModelRequest(ctx); err != nil {
			return nil, err
		}
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	}, f.api.executeHarness)
	engine.Limits.MaxModelRequests = 1
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- engine.Resume(ctx, run.ID) }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("model reservation was not reached")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("caller disconnection returned %v", err)
	}
	saved, err := store.Load(context.Background(), run.ID)
	if err != nil || saved.State != harness.StateQueued || saved.Owner != "" || saved.CancelRequested || saved.Loop.ModelRequests != 1 {
		t.Fatalf("disconnect lost reservation or cancelled run: %+v err=%v", saved, err)
	}
	f.restart()
	engine = harness.New(pocketHarnessStore{s: f.api}, func(ctx context.Context, _ *harness.Run) (*harness.Candidate, error) {
		return nil, harness.ReserveModelRequest(ctx)
	}, f.api.executeHarness)
	engine.Limits.MaxModelRequests = 1
	if err := engine.Resume(context.Background(), run.ID); !errors.Is(err, harness.ErrBudgetExhausted) {
		t.Fatalf("restart granted another model request: %v", err)
	}
}

func TestHarnessMigrationUpgradeKeepsExistingBusinessAndRunData(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	row := f.row("existing customer")
	_, run := newStoredHarness(t, f, false)
	_, task := newStoredHarness(t, f, true)
	runner := core.NewMigrationsRunner(f.runtime.App, core.AppMigrations)
	rollback := 0
	for index, migration := range core.AppMigrations.Items() {
		if migration.File == "20261012000000_durable_harness_loop.js" {
			rollback = len(core.AppMigrations.Items()) - index
			break
		}
	}
	if rollback == 0 {
		t.Fatal("durable harness migration is missing")
	}
	if _, err := runner.Down(rollback); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Up(); err != nil {
		t.Fatal(err)
	}
	restored, err := f.api.PB.Get(ctx, f.table, stringValue(row["id"]))
	if err != nil || restored["name"] != "existing customer" {
		t.Fatalf("migration changed business data: %+v err=%v", restored, err)
	}
	loaded, err := (pocketHarnessStore{s: f.api}).Load(ctx, run.ID)
	if err != nil || loaded.Prompt != "durability" || loaded.State != harness.StateQueued {
		t.Fatalf("migration changed existing interactive run: %+v err=%v", loaded, err)
	}
	loaded, err = (taskHarnessStore{s: f.api}).Load(ctx, task.ID)
	if err != nil || loaded.Prompt != "durability" || loaded.State != harness.StateQueued {
		t.Fatalf("migration changed existing task run: %+v err=%v", loaded, err)
	}
	if applied, err := runner.Up(); err != nil || len(applied) != 0 {
		t.Fatalf("migration reapplied unexpectedly: %+v err=%v", applied, err)
	}
}

func TestHarnessConfirmationRechecksRevokedAppRole(t *testing.T) {
	f := newIntegration(t)
	f.member("publisher", true)
	ctx := context.Background()
	user, err := f.api.PB.Find(ctx, "users", "email = "+pbFilterString("publisher@example.invalid"))
	if err != nil {
		t.Fatal(err)
	}
	appID, userID := strings.TrimPrefix(f.base, "/api/apps/"), stringValue(user["id"])
	options, err := backendHarnessCandidates(ctx, f.api.PB, f.tenantID, appID, userID)
	if err != nil {
		t.Fatal(err)
	}
	var option harness.CandidateOption
	for _, candidate := range options {
		if candidate.Capability == "ui.compose" {
			option = candidate
			break
		}
	}
	if option.ID == "" {
		t.Fatal("publisher UI candidate is missing")
	}
	engine := harness.New(pocketHarnessStore{s: f.api}, func(context.Context, *harness.Run) (*harness.Candidate, error) {
		return &harness.Candidate{ID: option.ID, Capability: option.Capability, Input: option.Input, Write: true}, nil
	}, f.api.executeHarness)
	run := harness.NewRun(f.tenantID, appID, userID, "compose", nil)
	if err := engine.Start(ctx, run); err != nil {
		t.Fatal(err)
	}
	waiting, err := engine.Store.Load(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	membership, err := f.api.PB.Find(ctx, "app_members", listFilter("app_id = "+pbFilterString(appID), "user_id = "+pbFilterString(userID)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.api.PB.Update(ctx, "app_members", stringValue(membership["id"]), map[string]any{"role": "viewer"}); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Confirm(ctx, run.ID, waiting.Candidate.Version, "approve", userID); !errors.Is(err, harness.ErrCapability) {
		t.Fatalf("stale role was accepted: %v", err)
	}
	saved, err := engine.Store.Load(ctx, run.ID)
	if err != nil || saved.State != harness.StateFailed {
		t.Fatalf("revoked capability did not stop the run: %+v err=%v", saved, err)
	}
	_, count, _, err := f.api.PB.List(ctx, "app_versions", "app_id = "+pbFilterString(appID), "", 1, 20)
	if err != nil || count != 0 {
		t.Fatalf("revoked role created a draft: count=%d err=%v", count, err)
	}
}

func TestHarnessAppliedBackendPlanReconcilesWithoutDuplicateCreation(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	candidates := f.request(f.token, "GET", f.base+"/backend/candidates", nil, 200)
	createID := ""
	for _, raw := range anySlice(candidates["candidates"]) {
		candidate := asMap(raw)
		if candidate["capability"] == "collections.create" {
			createID = stringValue(candidate["id"])
			break
		}
	}
	plan := f.request(f.token, "POST", f.base+"/backend/plans", map[string]any{"operations": []any{map[string]any{"candidate_id": createID, "input": map[string]any{"name": "Leads", "slug": "leads", "fields": []any{map[string]any{"name": "name", "type": "text"}}}}}}, 201)
	appID := strings.TrimPrefix(f.base, "/api/apps/")
	options, err := backendHarnessCandidates(ctx, f.api.PB, f.tenantID, appID, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	var option harness.CandidateOption
	for _, candidate := range options {
		if candidate.Capability == "backend_plan.apply" && candidate.Input["plan_id"] == plan["id"] {
			option = candidate
			break
		}
	}
	if option.ID == "" {
		t.Fatal("backend plan candidate is unavailable")
	}
	engine := harness.New(pocketHarnessStore{s: f.api}, func(context.Context, *harness.Run) (*harness.Candidate, error) {
		return &harness.Candidate{ID: option.ID, Capability: option.Capability, Input: option.Input, Write: true}, nil
	}, f.api.executeHarness)
	run := harness.NewRun(f.tenantID, appID, f.userID, "apply plan", nil)
	if err := engine.Start(ctx, run); err != nil {
		t.Fatal(err)
	}
	waiting, err := engine.Store.Load(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	var rejected atomic.Bool
	hook := f.runtime.App.OnRecordCreate("miao_harness_events").BindFunc(func(e *core.RecordEvent) error {
		if strings.Contains(e.Record.GetString("data"), `"phase":"record"`) {
			rejected.Store(true)
			return errors.New("injected receipt failure")
		}
		return e.Next()
	})
	_, err = engine.Confirm(ctx, run.ID, waiting.Candidate.Version, "approve", f.userID)
	f.runtime.App.OnRecordCreate("miao_harness_events").Unbind(hook)
	if err == nil || !rejected.Load() {
		t.Fatalf("receipt failure was not injected: %v", err)
	}
	f.restart()
	if err := f.api.harnessEngine().Resume(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	saved, err := (pocketHarnessStore{s: f.api}).Load(ctx, run.ID)
	if err != nil || saved.State != harness.StateCompleted || saved.Loop.Steps[0].Result.Receipt == nil {
		t.Fatalf("applied plan was not reconciled: %+v err=%v", saved, err)
	}
	_, count, _, err := f.api.PB.List(ctx, "app_collections", listFilter("app_id = "+pbFilterString(appID), "slug = \"leads\""), "", 1, 20)
	if err != nil || count != 1 {
		t.Fatalf("plan was replayed: count=%d err=%v", count, err)
	}
}
