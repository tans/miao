package harness

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestReadCandidateCompletesWithoutWriteConfirmation(t *testing.T) {
	store := &memoryStore{}
	engine := New(store, func(context.Context, *Run) (*Candidate, error) {
		return &Candidate{Capability: "records.query", Input: map[string]any{"table": "customers"}}, nil
	}, func(_ context.Context, _ *Run, candidate *Candidate) (any, error) {
		return candidate.Input["table"], nil
	})
	run := NewRun("tenant", "app", "user", "query", nil)
	if err := engine.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateCompleted || got.Result != "customers" || got.Authority != nil {
		t.Fatalf("run=%+v", got)
	}
}

func TestExpiredWriteAuthorityCannotExecute(t *testing.T) {
	store := &memoryStore{}
	executed := false
	engine := New(store, func(context.Context, *Run) (*Candidate, error) {
		return &Candidate{Capability: "business_actions.execute", Write: true}, nil
	}, func(context.Context, *Run, *Candidate) (any, error) {
		executed = true
		return nil, nil
	})
	run := NewRun("tenant", "app", "user", "execute", nil)
	if err := engine.Start(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	waiting, err := store.Load(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	waiting.Authority = &Authority{Version: waiting.Candidate.Version, TenantID: waiting.TenantID, AppID: waiting.AppID, UserID: waiting.UserID, Capability: waiting.Candidate.Capability, ExpiresAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)}
	waiting.State = StateQueued
	if err := store.Save(context.Background(), waiting); err != nil {
		t.Fatal(err)
	}
	err = engine.Resume(context.Background(), run.ID)
	if err == nil || !strings.Contains(err.Error(), "expired") || executed {
		t.Fatalf("expired authority executed=%v err=%v", executed, err)
	}
}
