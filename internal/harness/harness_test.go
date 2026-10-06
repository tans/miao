package harness

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu     sync.Mutex
	runs   map[string]*Run
	events []Event
}

func cloneRun(run *Run) (*Run, error) {
	data, err := json.Marshal(run)
	if err != nil {
		return nil, err
	}
	var copy Run
	err = json.Unmarshal(data, &copy)
	copy.Revision, copy.Owner, copy.LeaseExpiresAt, copy.ActiveStartedAt = run.Revision, run.Owner, run.LeaseExpiresAt, run.ActiveStartedAt
	return &copy, err
}
func (m *memoryStore) Create(_ context.Context, run *Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.runs == nil {
		m.runs = map[string]*Run{}
	}
	copy, err := cloneRun(run)
	if err != nil {
		return err
	}
	m.runs[run.ID] = copy
	return nil
}
func (m *memoryStore) Load(_ context.Context, id string) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run := m.runs[id]
	if run == nil {
		return nil, ErrNotFound
	}
	return cloneRun(run)
}
func (m *memoryStore) Save(_ context.Context, run *Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copy, err := cloneRun(run)
	if err != nil {
		return err
	}
	m.runs[run.ID] = copy
	return nil
}
func (m *memoryStore) Commit(_ context.Context, run *Run, event *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.runs[run.ID]
	if current == nil {
		return ErrNotFound
	}
	if current.Revision != run.Revision {
		return ErrConflict
	}
	if current.Owner == "" || current.Owner != run.Owner || !current.LeaseExpiresAt.After(time.Now()) {
		return ErrLeaseLost
	}
	copy, err := cloneRun(run)
	if err != nil {
		return err
	}
	copy.Revision++
	m.runs[run.ID] = copy
	run.Revision = copy.Revision
	if event != nil {
		m.events = append(m.events, *event)
	}
	return nil
}
func (m *memoryStore) Acquire(_ context.Context, id, owner string, now, expires time.Time) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run := m.runs[id]
	if run == nil {
		return nil, ErrNotFound
	}
	if run.Owner != "" && run.LeaseExpiresAt.After(now) {
		return nil, ErrBusy
	}
	chargeDuration(run, now)
	run.Owner, run.LeaseExpiresAt, run.Revision = owner, expires, run.Revision+1
	run.ActiveStartedAt = now
	return cloneRun(run)
}
func (m *memoryStore) Release(_ context.Context, id, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run := m.runs[id]
	if run == nil {
		return ErrNotFound
	}
	if run.Owner != owner {
		return ErrLeaseLost
	}
	chargeDuration(run, time.Now())
	run.Owner, run.LeaseExpiresAt, run.ActiveStartedAt, run.Revision = "", time.Time{}, time.Time{}, run.Revision+1
	return nil
}
func (m *memoryStore) RequestCancel(_ context.Context, id, actor string, now time.Time) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run := m.runs[id]
	if run == nil {
		return nil, ErrNotFound
	}
	if run.State == StateCompleted || run.State == StateFailed || run.State == StateCancelled {
		return cloneRun(run)
	}
	if run.CancelRequested {
		return cloneRun(run)
	}
	run.CancelRequested = true
	run.Error = "cancelled by " + actor
	if run.Owner == "" || !run.LeaseExpiresAt.After(now) {
		run.State, run.Phase = StateCancelled, "cancelled"
		run.Version++
	}
	run.Revision++
	run.Sequence++
	m.events = append(m.events, Event{RunID: id, Sequence: run.Sequence, Type: "cancel_requested", Data: map[string]any{"actor": actor}, CreatedAt: now})
	return cloneRun(run)
}

func chargeDuration(run *Run, now time.Time) {
	if run.ActiveStartedAt.IsZero() {
		return
	}
	if run.Loop == nil {
		run.Loop = &LoopState{}
	}
	if run.LeaseExpiresAt.Before(now) {
		now = run.LeaseExpiresAt
	}
	run.Loop.ActiveDuration += max(time.Duration(0), now.Sub(run.ActiveStartedAt))
}
func (m *memoryStore) Append(_ context.Context, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}
func (m *memoryStore) Events(_ context.Context, _ string, _ int64, _ int) ([]Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Event(nil), m.events...), nil
}

func TestWriteConfirmationUsesPersistedCandidate(t *testing.T) {
	store := &memoryStore{}
	engine := New(store, func(context.Context, *Run) (*Candidate, error) {
		return &Candidate{Capability: "records.write", Write: true, Input: map[string]any{"record_id": "server-record"}}, nil
	}, func(_ context.Context, _ *Run, candidate *Candidate) (any, error) {
		return candidate.Input["record_id"], nil
	})
	run := NewRun("tenant", "app", "user", "update record", nil)
	if err := engine.Start(context.Background(), run); err != nil && err != ErrNotConfirmable {
		t.Fatal(err)
	}
	saved, err := store.Load(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != StateWaiting || saved.Candidate == nil {
		t.Fatalf("state=%s candidate=%v", saved.State, saved.Candidate)
	}
	candidateVersion := saved.Candidate.Version
	if _, err := engine.Confirm(context.Background(), run.ID, candidateVersion-1, "approve", "user"); err != ErrStaleVersion {
		t.Fatalf("stale confirmation error=%v", err)
	}
	approved, err := engine.Confirm(context.Background(), run.ID, candidateVersion, "approve", "user")
	if err != nil {
		t.Fatal(err)
	}
	if approved.State != StateCompleted || approved.Result != "server-record" {
		t.Fatalf("state=%s result=%v", approved.State, approved.Result)
	}
	if approved.Candidate.Input["record_id"] != "server-record" {
		t.Fatal("candidate was replaced by caller data")
	}
}

func TestStoreLeasePreventsConcurrentAdvanceAndPreservesCancellation(t *testing.T) {
	store := &memoryStore{}
	run := NewRun("tenant", "app", "user", "lease", nil)
	if err := store.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	first, err := store.Acquire(context.Background(), run.ID, "first", time.Now(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Acquire(context.Background(), run.ID, "second", time.Now(), time.Now().Add(time.Minute)); err != ErrBusy {
		t.Fatalf("second executor acquired active lease: %v", err)
	}
	cancelled, err := store.RequestCancel(context.Background(), run.ID, "user", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled.CancelRequested || cancelled.State == StateCancelled {
		t.Fatalf("active run was finalized instead of receiving a cancellation request: %+v", cancelled)
	}
	// The cancellation update advances the storage revision. An executor must
	// reload that control flag before committing its completed receipt.
	first, err = store.Load(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	first.State, first.Phase, first.Version = StateCompleted, "complete", first.Version+1
	if err := store.Commit(context.Background(), first, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.Release(context.Background(), run.ID, "first"); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Load(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !saved.CancelRequested || saved.State != StateCompleted {
		t.Fatalf("cancellation or completed state was lost: %+v", saved)
	}
}

func TestWaitingExecutionRemainsResumable(t *testing.T) {
	store := &memoryStore{}
	engine := New(store, func(context.Context, *Run) (*Candidate, error) {
		return &Candidate{Capability: "needs_input", Write: true}, nil
	}, func(context.Context, *Run, *Candidate) (any, error) { return nil, ErrWaiting })
	run := NewRun("tenant", "app", "user", "wait", nil)
	if err := engine.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Load(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != StateWaiting {
		t.Fatalf("state=%s", saved.State)
	}
	if _, err := engine.Confirm(context.Background(), run.ID, saved.Candidate.Version, "approve", "user"); err != ErrWaiting {
		t.Fatalf("waiting execution error=%v", err)
	}
	saved, err = store.Load(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != StateWaiting {
		t.Fatalf("state=%s", saved.State)
	}
}
