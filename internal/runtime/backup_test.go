package runtime

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	store "github.com/tans/miao/internal/pocketbase"
)

func TestDataLockExcludesSecondProcess(t *testing.T) {
	root := t.TempDir()
	r, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := New(root); err == nil {
		second.Close()
		t.Fatal("two runtimes opened one data directory")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
}
func TestBackupRestoreIsolatesHistoricalWork(t *testing.T) {
	t.Setenv("POCKETBASE_SUPERUSER_EMAIL", "")
	t.Setenv("POCKETBASE_SUPERUSER_PASSWORD", "")
	t.Setenv("MIAO_SETTINGS_ENCRYPTION_KEY", "")
	ctx := context.Background()
	root := t.TempDir()
	r, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r != nil {
			r.Close()
		}
	}()
	create := func(collection string, body map[string]any) map[string]any {
		t.Helper()
		row, err := r.Store.Create(ctx, collection, body)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	user := create("users", map[string]any{"name": "Owner", "email": "backup@example.invalid", "password": "Smoke-Password-2026", "passwordConfirm": "Smoke-Password-2026"})
	tenant := create("tenants", map[string]any{"owner_id": user["id"], "name": "Backup", "slug": "backup"})
	app := create("apps", map[string]any{"tenant_id": tenant["id"], "name": "客户"})
	task := create("miao_tasks", map[string]any{"tenant_id": tenant["id"], "app_id": app["id"], "created_by": user["id"], "name": "检查", "status": "enabled", "revision": 1, "definition": map[string]any{"goal": "检查"}})
	loop := map[string]any{"model_requests": 2, "steps": []any{map[string]any{"id": "done", "result": map[string]any{"outcome": "continue", "receipt": "kept"}}}}
	run := create("miao_runs", map[string]any{"tenant_id": tenant["id"], "app_id": app["id"], "created_by": user["id"], "task_id": task["id"], "event_key": "event-1", "snapshot": map[string]any{"name": "检查"}, "status": "running", "checkpoint": map[string]any{"step": 1}, "harness_loop": loop, "harness_state": "executing", "harness_lease_owner": "old-task", "harness_lease_expires_at": "2099-01-01T00:00:00Z", "harness_active_started_at": "2026-10-06T00:00:00Z"})
	var interactive []map[string]any
	for _, state := range []string{"queued", "observing", "enumerating", "deciding", "validating", "executing", "recording", "waiting_confirmation", "unknown", "completed"} {
		interactive = append(interactive, create("miao_harness_runs", map[string]any{"tenant_id": tenant["id"], "app_id": app["id"], "user_id": user["id"], "prompt": "resume", "state": state, "phase": "confirmed", "loop": loop, "sequence": 1, "version": 3, "authority": map[string]any{"confirmed_by": user["id"]}, "lease_owner": "old-interactive", "lease_expires_at": "2099-01-01T00:00:00Z", "active_started_at": "2026-10-06T00:00:00Z"}))
	}
	event := create("miao_harness_events", map[string]any{"tenant_id": tenant["id"], "app_id": app["id"], "user_id": user["id"], "run_id": interactive[0]["id"], "sequence": 1, "event_type": "receipt", "data": map[string]any{"receipt": "kept"}})
	attempt := create("miao_run_attempts", map[string]any{"tenant_id": tenant["id"], "app_id": app["id"], "run_id": run["id"], "sequence": 1, "status": "running"})
	action := create("miao_actions", map[string]any{"tenant_id": tenant["id"], "app_id": app["id"], "run_id": run["id"], "action_key": "write-1", "tool": "update_record", "input": map[string]any{"record_id": "kept"}, "status": "done", "evidence": map[string]any{"after": "kept"}})
	create("miao_runtime_locks", map[string]any{"name": "background-worker", "owner": "old", "expires_at": "2099-01-01T00:00:00Z"})
	rule := create("automation_rules", map[string]any{"tenant_id": tenant["id"], "app_id": app["id"], "created_by": user["id"], "name": "提醒", "enabled": true, "definition": map[string]any{"trigger": "record_created", "table": "customers", "recipient_id": user["id"]}})
	file, err := r.Store.UploadNew(ctx, "app_files", map[string]any{"tenant_id": tenant["id"], "app_id": app["id"], "user_id": user["id"], "name": "notes.txt"}, []store.Upload{{Name: "file", Filename: "notes.txt", Data: []byte("durable file")}})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := r.Backup(ctx, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	secondArchive, err := r.Backup(ctx, "", 30)
	if err != nil || secondArchive == archive {
		t.Fatalf("successive backups must have distinct archives: %s %v", secondArchive, err)
	}
	if _, err := Restore(ctx, root, archive); err == nil {
		t.Fatal("restore accepted an active runtime")
	}
	if _, err := r.Store.Update(ctx, "apps", app["id"].(string), map[string]any{"name": "newer"}); err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	r = nil
	previous, err := Restore(ctx, root, archive)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(previous, "data.db")); err != nil {
		t.Fatal("previous database was not retained", err)
	}
	r, err = New(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		collection string
		id         any
		field      string
		want       any
	}{{"apps", app["id"], "name", "客户"}, {"miao_tasks", task["id"], "status", "paused"}, {"miao_runs", run["id"], "status", "cancelled"}, {"miao_runs", run["id"], "harness_state", "cancelled"}, {"miao_runs", run["id"], "harness_lease_owner", ""}, {"miao_runs", run["id"], "harness_lease_expires_at", ""}, {"miao_runs", run["id"], "harness_active_started_at", ""}, {"miao_run_attempts", attempt["id"], "status", "interrupted"}, {"miao_actions", action["id"], "status", "done"}, {"miao_harness_events", event["id"], "event_type", "receipt"}, {"automation_rules", rule["id"], "enabled", false}} {
		row, err := r.Store.Get(ctx, check.collection, check.id.(string))
		if err != nil || row[check.field] != check.want {
			t.Fatalf("%s: %v %v", check.collection, row, err)
		}
	}
	checkLoop := func(row map[string]any, field string) {
		t.Helper()
		got, err := json.Marshal(row[field])
		want, wantErr := json.Marshal(loop)
		if err != nil || wantErr != nil || string(got) != string(want) {
			t.Fatalf("historical step receipts/budget changed: got=%s want=%s err=%v", got, want, err)
		}
	}
	taskRun, err := r.Store.Get(ctx, "miao_runs", run["id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	checkLoop(taskRun, "harness_loop")
	for _, original := range interactive {
		row, err := r.Store.Get(ctx, "miao_harness_runs", original["id"].(string))
		if err != nil {
			t.Fatal(err)
		}
		checkLoop(row, "loop")
		if original["state"] == "completed" {
			if row["state"] != "completed" {
				t.Fatal("historical completed run changed")
			}
			continue
		}
		if row["state"] != "cancelled" || row["cancel_requested"] != true || row["lease_owner"] != "" || row["lease_expires_at"] != "" || row["active_started_at"] != "" {
			t.Fatalf("historical %s run retained execution authority: %+v", original["state"], row)
		}
	}
	if locks, err := r.Store.ListAll(ctx, "miao_runtime_locks", "", ""); err != nil || len(locks) != 0 {
		t.Fatalf("stale leases restored: %v %v", locks, err)
	}
	data, _, _, err := r.Store.ProtectedFile(ctx, "app_files", file["id"].(string), file["file"].(string))
	if err != nil || string(data) != "durable file" {
		t.Fatalf("file lost: %s %v", data, err)
	}
}
func TestMalformedRestoreLeavesCurrentDataUntouched(t *testing.T) {
	root := t.TempDir()
	r, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(root, "pb_data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []string{"../outside", "data.db"} {
		t.Run(entry, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "miao_backup_invalid.zip")
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(file)
			part, err := writer.Create(entry)
			if err != nil {
				t.Fatal(err)
			}
			part.Write([]byte("invalid database"))
			writer.Close()
			file.Close()
			if _, err := Restore(context.Background(), root, archive); err == nil {
				t.Fatal("malformed restore accepted")
			}
			actual, err := os.ReadFile(filepath.Join(root, "pb_data", "data.db"))
			if err != nil || string(actual) != string(original) {
				t.Fatal("malformed restore changed active data", err)
			}
		})
	}
}
