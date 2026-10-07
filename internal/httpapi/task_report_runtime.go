package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/tans/miao/internal/harness"
)

type taskReportRuntime struct {
	s       *Server
	leaseID string
}

func (r taskReportRuntime) active(ctx context.Context, run *harness.Run) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.s.assertWorkerLease(ctx, r.leaseID); err != nil {
		return nil, err
	}
	row, err := r.s.PB.Get(ctx, "miao_runs", run.ID)
	if err != nil {
		return nil, err
	}
	if boolValue(row["cancel_requested"]) {
		return nil, harness.ErrCancelled
	}
	if row["tenant_id"] != run.TenantID || row["app_id"] != run.AppID || row["created_by"] != run.UserID || !equalJSON(row["snapshot"], run.Context) {
		return nil, businessError(409, "任务授权快照已变化")
	}
	task, err := r.s.PB.Get(ctx, "miao_tasks", stringValue(row["task_id"]))
	if err != nil {
		return nil, err
	}
	if task["created_by"] != run.UserID || task["status"] == "archived" {
		return nil, businessError(403, "任务已归档或负责人已变化")
	}
	if _, err := taskAuthority(ctx, r.s, row); err != nil {
		return nil, err
	}
	return row, nil
}

func (r taskReportRuntime) Observe(ctx context.Context, run *harness.Run) (harness.Observation, error) {
	_, err := r.active(ctx, run)
	return harness.Observation{Values: map[string]any{"task_snapshot": run.Context}}, err
}

func (r taskReportRuntime) Enumerate(_ context.Context, run *harness.Run, _ harness.Observation) ([]harness.CandidateOption, error) {
	return []harness.CandidateOption{{ID: "report:" + run.ID, Capability: "task.report", Description: "读取已授权字段的固定数据快照"}}, nil
}

func (r taskReportRuntime) Decide(_ context.Context, _ *harness.Run, _ harness.Observation, options []harness.CandidateOption) (harness.Decision, error) {
	if len(options) != 1 || options[0].Capability != "task.report" {
		return harness.Decision{}, harness.ErrCapability
	}
	return harness.Decision{CandidateID: options[0].ID}, nil
}

func (r taskReportRuntime) Validate(ctx context.Context, run *harness.Run, _ harness.Observation, candidate *harness.Candidate) error {
	if candidate.Capability != "task.report" || candidate.Write || asMap(run.Context)["execution"] != "report" {
		return harness.ErrCapability
	}
	_, err := r.active(ctx, run)
	return err
}

func (r taskReportRuntime) Execute(ctx context.Context, run *harness.Run, _ *harness.Candidate) (harness.StepResult, error) {
	row, err := r.active(ctx, run)
	if err != nil {
		return harness.StepResult{Outcome: harness.OutcomeFailed}, err
	}
	output, err := r.s.runFixedReport(ctx, row, func() error { _, err := r.active(ctx, run); return err })
	if err != nil {
		return harness.StepResult{Outcome: harness.OutcomeFailed}, err
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		return harness.StepResult{Outcome: harness.OutcomeFailed}, err
	}
	return harness.StepResult{Outcome: harness.OutcomeContinue, Value: output, Receipt: map[string]any{"run_id": run.ID, "report": report}}, nil
}

func (r taskReportRuntime) CheckComplete(_ context.Context, run *harness.Run, _ harness.Observation) (harness.Completion, error) {
	if run.Loop == nil || len(run.Loop.Steps) == 0 {
		return harness.Completion{}, nil
	}
	step := run.Loop.Steps[len(run.Loop.Steps)-1]
	receipt := asMap(step.Result.Receipt)
	if step.CompletedAt.IsZero() || step.Candidate.Capability != "task.report" || step.Result.Outcome != harness.OutcomeContinue || receipt["run_id"] != run.ID || asMap(receipt["report"])["tables"] == nil {
		return harness.Completion{}, nil
	}
	return harness.Completion{Satisfied: true, Evidence: []any{step.Result.Receipt}}, nil
}

// A report has no business side effects. Only an unfinished read may be repeated;
// completed report receipts remain in the shared loop and are never re-executed.
func (r taskReportRuntime) Reconcile(ctx context.Context, run *harness.Run, step harness.Step) (harness.StepResult, error) {
	if step.Candidate.Write || step.Candidate.Capability != "task.report" {
		return harness.StepResult{Outcome: harness.OutcomeUnknown}, harness.ErrUnknown
	}
	return r.Execute(ctx, run, &step.Candidate)
}

func (s *Server) executeReportRun(ctx context.Context, initial map[string]any, leaseID string) {
	id := stringValue(initial["id"])
	row, err := s.PB.Get(ctx, "miao_runs", id)
	if err != nil || row["status"] != "queued" || boolValue(row["cancel_requested"]) {
		return
	}
	attemptNo := intValue(row["attempts"]) + 1
	attempt, err := s.PB.Create(ctx, "miao_run_attempts", map[string]any{"tenant_id": row["tenant_id"], "app_id": row["app_id"], "run_id": id, "sequence": attemptNo, "status": "running", "started_at": nowISO()})
	if err != nil {
		s.Logger.Error("report attempt persistence failed", "run_id", id, "error", err)
		return
	}
	row, err = s.PB.Update(ctx, "miao_runs", id, map[string]any{"status": "running", "attempts": attemptNo, "started_at": defaultString(stringValue(row["started_at"]), nowISO()), "finished_at": "", "error": ""})
	if err == nil {
		runtime := taskReportRuntime{s: s, leaseID: leaseID}
		limits := asMap(asMap(row["snapshot"])["limits"])
		engine := harness.NewRuntime(taskHarnessStore{s}, runtime, harness.Limits{Timeout: time.Duration(max(1, intValue(limits["timeout_seconds"]))) * time.Second})
		if stringValue(row["harness_state"]) == "" {
			err = engine.Start(ctx, taskHarnessRun(row))
		} else {
			err = engine.Resume(ctx, id)
		}
	}
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	latest, loadErr := s.PB.Get(finalCtx, "miao_runs", id)
	if loadErr != nil {
		s.Logger.Error("report finalization load failed", "run_id", id, "error", loadErr)
		return
	}
	status, message := "failed", "报告没有共享内核完成回执"
	if err != nil {
		message = err.Error()
	}
	if latest["harness_state"] == string(harness.StateCompleted) {
		status, message = "completed", ""
	} else if boolValue(latest["cancel_requested"]) || errors.Is(err, harness.ErrCancelled) {
		status = "cancelled"
	} else if ctx.Err() != nil && latest["harness_state"] == string(harness.StateQueued) {
		status = "queued"
	}
	delivery := taskDeliveryFor(asMap(latest["snapshot"]), "pending")
	finished := nowISO()
	if status == "queued" {
		finished = ""
	}
	if _, err := s.PB.Update(finalCtx, "miao_runs", id, map[string]any{"status": status, "output": clip(stringValue(latest["harness_result"]), 30000), "error": clip(message, 1000), "finished_at": finished, "delivery_status": delivery}); err != nil {
		s.Logger.Error("report finalization failed", "run_id", id, "error", err)
		return
	}
	if _, err := s.PB.Update(finalCtx, "miao_run_attempts", stringValue(attempt["id"]), map[string]any{"status": status, "output": clip(stringValue(latest["harness_result"]), 30000), "error": clip(message, 1000), "finished_at": nowISO(), "model_requests": 0}); err != nil {
		s.Logger.Error("report attempt finalization failed", "run_id", id, "error", err)
	}
}
