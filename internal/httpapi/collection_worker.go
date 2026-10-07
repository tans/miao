package httpapi

import (
	"context"
	"time"
)

type collectionLeaseKey struct{}

// Collection work shares the existing worker slot and durable worker lease.
// HTTP requests and the scheduler only enqueue; neither owns execution lifetime.
func (s *Server) startQueuedCollectionScript(ctx context.Context, leaseID string) bool {
	rows, _, _, err := s.PB.List(ctx, "collection_script_runs", "status = \"queued\"", "created,id", 1, 1)
	if err != nil || len(rows) == 0 {
		return false
	}
	run := rows[0]
	runCtx, cancel := context.WithTimeout(context.WithValue(ctx, collectionLeaseKey{}, leaseID), 10*time.Minute)
	s.workerMu.Lock()
	if s.activeRun != "" {
		s.workerMu.Unlock()
		cancel()
		return true
	}
	s.activeRun, s.activeCancel = "collection:"+stringValue(run["id"]), cancel
	s.workerMu.Unlock()
	s.workerWG.Add(1)
	go func() {
		defer s.workerWG.Done()
		defer cancel()
		defer func() {
			s.workerMu.Lock()
			s.activeRun, s.activeCancel = "", nil
			s.workerMu.Unlock()
		}()
		if err := s.assertWorkerLease(runCtx, leaseID); err != nil {
			s.Logger.Error("collection worker lease lost", "error", err)
			return
		}
		script, err := s.PB.Get(runCtx, "collection_scripts", stringValue(run["script_id"]))
		if err == nil && (intValue(script["revision"]) != intValue(run["version"]) || script["created_by"] != run["created_by"] || script["app_id"] != run["app_id"] || script["tenant_id"] != run["tenant_id"]) {
			err = businessError(409, "采集运行版本或负责人已变化，原快照不再执行")
		}
		if err == nil {
			err = s.collectionScriptExecutionGuard(runCtx, script)
		}
		if err != nil {
			_, finishErr := s.finishCollectionScriptRun(runCtx, run, "failed", nil, []string{err.Error()})
			if finishErr != nil {
				s.Logger.Error("collection finalization failed", "run_id", run["id"], "error", finishErr)
			}
			return
		}
		run, err = s.PB.Update(runCtx, "collection_script_runs", stringValue(run["id"]), map[string]any{"status": "running", "started_at": nowISO()})
		if err != nil {
			s.Logger.Error("collection claim failed", "error", err)
			return
		}
		result, runErr := s.collectCollectionScript(runCtx, script, run, "live")
		status, failures := "completed", []string{}
		if intValue(asMap(asMap(result)["counts"])["errors"]) > 0 {
			status = "partial"
		}
		if runErr != nil {
			status, failures = "failed", []string{runErr.Error()}
			if intValue(asMap(asMap(result)["counts"])["written"]) > 0 {
				status = "partial"
			}
		}
		if _, err := s.finishCollectionScriptRun(runCtx, run, status, result, failures); err != nil {
			s.Logger.Error("collection finalization failed", "run_id", run["id"], "error", err)
		}
	}()
	return true
}
