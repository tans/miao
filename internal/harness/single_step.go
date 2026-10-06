package harness

import (
	"context"
	"errors"
)

// Existing explicit single-operation callers use the same engine while their
// application adapters migrate to Runtime. The callback is not an agent loop.
type singleStepRuntime struct {
	plan    PlanFunc
	execute ExecuteFunc
}

func (r singleStepRuntime) Observe(_ context.Context, run *Run) (Observation, error) {
	return Observation{Values: map[string]any{"prompt": run.Prompt, "context": run.Context}}, nil
}
func (r singleStepRuntime) Enumerate(ctx context.Context, run *Run, _ Observation) ([]CandidateOption, error) {
	candidate, err := r.plan(ctx, run)
	if err != nil {
		return nil, err
	}
	if candidate == nil || candidate.Capability == "" {
		return nil, ErrCapability
	}
	if candidate.ID == "" {
		candidate.ID = token()
	}
	return []CandidateOption{{ID: candidate.ID, Capability: candidate.Capability, Input: candidate.Input, Write: candidate.Write, Evidence: candidate.Evidence}}, nil
}
func (r singleStepRuntime) Decide(_ context.Context, _ *Run, _ Observation, options []CandidateOption) (Decision, error) {
	return Decision{CandidateID: options[0].ID}, nil
}
func (r singleStepRuntime) Validate(context.Context, *Run, Observation, *Candidate) error {
	return nil
}
func (r singleStepRuntime) Execute(ctx context.Context, run *Run, candidate *Candidate) (StepResult, error) {
	value, err := r.execute(ctx, run, candidate)
	result := StepResult{Outcome: OutcomeContinue, Value: value, Receipt: value}
	if err == ErrWaiting {
		result.Outcome = OutcomeWaiting
	}
	if errors.Is(err, ErrCapability) || errors.Is(err, ErrChooserUnavailable) || errors.Is(err, ErrStaleVersion) {
		result.Outcome = OutcomeFailed
	}
	return result, err
}
func (r singleStepRuntime) CheckComplete(_ context.Context, run *Run, _ Observation) (Completion, error) {
	if run.Loop == nil || len(run.Loop.Steps) == 0 {
		return Completion{}, nil
	}
	last := run.Loop.Steps[len(run.Loop.Steps)-1]
	if last.CompletedAt.IsZero() || last.Result.Outcome != OutcomeContinue {
		return Completion{}, nil
	}
	return Completion{Satisfied: true, Evidence: []any{map[string]any{"step_id": last.ID, "result": last.Result.Value}}}, nil
}
