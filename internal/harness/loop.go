package harness

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func (e *Engine) advance(ctx context.Context, id string) (returned error) {
	if e.Store == nil || e.Runtime == nil {
		return errors.New("harness is not configured")
	}
	run, err := e.Store.Load(ctx, id)
	if err != nil {
		return err
	}
	if run.CancelRequested || run.State == StateCancelled {
		return ErrCancelled
	}
	switch run.State {
	case StateCompleted, StateFailed, StateBudgetExhausted, StateUnsupported:
		return nil
	case StateUnknown:
		return ErrUnknown
	}
	if run.Loop == nil {
		run.Loop = &LoopState{}
	}
	limits, err := normalizedLimits(e.Limits)
	if err != nil {
		return e.fail(ctx, run, err)
	}
	remaining := limits.Timeout - run.Loop.ActiveDuration
	if remaining <= 0 {
		return e.stop(ctx, run, StateBudgetExhausted, "budget", ErrBudgetExhausted)
	}
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	ctx = e.withModelBudget(ctx, run, limits.MaxModelRequests)
	started := e.now()
	defer func() {
		run.Loop.ActiveDuration += max(time.Duration(0), e.now().Sub(started))
		if err := e.saveRun(context.WithoutCancel(parent), run); returned == nil && err != nil {
			returned = err
		}
	}()
	if len(run.Loop.Steps) > 0 {
		last := run.Loop.Steps[len(run.Loop.Steps)-1]
		if last.CompletedAt.IsZero() && last.Result.Outcome != OutcomeWaiting {
			return e.stop(ctx, run, StateUnknown, "reconciliation", ErrUnknown)
		}
		if !last.CompletedAt.IsZero() && run.Candidate != nil && run.Candidate.Version == last.Candidate.Version {
			run.Candidate, run.Authority = nil, nil
		}
	}
	for {
		if err := e.boundary(ctx, parent, run); err != nil {
			return err
		}
		if err := e.transition(ctx, run, StateObserving, "observe", nil); err != nil {
			return err
		}
		observation, err := e.Runtime.Observe(ctx, run)
		if err != nil {
			return e.callbackError(ctx, parent, run, err)
		}
		data, err := json.Marshal(observation.Values)
		if err != nil || len(data) > limits.MaxObservationBytes {
			return e.fail(ctx, run, errors.New("harness observation is invalid or exceeds size limit"))
		}
		if observation.Revision == "" {
			hash := sha256.Sum256(data)
			observation.Revision = hex.EncodeToString(hash[:])
		}
		if len(run.Loop.Steps) > run.Loop.ObservedSteps {
			last := run.Loop.Steps[len(run.Loop.Steps)-1]
			if !last.CompletedAt.IsZero() {
				if observation.Revision == run.Loop.Observation.Revision {
					run.Loop.NoProgress++
				} else {
					run.Loop.NoProgress = 0
				}
				run.Loop.ObservedSteps = len(run.Loop.Steps)
			}
		}
		run.Loop.Observation = observation
		completion, err := e.Runtime.CheckComplete(ctx, run, observation)
		if err != nil {
			return e.callbackError(ctx, parent, run, err)
		}
		run.Loop.Completion = completion
		if completion.Satisfied {
			if len(completion.Evidence) == 0 || len(completion.Missing) > 0 {
				return e.fail(ctx, run, errors.New("harness goal completion lacks evidence or has missing requirements"))
			}
			if err := e.boundary(ctx, parent, run); err != nil {
				return err
			}
			if len(run.Loop.Steps) > 0 {
				last := run.Loop.Steps[len(run.Loop.Steps)-1]
				candidate := last.Candidate
				run.Candidate, run.Authority = &candidate, last.Authority
			}
			if err := e.transition(ctx, run, StateCompleted, "complete", nil); err != nil {
				return err
			}
			return e.emit(ctx, run, "completed", map[string]any{"evidence": completion.Evidence})
		}
		if run.Loop.NoProgress >= limits.MaxNoProgress {
			return e.fail(ctx, run, ErrNoProgress)
		}
		if run.Candidate == nil {
			if len(run.Loop.Steps) >= limits.MaxSteps || run.Loop.Decisions >= limits.MaxDecisions {
				return e.stop(ctx, run, StateBudgetExhausted, "budget", ErrBudgetExhausted)
			}
			if err := e.transition(ctx, run, StateEnumerating, "enumerate", nil); err != nil {
				return err
			}
			options, err := e.Runtime.Enumerate(ctx, run, observation)
			if err != nil {
				return e.callbackError(ctx, parent, run, err)
			}
			if len(options) == 0 {
				return e.stop(ctx, run, StateUnsupported, "unsupported", ErrCapability)
			}
			// Freeze trusted bindings before invoking the decision adapter.
			data, err := json.Marshal(options)
			if err != nil || len(data) > limits.MaxObservationBytes {
				return e.fail(ctx, run, errors.New("harness candidates are invalid or exceed size limit"))
			}
			var snapshot []CandidateOption
			if err := json.Unmarshal(data, &snapshot); err != nil {
				return e.fail(ctx, run, err)
			}
			byID := map[string]CandidateOption{}
			for _, option := range snapshot {
				if option.ID == "" || option.Capability == "" {
					return e.fail(ctx, run, ErrCapability)
				}
				if _, duplicate := byID[option.ID]; duplicate {
					return e.fail(ctx, run, errors.New("duplicate harness candidate ID"))
				}
				byID[option.ID] = option
			}
			run.Loop.Decisions++
			if err := e.transition(ctx, run, StateDeciding, "decide", nil); err != nil {
				return err
			}
			decision, err := e.Runtime.Decide(ctx, run, observation, options)
			if err != nil {
				return e.callbackError(ctx, parent, run, err)
			}
			selected, allowed := byID[decision.CandidateID]
			if !allowed {
				return e.fail(ctx, run, ErrCapability)
			}
			run.Candidate = &Candidate{ID: selected.ID, Version: run.Version + 1, Capability: selected.Capability, Input: selected.Input, Write: selected.Write, Evidence: selected.Evidence}
			run.Authority = nil
			run.Version = run.Candidate.Version
			if err := e.emit(ctx, run, "candidate", map[string]any{"candidate_id": selected.ID, "capability": selected.Capability, "version": run.Candidate.Version, "write": selected.Write}); err != nil {
				return err
			}
		}
		if run.Candidate.Write && run.Authority == nil {
			if err := e.transition(ctx, run, StateWaiting, "confirmation", nil); err != nil {
				return err
			}
			return e.emit(ctx, run, "confirmation_required", map[string]any{"candidate_id": run.Candidate.ID, "version": run.Candidate.Version})
		}
		if err := e.transition(ctx, run, StateValidating, "validate", nil); err != nil {
			return err
		}
		if run.Candidate.Write && !validAuthority(run, e.now()) {
			return e.fail(ctx, run, errors.New("candidate authority is missing, stale or expired"))
		}
		if err := e.Runtime.Validate(ctx, run, observation, run.Candidate); err != nil {
			return e.callbackError(ctx, parent, run, err)
		}
		if err := e.boundary(ctx, parent, run); err != nil {
			return err
		}
		index := len(run.Loop.Steps)
		if index > 0 && run.Loop.Steps[index-1].CompletedAt.IsZero() && run.Loop.Steps[index-1].Result.Outcome == OutcomeWaiting {
			index--
		} else {
			step := Step{ID: fmt.Sprintf("%s:%d", run.ID, run.Candidate.Version), Candidate: *run.Candidate, Authority: run.Authority, Observation: observation.Revision, StartedAt: e.now()}
			run.Loop.Steps = append(run.Loop.Steps, step)
		}
		if err := e.transition(ctx, run, StateExecuting, "execute", nil); err != nil {
			return err
		}
		result, executionErr := e.Runtime.Execute(ctx, run, run.Candidate)
		run.Result = result.Value
		run.Loop.Steps[index].Result = result
		if result.Outcome == OutcomeUnknown || (executionErr != nil && !errors.Is(executionErr, ErrWaiting) && run.Candidate.Write && result.Outcome != OutcomeFailed) {
			run.Loop.Steps[index].Result.Outcome = OutcomeUnknown
			return e.stop(context.WithoutCancel(ctx), run, StateUnknown, "reconciliation", ErrUnknown)
		}
		if errors.Is(executionErr, ErrWaiting) || result.Outcome == OutcomeWaiting {
			run.Loop.Steps[index].Result.Outcome = OutcomeWaiting
			if err := e.boundary(ctx, parent, run); err != nil {
				return err
			}
			if err := e.transition(ctx, run, StateWaiting, "execution_waiting", nil); err != nil {
				return err
			}
			if err := e.emit(ctx, run, "waiting", map[string]any{"step_id": run.Loop.Steps[index].ID}); err != nil {
				return err
			}
			return ErrWaiting
		}
		if executionErr != nil {
			if result.Outcome == OutcomeFailed {
				run.Loop.Steps[index].CompletedAt = e.now()
			}
			return e.callbackError(ctx, parent, run, executionErr)
		}
		if result.Outcome != OutcomeContinue {
			return e.fail(ctx, run, errors.New("harness executor returned an invalid outcome"))
		}
		run.Loop.Steps[index].CompletedAt = e.now()
		if err := e.transition(context.WithoutCancel(ctx), run, StateRecording, "record", nil); err != nil {
			return err
		}
		if err := e.boundary(ctx, parent, run); err != nil {
			return err
		}
		run.Candidate, run.Authority = nil, nil
		if err := e.saveRun(ctx, run); err != nil {
			return err
		}
	}
}

func (e *Engine) boundary(ctx, parent context.Context, run *Run) error {
	current, err := e.Store.Load(context.WithoutCancel(ctx), run.ID)
	if err != nil {
		return err
	}
	if current.CancelRequested || current.State == StateCancelled {
		run.CancelRequested = true
		run.Version = max(run.Version, current.Version)
		return e.stop(context.WithoutCancel(ctx), run, StateCancelled, "cancelled", ErrCancelled)
	}
	if ctx.Err() != nil {
		state, phase, cause := StateQueued, "interrupted", ctx.Err()
		if parent.Err() == nil {
			state, phase, cause = StateBudgetExhausted, "budget", ErrBudgetExhausted
		}
		return e.stop(context.WithoutCancel(ctx), run, state, phase, cause)
	}
	return nil
}

func (e *Engine) callbackError(ctx, parent context.Context, run *Run, cause error) error {
	if err := e.boundary(ctx, parent, run); err != nil {
		return err
	}
	if errors.Is(cause, ErrBudgetExhausted) {
		return e.stop(ctx, run, StateBudgetExhausted, "budget", cause)
	}
	if errors.Is(cause, ErrCancelled) {
		return e.stop(ctx, run, StateCancelled, "cancelled", cause)
	}
	return e.fail(ctx, run, cause)
}

func (e *Engine) stop(ctx context.Context, run *Run, state State, phase string, cause error) error {
	run.State, run.Phase, run.Error = state, phase, cause.Error()
	run.Version++
	if err := e.saveRun(ctx, run); err != nil {
		return err
	}
	if err := e.emit(ctx, run, string(state), map[string]any{"error": cause.Error()}); err != nil {
		return err
	}
	return cause
}

func (e *Engine) saveRun(ctx context.Context, run *Run) error {
	current, err := e.Store.Load(ctx, run.ID)
	if err != nil {
		return err
	}
	if current.CancelRequested || current.State == StateCancelled {
		run.CancelRequested = true
	}
	run.Version = max(run.Version, current.Version)
	return e.Store.Save(ctx, run)
}

func validAuthority(run *Run, now time.Time) bool {
	a, c := run.Authority, run.Candidate
	return a != nil && a.Version == c.Version && a.Capability == c.Capability &&
		a.TenantID == run.TenantID && a.AppID == run.AppID && a.UserID == run.UserID &&
		parseAuthorityTime(a.ExpiresAt).After(now)
}

func normalizedLimits(l Limits) (Limits, error) {
	if l.MaxSteps < 0 || l.MaxDecisions < 0 || l.MaxModelRequests < 0 || l.MaxNoProgress < 0 || l.MaxObservationBytes < 0 || l.Timeout < 0 {
		return l, errors.New("harness limits must be positive")
	}
	if l.MaxSteps == 0 {
		l.MaxSteps = 64
	}
	if l.MaxDecisions == 0 {
		l.MaxDecisions = 96
	}
	if l.MaxModelRequests == 0 {
		l.MaxModelRequests = 96
	}
	if l.MaxNoProgress == 0 {
		l.MaxNoProgress = 3
	}
	if l.MaxObservationBytes == 0 {
		l.MaxObservationBytes = 256 << 10
	}
	if l.Timeout == 0 {
		l.Timeout = 10 * time.Minute
	}
	return l, nil
}
