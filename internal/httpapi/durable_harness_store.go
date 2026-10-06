package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/pocketbase"
)

func commitHarnessRecord(ctx context.Context, pb *pocketbase.Client, collection string, run *harness.Run, event *harness.Event, prefix string) error {
	if _, err := json.Marshal(run); err != nil {
		return err
	}
	if event != nil && (event.RunID != run.ID || event.Sequence != run.Sequence) {
		return harness.ErrConflict
	}
	next := *run
	next.Revision++
	err := pb.Transaction(ctx, func(tx *pocketbase.Client) error {
		row, err := tx.Get(ctx, collection, run.ID)
		if err != nil {
			return harnessLoadError(err)
		}
		if err := validateHarnessRecord(row, prefix); err != nil {
			return err
		}
		current := decodeHarnessRecord(row, prefix)
		if current.Revision != run.Revision {
			return harness.ErrConflict
		}
		if current.Owner != run.Owner || current.Owner == "" || !current.LeaseExpiresAt.After(time.Now()) {
			return harness.ErrLeaseLost
		}
		if event != nil && event.Sequence != current.Sequence+1 {
			return harness.ErrConflict
		}
		next.CancelRequested = next.CancelRequested || current.CancelRequested
		updated, err := tx.UpdateWhere(ctx, collection, map[string]any{field(prefix, "storage_revision"): next.Revision}, dbx.HashExp{"id": run.ID, field(prefix, "storage_revision"): run.Revision, field(prefix, "lease_owner"): run.Owner})
		if err != nil {
			return err
		}
		if !updated {
			return harness.ErrConflict
		}
		// Validate JSON size and field types through PocketBase after the CAS.
		// Both writes and the event roll back if any validation/storage fails.
		if _, err := tx.Update(ctx, collection, run.ID, harnessRunFields(&next, prefix)); err != nil {
			return err
		}
		if event == nil {
			return nil
		}
		data, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}
		_, err = tx.Create(ctx, "miao_harness_events", map[string]any{"tenant_id": run.TenantID, "app_id": run.AppID, "user_id": run.UserID, "run_id": harnessEventKey(prefix, run.ID), "sequence": event.Sequence, "event_type": event.Type, "data": json.RawMessage(data)})
		return err
	})
	if err == nil {
		run.Revision = next.Revision
		run.CancelRequested = next.CancelRequested
	}
	return err
}

func acquireHarnessRecord(ctx context.Context, pb *pocketbase.Client, collection, id, owner string, now, expires time.Time, prefix string) (*harness.Run, error) {
	if owner == "" || len(owner) > 80 || !expires.After(now) {
		return nil, harness.ErrLeaseLost
	}
	var result *harness.Run
	err := pb.Transaction(ctx, func(tx *pocketbase.Client) error {
		row, err := tx.Get(ctx, collection, id)
		if err != nil {
			return harnessLoadError(err)
		}
		if err := validateHarnessRecord(row, prefix); err != nil {
			return err
		}
		current := decodeHarnessRecord(row, prefix)
		if current.Owner != "" && current.LeaseExpiresAt.After(now) {
			return harness.ErrBusy
		}
		chargeHarnessDuration(current, now)
		updated, err := tx.UpdateWhere(ctx, collection, map[string]any{field(prefix, "storage_revision"): current.Revision + 1, field(prefix, "lease_owner"): owner, field(prefix, "lease_expires_at"): formatLease(expires)}, dbx.HashExp{"id": id, field(prefix, "storage_revision"): current.Revision, field(prefix, "lease_owner"): current.Owner})
		if err != nil {
			return err
		}
		if !updated {
			return harness.ErrConflict
		}
		current.Owner, current.LeaseExpiresAt, current.Revision = owner, expires, current.Revision+1
		current.ActiveStartedAt = now
		loop, err := json.Marshal(current.Loop)
		if err != nil {
			return err
		}
		if _, err := tx.Update(ctx, collection, id, map[string]any{field(prefix, "loop"): json.RawMessage(loop), field(prefix, "active_started_at"): formatLease(now)}); err != nil {
			return err
		}
		result = current
		return nil
	})
	return result, err
}

func releaseHarnessRecord(ctx context.Context, pb *pocketbase.Client, collection, id, owner, prefix string) error {
	return pb.Transaction(ctx, func(tx *pocketbase.Client) error {
		row, err := tx.Get(ctx, collection, id)
		if err != nil {
			return harnessLoadError(err)
		}
		current := decodeHarnessRecord(row, prefix)
		if current.Owner != owner {
			return harness.ErrLeaseLost
		}
		chargeHarnessDuration(current, time.Now())
		updated, err := tx.UpdateWhere(ctx, collection, map[string]any{field(prefix, "storage_revision"): current.Revision + 1, field(prefix, "lease_owner"): "", field(prefix, "lease_expires_at"): ""}, dbx.HashExp{"id": id, field(prefix, "storage_revision"): current.Revision, field(prefix, "lease_owner"): owner})
		if err == nil && !updated {
			err = harness.ErrConflict
		}
		if err != nil {
			return err
		}
		loop, err := json.Marshal(current.Loop)
		if err != nil {
			return err
		}
		_, err = tx.Update(ctx, collection, id, map[string]any{field(prefix, "loop"): json.RawMessage(loop), field(prefix, "active_started_at"): ""})
		return err
	})
}

func chargeHarnessDuration(run *harness.Run, now time.Time) {
	if run.ActiveStartedAt.IsZero() {
		return
	}
	if run.Loop == nil {
		run.Loop = &harness.LoopState{}
	}
	if run.LeaseExpiresAt.Before(now) {
		now = run.LeaseExpiresAt
	}
	run.Loop.ActiveDuration += max(time.Duration(0), now.Sub(run.ActiveStartedAt))
}

func field(prefix, name string) string { return prefix + name }

func decodeHarnessRecord(row map[string]any, prefix string) *harness.Run {
	if prefix == "" {
		return harnessRun(row)
	}
	return taskHarnessRun(row)
}

func validateHarnessRecord(row map[string]any, prefix string) error {
	raw := row[field(prefix, "loop")]
	if raw != nil && raw != "" {
		data, err := json.Marshal(raw)
		if err != nil {
			return err
		}
		var loop *harness.LoopState
		if err := json.Unmarshal(data, &loop); err != nil {
			return fmt.Errorf("invalid persisted harness loop: %w", err)
		}
		if loop != nil && (loop.Decisions < 0 || loop.ModelRequests < 0 || loop.NoProgress < 0 || loop.ActiveDuration < 0 || loop.ObservedSteps < 0 || loop.ObservedSteps > len(loop.Steps)) {
			return fmt.Errorf("invalid persisted harness counters")
		}
	}
	if stringValue(row[field(prefix, "lease_owner")]) != "" && parseTime(row[field(prefix, "lease_expires_at")]).IsZero() {
		return harness.ErrLeaseLost
	}
	return nil
}

func requestHarnessCancel(ctx context.Context, pb *pocketbase.Client, collection, id, actor string, now time.Time, prefix string) (*harness.Run, error) {
	var result *harness.Run
	err := pb.Transaction(ctx, func(tx *pocketbase.Client) error {
		row, err := tx.Get(ctx, collection, id)
		if err != nil {
			return harnessLoadError(err)
		}
		run := decodeHarnessRecord(row, prefix)
		if run.State == harness.StateCompleted || run.State == harness.StateFailed || run.State == harness.StateCancelled || run.CancelRequested {
			result = run
			return nil
		}
		oldRevision := run.Revision
		run.CancelRequested = true
		run.Error = "cancelled by " + actor
		patch := map[string]any{"cancel_requested": true, field(prefix, "storage_revision"): oldRevision + 1, field(prefix, "sequence"): run.Sequence + 1}
		if run.Owner == "" || !run.LeaseExpiresAt.After(now) {
			run.State, run.Phase = harness.StateCancelled, "cancelled"
			run.Version++
			patch[field(prefix, "state")], patch[field(prefix, "phase")], patch[field(prefix, "version")], patch["error"] = run.State, run.Phase, run.Version, run.Error
		}
		run.Revision++
		run.Sequence++
		updated, err := tx.UpdateWhere(ctx, collection, patch, dbx.HashExp{"id": id, field(prefix, "storage_revision"): oldRevision})
		if err != nil {
			return err
		}
		if !updated {
			return harness.ErrConflict
		}
		if _, err := tx.Create(ctx, "miao_harness_events", map[string]any{"tenant_id": run.TenantID, "app_id": run.AppID, "user_id": run.UserID, "run_id": harnessEventKey(prefix, run.ID), "sequence": run.Sequence, "event_type": "cancel_requested", "data": map[string]any{"actor": actor}}); err != nil {
			return err
		}
		result = run
		return nil
	})
	return result, err
}

func harnessEventKey(prefix, id string) string {
	if prefix == "harness_" {
		return "task:" + id
	}
	return id
}

func harnessLoadError(err error) error {
	if isMissing(err) {
		return harness.ErrNotFound
	}
	return err
}
