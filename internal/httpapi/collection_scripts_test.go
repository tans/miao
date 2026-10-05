package httpapi

import (
	"context"
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

func TestNormalizeCollectionScriptScheduleBounds(t *testing.T) {
	got, msg := normalizeCollectionScriptSchedule(map[string]any{"type": "daily", "timezone": "UTC", "time": "09:30"})
	if msg != "" || got["type"] != "daily" || got["timezone"] != "UTC" {
		t.Fatalf("valid schedule rejected: %#v %s", got, msg)
	}
	for _, raw := range []map[string]any{
		{"type": "daily", "timezone": "Not/AZone", "time": "09:30"},
		{"type": "daily", "timezone": "UTC", "time": "9:30"},
		{"type": "weekly", "timezone": "UTC", "time": "09:30", "weekdays": []any{7}},
		{"type": "once", "timezone": "UTC", "at": "2026-10-04T09:30:00"},
	} {
		if _, msg := normalizeCollectionScriptSchedule(raw); msg == "" {
			t.Fatalf("invalid schedule accepted: %#v", raw)
		}
	}
}

func TestCollectionScriptFiltersConversionsAndDedup(t *testing.T) {
	row := map[string]any{"id": "item-1", "name": "Alpha", "amount": "12.5", "active": "yes"}
	if !collectionScriptFilterMatch(row, []any{map[string]any{"field": "name", "operator": "contains", "value": "alp"}}) {
		t.Fatal("case-insensitive contains filter did not match")
	}
	if collectionScriptFilterMatch(row, []any{map[string]any{"field": "amount", "operator": "lt", "value": 10}}) {
		t.Fatal("numeric filter unexpectedly matched")
	}
	if value, ok := collectionScriptConvert(row["amount"], "number"); !ok || value != 12.5 {
		t.Fatalf("number conversion failed: %#v %v", value, ok)
	}
	if value, ok := collectionScriptConvert(row["active"], "bool"); !ok || value != true {
		t.Fatalf("bool conversion failed: %#v %v", value, ok)
	}
	first := collectionScriptDedupKey(row, []string{"id", "name"})
	second := collectionScriptDedupKey(map[string]any{"id": "item-1", "name": "Alpha", "amount": "changed"}, []string{"id", "name"})
	if first == "" || first != second {
		t.Fatalf("dedup key was not stable: %q %q", first, second)
	}
}

func TestCollectionScriptShouldSkipOnlyIdenticalSource(t *testing.T) {
	row := map[string]any{"id": "item-1", "name": "Alpha"}
	item := map[string]any{"status": "written", "source": row}
	if !collectionScriptShouldSkip(item, row) {
		t.Fatal("identical source was not skipped")
	}
	changed := map[string]any{"id": "item-1", "name": "Beta"}
	if collectionScriptShouldSkip(item, changed) {
		t.Fatal("changed source was incorrectly skipped")
	}
}

func TestCollectionScriptPathBounds(t *testing.T) {
	for _, path := range []string{"/api/items", "/v1/detail?id=1"} {
		if !collectionScriptPath(path) {
			t.Fatalf("safe path rejected: %s", path)
		}
	}
	for _, path := range []string{"", "api/items", "/api/../private", "/api/items#fragment"} {
		if collectionScriptPath(path) {
			t.Fatalf("unsafe path accepted: %s", path)
		}
	}
}

func TestCollectionScriptPreviewDoesNotPersistRun(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	appID := f.base[len("/api/apps/"):]
	script, err := f.api.PB.Create(ctx, "collection_scripts", map[string]any{"tenant_id": f.tenantID, "app_id": appID, "created_by": f.userID, "name": "Preview", "revision": 1, "definition": map[string]any{"source": map[string]any{"connector_id": "missing", "path": "/api/items"}, "schedule": map[string]any{"type": "manual"}}, "status": "draft"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.api.executeCollectionScript(ctx, script, "preview", "manual:preview-test"); err != nil {
		t.Fatal(err)
	}
	if rows, err := f.api.PB.ListAll(ctx, "collection_script_runs", "script_id = "+pbFilterString(stringValue(script["id"])), ""); err != nil || len(rows) != 0 {
		t.Fatalf("preview persisted execution runs: %d %v", len(rows), err)
	}
}
func TestCollectionScriptNotificationFailureRetriesFromPersistedState(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	appID := f.base[len("/api/apps/"):]
	script, err := f.api.PB.Create(ctx, "collection_scripts", map[string]any{"tenant_id": f.tenantID, "app_id": appID, "created_by": f.userID, "name": "Discovery", "revision": 1, "definition": map[string]any{"schedule": map[string]any{"type": "manual"}}, "status": "enabled"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := f.api.PB.Create(ctx, "collection_script_runs", map[string]any{"tenant_id": f.tenantID, "app_id": appID, "script_id": script["id"], "version": 1, "created_by": f.userID, "event_key": "manual:test", "mode": "live", "status": "completed", "snapshot": map[string]any{"version": 1}})
	if err != nil {
		t.Fatal(err)
	}
	item, err := f.api.PB.Create(ctx, "collection_script_items", map[string]any{"tenant_id": f.tenantID, "app_id": appID, "script_id": script["id"], "dedup_key": "one", "status": "written", "last_run_id": run["id"]})
	if err != nil {
		t.Fatal(err)
	}
	hook := f.runtime.App.OnRecordCreate("automation_notifications").BindFunc(func(e *core.RecordEvent) error { return errors.New("injected inbox failure") })
	_, err = f.api.createCollectionScriptNotification(ctx, script, run, item, f.userID)
	f.runtime.App.OnRecordCreate("automation_notifications").Unbind(hook)
	if err == nil {
		t.Fatal("inbox failure was not reported")
	}
	pending, err := f.api.PB.Find(ctx, "collection_script_notifications", "item_id = "+pbFilterString(stringValue(item["id"])))
	if err != nil || pending["status"] != "failed" {
		t.Fatalf("notification state not persisted: %#v %v", pending, err)
	}
	f.api.retryCollectionScriptNotifications(ctx)
	updated, err := f.api.PB.Get(ctx, "collection_script_notifications", stringValue(pending["id"]))
	if err != nil || updated["status"] != "delivered" {
		t.Fatalf("notification retry failed: %#v %v", updated, err)
	}
	f.api.retryCollectionScriptNotifications(ctx)
	_, total, _, err := f.api.PB.List(ctx, "automation_notifications", "rule_id = "+pbFilterString(stringValue(script["id"])), "", 1, 10)
	if err != nil || total != 1 {
		t.Fatalf("inbox delivery duplicated or missing: %d %v", total, err)
	}
}
