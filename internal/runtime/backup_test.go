package runtime

import (
	"archive/zip"
	"context"
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
	run := create("miao_runs", map[string]any{"tenant_id": tenant["id"], "app_id": app["id"], "created_by": user["id"], "task_id": task["id"], "event_key": "event-1", "snapshot": map[string]any{"name": "检查"}, "status": "running", "checkpoint": map[string]any{"step": 1}})
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
	}{{"apps", app["id"], "name", "客户"}, {"miao_tasks", task["id"], "status", "paused"}, {"miao_runs", run["id"], "status", "cancelled"}, {"miao_run_attempts", attempt["id"], "status", "interrupted"}, {"miao_actions", action["id"], "status", "done"}, {"automation_rules", rule["id"], "enabled", false}} {
		row, err := r.Store.Get(ctx, check.collection, check.id.(string))
		if err != nil || row[check.field] != check.want {
			t.Fatalf("%s: %v %v", check.collection, row, err)
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
