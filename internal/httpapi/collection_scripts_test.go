package httpapi

import (
	"testing"
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
