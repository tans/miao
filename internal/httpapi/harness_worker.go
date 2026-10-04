package httpapi

import (
	"context"
	"time"
)

func (s *Server) runQueuedHarness(ctx context.Context) {
	rows, err := s.PB.List(ctx, "miao_harness_runs", "state = \"queued\" || state = \"observing\" || state = \"enumerating\" || state = \"deciding\" || state = \"validating\" || state = \"executing\" || state = \"recording\"", "created", 1, 10)
func (s *Server) runQueuedHarness(ctx context.Context) {
	rows, _, _, err := s.PB.List(ctx, "miao_harness_runs", "state = \"queued\" || state = \"observing\" || state = \"enumerating\" || state = \"deciding\" || state = \"validating\" || state = \"executing\" || state = \"recording\"", "created", 1, 10)
	if err != nil { return }
	for _, row := range rows {
		run := harnessRun(row)
		if run.CancelRequested { continue }
		runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		_ = s.harnessEngine().Resume(runCtx, run.ID)
		cancel()
	}
}
