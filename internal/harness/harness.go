package harness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// State is the persisted lifecycle of one server-side run.
type State string

const (
	StateQueued          State = "queued"
	StateObserving       State = "observing"
	StateEnumerating     State = "enumerating"
	StateDeciding        State = "deciding"
	StateValidating      State = "validating"
	StateExecuting       State = "executing"
	StateRecording       State = "recording"
	StateWaiting         State = "waiting_confirmation"
	StateCompleted       State = "completed"
	StateFailed          State = "failed"
	StateCancelled       State = "cancelled"
	StateBudgetExhausted State = "budget_exhausted"
	StateUnknown         State = "unknown"
	StateUnsupported     State = "unsupported"
)

var (
	ErrNotFound        = errors.New("harness run not found")
	ErrStaleVersion    = errors.New("harness run version is stale")
	ErrNotConfirmable  = errors.New("harness run is not awaiting confirmation")
	ErrCancelled       = errors.New("harness run was cancelled")
	ErrCapability      = errors.New("harness capability is unavailable")
	ErrWaiting         = errors.New("harness run is waiting for input or confirmation")
	ErrBudgetExhausted = errors.New("harness run exhausted its budget")
	ErrNoProgress      = errors.New("harness run made no progress")
	ErrUnknown         = errors.New("harness execution effect requires reconciliation")
	ErrBusy            = errors.New("harness run is owned by another executor")
	ErrConflict        = errors.New("harness storage revision is stale")
	ErrLeaseLost       = errors.New("harness execution ownership was lost")
)

type Run struct {
	ID              string     `json:"id"`
	TenantID        string     `json:"tenant_id"`
	AppID           string     `json:"app_id"`
	UserID          string     `json:"user_id"`
	Prompt          string     `json:"prompt"`
	Context         any        `json:"context,omitempty"`
	State           State      `json:"state"`
	Phase           string     `json:"phase"`
	Sequence        int64      `json:"sequence"`
	Version         int64      `json:"version"`
	Candidate       *Candidate `json:"candidate,omitempty"`
	Authority       *Authority `json:"authority,omitempty"`
	Result          any        `json:"result,omitempty"`
	Error           string     `json:"error,omitempty"`
	CancelRequested bool       `json:"cancel_requested,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Loop            *LoopState `json:"loop,omitempty"`
	Revision        int64      `json:"-"`
	Owner           string     `json:"-"`
	LeaseExpiresAt  time.Time  `json:"-"`
	ActiveStartedAt time.Time  `json:"-"`
}

// Candidate is an immutable server-produced choice. Confirmation never accepts
// a replacement capability or input from the caller.
type Candidate struct {
	ID         string         `json:"id"`
	Version    int64          `json:"version"`
	Capability string         `json:"capability"`
	Input      map[string]any `json:"input"`
	Write      bool           `json:"write"`
	// Direct is a trusted runtime policy for a scoped user-requested record write.
	// It preserves the write effect/recovery semantics without a build approval.
	Direct   bool           `json:"direct,omitempty"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

type Authority struct {
	Version     int64    `json:"version"`
	TenantID    string   `json:"tenant_id"`
	AppID       string   `json:"app_id"`
	UserID      string   `json:"user_id"`
	Capability  string   `json:"capability"`
	Permissions []string `json:"permissions"`
	ConfirmedBy string   `json:"confirmed_by,omitempty"`
	ConfirmedAt string   `json:"confirmed_at,omitempty"`
	ExpiresAt   string   `json:"expires_at"`
}

type Event struct {
	RunID     string         `json:"run_id"`
	Sequence  int64          `json:"sequence"`
	Type      string         `json:"type"`
	Data      map[string]any `json:"data,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Store interface {
	Create(context.Context, *Run) error
	Load(context.Context, string) (*Run, error)
	Events(context.Context, string, int64, int) ([]Event, error)
	// Commit atomically checks the storage revision and execution lease, then
	// saves the run and optional event in one transaction.
	Commit(context.Context, *Run, *Event) error
	Acquire(context.Context, string, string, time.Time, time.Time) (*Run, error)
	Release(context.Context, string, string) error
	RequestCancel(context.Context, string, string, time.Time) (*Run, error)
}

type PlanFunc func(context.Context, *Run) (*Candidate, error)
type ExecuteFunc func(context.Context, *Run, *Candidate) (any, error)

type Engine struct {
	Store   Store
	Runtime Runtime
	Limits  Limits
	Now     func() time.Time
}

func New(store Store, plan PlanFunc, execute ExecuteFunc) *Engine {
	if plan == nil || execute == nil {
		return NewRuntime(store, nil, Limits{})
	}
	return NewRuntime(store, singleStepRuntime{plan: plan, execute: execute}, Limits{})
}

func NewRuntime(store Store, runtime Runtime, limits Limits) *Engine {
	return &Engine{Store: store, Runtime: runtime, Limits: limits, Now: time.Now}
}

func NewRun(tenantID, appID, userID, prompt string, value any) *Run {
	now := time.Now().UTC()
	return &Run{ID: token(), TenantID: tenantID, AppID: appID, UserID: userID, Prompt: prompt, Context: value, State: StateQueued, Phase: "queued", CreatedAt: now, UpdatedAt: now}
}

func (e *Engine) Start(ctx context.Context, run *Run) error {
	if e.Store == nil || e.Runtime == nil {
		return errors.New("harness is not configured")
	}
	if run.ID == "" {
		run.ID = token()
	}
	if run.State == "" {
		run.State = StateQueued
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = e.now()
	}
	if err := e.Store.Create(ctx, run); err != nil {
		return err
	}
	return e.advance(ctx, run.ID)
}

// Resume is idempotent: persisted candidates and authority are always reused.
func (e *Engine) Resume(ctx context.Context, id string) error { return e.advance(ctx, id) }

func (e *Engine) Confirm(ctx context.Context, id string, expectedVersion int64, decision, actor string) (*Run, error) {
	owner := token()
	run, err := e.Store.Acquire(ctx, id, owner, e.now(), e.now().Add(time.Minute))
	if err != nil {
		return nil, err
	}
	released := false
	defer func() {
		if !released {
			_ = e.Store.Release(context.WithoutCancel(ctx), id, owner)
		}
	}()
	if run.CancelRequested {
		return run, ErrCancelled
	}
	if run.State != StateWaiting || run.Phase != "confirmation" || run.Candidate == nil {
		return run, ErrNotConfirmable
	}
	if expectedVersion != run.Candidate.Version {
		return run, ErrStaleVersion
	}
	if decision == "reject" {
		run.State, run.Phase, run.Error = StateCancelled, "cancelled", "confirmation rejected"
		run.CancelRequested, run.Version = true, run.Version+1
		return run, e.emit(ctx, run, "cancelled", map[string]any{"reason": "confirmation_rejected"})
	}
	if decision != "approve" {
		return run, fmt.Errorf("decision must be approve or reject")
	}
	run.Authority = &Authority{Version: run.Candidate.Version, TenantID: run.TenantID, AppID: run.AppID, UserID: run.UserID, Capability: run.Candidate.Capability, Permissions: []string{run.Candidate.Capability}, ConfirmedBy: actor, ConfirmedAt: e.now().UTC().Format(time.RFC3339Nano), ExpiresAt: e.now().Add(15 * time.Minute).UTC().Format(time.RFC3339Nano)}
	run.State, run.Phase, run.Error = StateQueued, "confirmed", ""
	run.Version++
	if err := e.emit(ctx, run, "confirmed", map[string]any{"candidate_id": run.Candidate.ID, "version": run.Candidate.Version, "actor": actor}); err != nil {
		return run, err
	}
	if err := e.Store.Release(context.WithoutCancel(ctx), id, owner); err != nil {
		return run, err
	}
	released = true
	if err := e.advance(ctx, id); err != nil {
		if errors.Is(err, ErrBusy) {
			return e.Store.Load(ctx, id)
		}
		return nil, err
	}
	return e.Store.Load(ctx, id)
}

func (e *Engine) Cancel(ctx context.Context, id, actor string) (*Run, error) {
	return e.Store.RequestCancel(ctx, id, actor, e.now())
}

func (e *Engine) transition(ctx context.Context, run *Run, state State, phase string, data map[string]any) error {
	run.State, run.Phase, run.Version = state, phase, run.Version+1
	return e.emit(ctx, run, "phase", map[string]any{"phase": phase, "state": state, "data": data})
}
func (e *Engine) fail(ctx context.Context, run *Run, cause error) error {
	run.State, run.Phase, run.Error, run.Version = StateFailed, "failed", cause.Error(), run.Version+1
	if err := e.emit(ctx, run, "failed", map[string]any{"error": cause.Error()}); err != nil {
		return err
	}
	return cause
}
func (e *Engine) emit(ctx context.Context, run *Run, typ string, data map[string]any) error {
	run.Sequence++
	run.UpdatedAt = e.now().UTC()
	event := Event{RunID: run.ID, Sequence: run.Sequence, Type: typ, Data: data, CreatedAt: run.UpdatedAt}
	return e.commit(ctx, run, &event)
}

func (e *Engine) commit(ctx context.Context, run *Run, event *Event) error {
	err := e.Store.Commit(ctx, run, event)
	if !errors.Is(err, ErrConflict) {
		return err
	}
	current, loadErr := e.Store.Load(ctx, run.ID)
	if loadErr != nil || !current.CancelRequested || current.Owner != run.Owner {
		return err
	}
	// A controller may set cancellation while the owner is executing. Merge
	// only that control flag and the latest revision; execution receipts remain
	// owned by the current executor and are committed with the next sequence.
	run.CancelRequested = true
	run.Revision = current.Revision
	run.Sequence = current.Sequence
	if event != nil {
		run.Sequence++
		event.Sequence = run.Sequence
	}
	return e.Store.Commit(ctx, run, event)
}
func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}
func parseAuthorityTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
func token() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("run-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
