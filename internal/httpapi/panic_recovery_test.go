package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// runPanics verifies runScanStage swallows a panicking stage; if it didn't,
// one bad scan would crash the whole process.
func runPanics(t *testing.T) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runScanStage let a panic escape: %v", r)
		}
	}()
	s := &Server{Logger: discardLogger()}
	s.runScanStage(context.Background(), "test", func(context.Context) {
		panic(errors.New("boom"))
	})
}

func TestScanStagePanicIsolation(t *testing.T) { runPanics(t) }

func TestScanStageNormalCall(t *testing.T) {
	called := false
	s := &Server{Logger: discardLogger()}
	s.runScanStage(context.Background(), "test", func(context.Context) { called = true })
	if !called {
		t.Fatal("stage was not called")
	}
}

// failPanicRun issues DB writes; here only verify it survives without a
// usable client (best-effort semantics) rather than panicking again.
func TestFailPanicRunNilSafe(t *testing.T) {
	s := &Server{Logger: discardLogger()}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("failPanicRun panicked: %v", r)
		}
	}()
	s.failPanicRun(context.Background(), map[string]any{"id": "x", "snapshot": "oops"})
}

func TestPanicMessageClipped(t *testing.T) {
	long := strings.Repeat("异", 2000)
	if got := clip(long, 1000); len([]rune(got)) != 1000 {
		t.Fatalf("clip length = %d", len([]rune(got)))
	}
}
