package httpapi

import (
	"context"
	"runtime/debug"
	"time"
)

func (s *Server) runQueuedHarness(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			s.Logger.Error("background harness worker panicked", "panic", r, "stack", string(debug.Stack()))
		}
	}()
	rows, _, _, err := s.PB.List(ctx, "miao_harness_runs", "state = \"queued\" || state = \"observing\" || state = \"enumerating\" || state = \"deciding\" || state = \"validating\" || state = \"executing\" || state = \"recording\"", "created", 1, 10)
	if err != nil {
		return
	}
	for _, row := range rows {
		run := harnessRun(row)
		runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		_ = s.harnessEngine().Resume(runCtx, run.ID)
		cancel()
	}
}
