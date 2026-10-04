package harness

import (
	"context"
	"testing"
)

type memoryStore struct { runs map[string]*Run; events []Event }
func (m *memoryStore) Create(_ context.Context, run *Run) error { if m.runs == nil { m.runs = map[string]*Run{} }; copy := *run; m.runs[run.ID] = &copy; return nil }
func (m *memoryStore) Load(_ context.Context, id string) (*Run, error) { run := m.runs[id]; if run == nil { return nil, ErrNotFound }; copy := *run; return &copy, nil }
func (m *memoryStore) Save(_ context.Context, run *Run) error { copy := *run; m.runs[run.ID] = &copy; return nil }
func (m *memoryStore) Append(_ context.Context, event Event) error { m.events = append(m.events, event); return nil }
func (m *memoryStore) Events(_ context.Context, _ string, _ int64, _ int) ([]Event, error) { return m.events, nil }

func TestWriteConfirmationUsesPersistedCandidate(t *testing.T) {
	store := &memoryStore{}
	engine := New(store, func(context.Context, *Run) (*Candidate, error) {
		return &Candidate{Capability: "records.write", Write: true, Input: map[string]any{"record_id": "server-record"}}, nil
	}, func(_ context.Context, _ *Run, candidate *Candidate) (any, error) {
		return candidate.Input["record_id"], nil
	})
	run := NewRun("tenant", "app", "user", "update record", nil)
	if err := engine.Start(context.Background(), run); err != nil && err != ErrNotConfirmable { t.Fatal(err) }
	saved, err := store.Load(context.Background(), run.ID)
	if err != nil { t.Fatal(err) }
	if saved.State != StateWaiting || saved.Candidate == nil { t.Fatalf("state=%s candidate=%v", saved.State, saved.Candidate) }
	candidateVersion := saved.Candidate.Version
	if _, err := engine.Confirm(context.Background(), run.ID, candidateVersion-1, "approve", "user"); err != ErrStaleVersion { t.Fatalf("stale confirmation error=%v", err) }
	approved, err := engine.Confirm(context.Background(), run.ID, candidateVersion, "approve", "user")
	if err != nil { t.Fatal(err) }
	if approved.State != StateCompleted || approved.Result != "server-record" { t.Fatalf("state=%s result=%v", approved.State, approved.Result) }
	if approved.Candidate.Input["record_id"] != "server-record" { t.Fatal("candidate was replaced by caller data") }
}
