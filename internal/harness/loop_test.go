package harness

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type testRuntime struct {
	progress            int
	goal                int
	executions          int
	invalidDecision     bool
	mutateDecision      bool
	noProgress          bool
	write               bool
	wait                bool
	unknown             bool
	model               bool
	nestedModels        int
	falseCompletion     bool
	cancelDuringExecute func(*Run)
}

func (r *testRuntime) Observe(_ context.Context, run *Run) (Observation, error) {
	values := map[string]any{"progress": r.progress}
	if len(run.Loop.Steps) > 0 {
		values["previous_result"] = run.Loop.Steps[len(run.Loop.Steps)-1].Result.Value
	}
	return Observation{Revision: fmt.Sprint(r.progress), Values: values}, nil
}
func (r *testRuntime) Enumerate(_ context.Context, _ *Run, observation Observation) ([]CandidateOption, error) {
	if r.progress > 0 && observation.Values["previous_result"] != fmt.Sprintf("receipt-%d", r.progress) {
		return nil, errors.New("prior execution result was not observed")
	}
	return []CandidateOption{{ID: fmt.Sprintf("step-%d", r.progress), Capability: "records.query", Write: r.write, Input: map[string]any{"binding": "trusted-resource"}}}, nil
}
func (r *testRuntime) Decide(ctx context.Context, _ *Run, _ Observation, options []CandidateOption) (Decision, error) {
	if r.model {
		if err := ReserveModelRequest(ctx); err != nil {
			return Decision{}, err
		}
	}
	if r.invalidDecision {
		return Decision{CandidateID: "model-finish"}, nil
	}
	if r.mutateDecision {
		options[0].Input["binding"] = "invented-resource"
	}
	return Decision{CandidateID: options[0].ID}, nil
}
func (r *testRuntime) Validate(_ context.Context, _ *Run, _ Observation, candidate *Candidate) error {
	if candidate.Input["binding"] != "trusted-resource" {
		return errors.New("untrusted binding")
	}
	return nil
}
func (r *testRuntime) Execute(ctx context.Context, run *Run, _ *Candidate) (StepResult, error) {
	r.executions++
	for i := 0; i < r.nestedModels; i++ {
		if err := ReserveModelRequest(ctx); err != nil {
			return StepResult{Outcome: OutcomeFailed}, err
		}
	}
	if r.wait {
		return StepResult{Outcome: OutcomeWaiting, Value: "needs-information"}, nil
	}
	if r.unknown {
		return StepResult{Outcome: OutcomeUnknown, Value: "write may have happened"}, nil
	}
	if !r.noProgress {
		r.progress++
	}
	if r.cancelDuringExecute != nil {
		r.cancelDuringExecute(run)
	}
	value := fmt.Sprintf("receipt-%d", r.progress)
	return StepResult{Outcome: OutcomeContinue, Value: value, Receipt: map[string]any{"value": value}}, nil
}
func (r *testRuntime) CheckComplete(context.Context, *Run, Observation) (Completion, error) {
	if r.falseCompletion {
		return Completion{Satisfied: true}, nil
	}
	if r.progress >= r.goal {
		return Completion{Satisfied: true, Evidence: []any{map[string]any{"actual_progress": r.progress}}}, nil
	}
	return Completion{Missing: []string{"remaining operations"}}, nil
}

func TestMultiStepLoopUsesExecutionResultsAndRealGoal(t *testing.T) {
	store := &memoryStore{}
	runtime := &testRuntime{goal: 3}
	engine := NewRuntime(store, runtime, Limits{})
	run := NewRun("tenant", "app", "user", "three operations", nil)
	if err := engine.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Load(context.Background(), run.ID)
	if err != nil || saved.State != StateCompleted || runtime.executions != 3 || len(saved.Loop.Steps) != 3 || !saved.Loop.Completion.Satisfied {
		t.Fatalf("run=%+v executions=%d err=%v", saved, runtime.executions, err)
	}
	for i, step := range saved.Loop.Steps {
		if step.ID == "" || step.CompletedAt.IsZero() || step.Result.Value != fmt.Sprintf("receipt-%d", i+1) {
			t.Fatalf("invalid completed step=%+v", step)
		}
	}
	if err := engine.Resume(context.Background(), run.ID); err != nil || runtime.executions != 3 {
		t.Fatalf("terminal resume repeated execution: %d %v", runtime.executions, err)
	}
}

func TestLoopRejectsInvalidSelectionAndCompletion(t *testing.T) {
	for name, runtime := range map[string]*testRuntime{
		"unoffered finish":            {goal: 1, invalidDecision: true},
		"completion without evidence": {goal: 1, falseCompletion: true},
	} {
		t.Run(name, func(t *testing.T) {
			store := &memoryStore{}
			engine := NewRuntime(store, runtime, Limits{})
			run := NewRun("tenant", "app", "user", "goal", nil)
			if err := engine.Start(context.Background(), run); err == nil || runtime.executions != 0 {
				t.Fatalf("invalid choice/completion executed=%d err=%v", runtime.executions, err)
			}
			saved, _ := store.Load(context.Background(), run.ID)
			if saved.State == StateCompleted {
				t.Fatal("unverified goal completed")
			}
		})
	}
}

func TestDecisionCannotMutateTrustedBindings(t *testing.T) {
	store := &memoryStore{}
	runtime := &testRuntime{goal: 1, mutateDecision: true}
	engine := NewRuntime(store, runtime, Limits{})
	run := NewRun("tenant", "app", "user", "query", nil)
	if err := engine.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	saved, _ := store.Load(context.Background(), run.ID)
	if saved.Loop.Steps[0].Candidate.Input["binding"] != "trusted-resource" {
		t.Fatal("decision changed trusted input")
	}
}

func TestLoopBudgetsAndNoProgress(t *testing.T) {
	for name, test := range map[string]struct {
		limits     Limits
		runtime    *testRuntime
		cause      error
		state      State
		executions int
	}{
		"steps":            {Limits{MaxSteps: 1}, &testRuntime{goal: 2}, ErrBudgetExhausted, StateBudgetExhausted, 1},
		"decisions":        {Limits{MaxDecisions: 1}, &testRuntime{goal: 2}, ErrBudgetExhausted, StateBudgetExhausted, 1},
		"model requests":   {Limits{MaxModelRequests: 1}, &testRuntime{goal: 2, model: true}, ErrBudgetExhausted, StateBudgetExhausted, 1},
		"no progress":      {Limits{MaxNoProgress: 2}, &testRuntime{goal: 2, noProgress: true}, ErrNoProgress, StateFailed, 2},
		"observation size": {Limits{MaxObservationBytes: 1}, &testRuntime{goal: 2}, nil, StateFailed, 0},
	} {
		t.Run(name, func(t *testing.T) {
			store := &memoryStore{}
			engine := NewRuntime(store, test.runtime, test.limits)
			run := NewRun("tenant", "app", "user", "goal", nil)
			err := engine.Start(context.Background(), run)
			if err == nil || (test.cause != nil && !errors.Is(err, test.cause)) {
				t.Fatalf("error=%v", err)
			}
			saved, _ := store.Load(context.Background(), run.ID)
			if saved.State != test.state || test.runtime.executions != test.executions {
				t.Fatalf("state=%s executions=%d", saved.State, test.runtime.executions)
			}
		})
	}
}

func TestEachWriteRequiresItsOwnConfirmation(t *testing.T) {
	store := &memoryStore{}
	runtime := &testRuntime{goal: 2, write: true}
	engine := NewRuntime(store, runtime, Limits{})
	run := NewRun("tenant", "app", "user", "two writes", nil)
	if err := engine.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	first, _ := store.Load(context.Background(), run.ID)
	if first.State != StateWaiting || runtime.executions != 0 {
		t.Fatal("write bypassed confirmation")
	}
	second, err := engine.Confirm(context.Background(), run.ID, first.Candidate.Version, "approve", "user")
	if err != nil || second.State != StateWaiting || runtime.executions != 1 || second.Candidate.Version == first.Candidate.Version || second.Authority != nil {
		t.Fatalf("second write inherited approval: %+v err=%v", second, err)
	}
	if _, err := engine.Confirm(context.Background(), run.ID, first.Candidate.Version, "approve", "user"); !errors.Is(err, ErrStaleVersion) {
		t.Fatalf("stale approval accepted: %v", err)
	}
	completed, err := engine.Confirm(context.Background(), run.ID, second.Candidate.Version, "approve", "user")
	if err != nil || completed.State != StateCompleted || runtime.executions != 2 {
		t.Fatalf("run=%+v error=%v", completed, err)
	}
	if completed.Loop.Steps[0].Authority == nil || completed.Loop.Steps[1].Authority == nil {
		t.Fatal("confirmation evidence lost")
	}
}

func TestWaitingResumesSameStepAndPreservesBudget(t *testing.T) {
	store := &memoryStore{}
	runtime := &testRuntime{goal: 1, wait: true, model: true}
	engine := NewRuntime(store, runtime, Limits{})
	run := NewRun("tenant", "app", "user", "wait", nil)
	if err := engine.Start(context.Background(), run); !errors.Is(err, ErrWaiting) {
		t.Fatal(err)
	}
	waiting, _ := store.Load(context.Background(), run.ID)
	stepID := waiting.Loop.Steps[0].ID
	if waiting.State != StateWaiting || waiting.Loop.ModelRequests != 1 {
		t.Fatalf("waiting=%+v", waiting)
	}
	runtime.wait = false
	// A newly constructed engine simulates losing the in-process execution state.
	if err := NewRuntime(store, runtime, Limits{}).Resume(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	completed, _ := store.Load(context.Background(), run.ID)
	if completed.State != StateCompleted || len(completed.Loop.Steps) != 1 || completed.Loop.Steps[0].ID != stepID || completed.Loop.ModelRequests != 1 {
		t.Fatalf("resume lost step identity or budget: %+v", completed)
	}
}

func TestUnknownEffectDoesNotReplay(t *testing.T) {
	store := &memoryStore{}
	runtime := &testRuntime{goal: 1, unknown: true}
	engine := NewRuntime(store, runtime, Limits{})
	run := NewRun("tenant", "app", "user", "unknown effect", nil)
	if err := engine.Start(context.Background(), run); !errors.Is(err, ErrUnknown) {
		t.Fatal(err)
	}
	if err := NewRuntime(store, runtime, Limits{}).Resume(context.Background(), run.ID); !errors.Is(err, ErrUnknown) || runtime.executions != 1 {
		t.Fatalf("unknown effect replayed: executions=%d err=%v", runtime.executions, err)
	}
}

func TestInterruptedExecutionRequiresReconciliation(t *testing.T) {
	store := &memoryStore{}
	runtime := &testRuntime{goal: 1}
	run := NewRun("tenant", "app", "user", "crash", nil)
	run.Loop = &LoopState{Steps: []Step{{ID: "started", StartedAt: time.Now(), Candidate: Candidate{ID: "write", Write: true}}}}
	run.State = StateExecuting
	if err := store.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	err := NewRuntime(store, runtime, Limits{}).Resume(context.Background(), run.ID)
	if !errors.Is(err, ErrUnknown) || runtime.executions != 0 {
		t.Fatalf("interrupted effect replayed: %v", err)
	}
}

func TestCancellationKeepsCompletedEffectsAndStopsNextStep(t *testing.T) {
	store := &memoryStore{}
	runtime := &testRuntime{goal: 2}
	engine := NewRuntime(store, runtime, Limits{})
	runtime.cancelDuringExecute = func(run *Run) {
		if _, err := engine.Cancel(context.Background(), run.ID, "user"); err != nil {
			t.Error(err)
		}
	}
	run := NewRun("tenant", "app", "user", "cancel after first effect", nil)
	if err := engine.Start(context.Background(), run); !errors.Is(err, ErrCancelled) {
		t.Fatal(err)
	}
	saved, _ := store.Load(context.Background(), run.ID)
	if saved.State != StateCancelled || runtime.executions != 1 || len(saved.Loop.Steps) != 1 || saved.Loop.Steps[0].CompletedAt.IsZero() {
		t.Fatalf("cancellation lost receipt or continued: %+v", saved)
	}
}

type blockedRuntime struct{ *testRuntime }

func (r blockedRuntime) Observe(ctx context.Context, _ *Run) (Observation, error) {
	<-ctx.Done()
	return Observation{}, ctx.Err()
}

func TestExecutionTimeBudgetAndDisconnectedCaller(t *testing.T) {
	t.Run("active timeout", func(t *testing.T) {
		store := &memoryStore{}
		runtime := blockedRuntime{&testRuntime{goal: 1}}
		engine := NewRuntime(store, runtime, Limits{Timeout: 5 * time.Millisecond})
		run := NewRun("tenant", "app", "user", "timeout", nil)
		if err := engine.Start(context.Background(), run); !errors.Is(err, ErrBudgetExhausted) {
			t.Fatal(err)
		}
		saved, _ := store.Load(context.Background(), run.ID)
		if saved.State != StateBudgetExhausted || runtime.executions != 0 {
			t.Fatalf("budget run=%+v", saved)
		}
	})
	t.Run("disconnect is resumable", func(t *testing.T) {
		store := &memoryStore{}
		runtime := &testRuntime{goal: 1}
		engine := NewRuntime(store, runtime, Limits{})
		run := NewRun("tenant", "app", "user", "disconnect", nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := engine.Start(ctx, run); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		saved, _ := store.Load(context.Background(), run.ID)
		if saved.State != StateQueued || saved.CancelRequested {
			t.Fatalf("disconnect treated as user cancellation: %+v", saved)
		}
		if err := engine.Resume(context.Background(), run.ID); err != nil || runtime.executions != 1 {
			t.Fatalf("disconnect could not resume: %v", err)
		}
	})
}

func TestNestedModelCallsShareRunBudget(t *testing.T) {
	store := &memoryStore{}
	runtime := &testRuntime{goal: 1, model: true, nestedModels: 2}
	engine := NewRuntime(store, runtime, Limits{MaxModelRequests: 2})
	run := NewRun("tenant", "app", "user", "nested models", nil)
	if err := engine.Start(context.Background(), run); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatal(err)
	}
	saved, _ := store.Load(context.Background(), run.ID)
	if saved.State != StateBudgetExhausted || saved.Loop.ModelRequests != 2 || runtime.progress != 0 {
		t.Fatalf("nested calls bypassed shared budget: %+v", saved)
	}
}
