// Package collector is the pure decision core for collection scripts: it
// normalizes schedules, matches filters, converts and deduplicates fetched
// rows, and projects script records for the API. It has no HTTP, PocketBase
// or Server dependency, so budget and idempotency rules are assertable
// without a database. httpapi handlers are thin adapters over this seam.
package collector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var offsetPattern = regexp.MustCompile(`(?:Z|[+-]\d{2}:\d{2})$`)
var hhmmPattern = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// Schedule returns the schedule block of a script definition.
func Schedule(definition map[string]any) map[string]any {
	return asMap(definition["schedule"])
}

// Public projects a script row for API responses.
func Public(row map[string]any) map[string]any {
	definition := asMap(row["definition"])
	return map[string]any{
		"id": row["id"], "name": row["name"], "created_by": row["created_by"], "status": row["status"],
		"pause_reason": defaultString(stringValue(row["pause_reason"]), ""),
		"revision":     row["revision"], "version": row["revision"],
		"schedule": Schedule(definition), "source": definition["source"],
		"target": definition["target"], "recipients": definition["recipients"],
		"definition": definition, "next_run_at": row["next_run_at"],
	}
}

// RunPublic projects a script-run row, hiding the snapshot blob.
func RunPublic(row map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range row {
		if k != "snapshot" {
			out[k] = v
		}
	}
	snapshot := asMap(row["snapshot"])
	out["version"] = snapshot["version"]
	out["schedule"] = snapshot["schedule"]
	out["source"] = snapshot["source"]
	out["target"] = snapshot["target"]
	out["counts"] = row["counts"]
	out["errors"] = row["errors"]
	return out
}

// NormalizeSchedule validates and canonicalizes a raw schedule block.
func NormalizeSchedule(raw any) (map[string]any, string) {
	input := asMap(raw)
	if len(input) == 0 {
		return nil, "必须提供 schedule"
	}
	typ := stringValue(input["type"])
	if !containsString([]string{"manual", "once", "daily", "weekly"}, typ) {
		return nil, "schedule.type 无效"
	}
	tz := defaultString(stringValue(input["timezone"]), "Asia/Shanghai")
	if _, err := time.LoadLocation(tz); err != nil {
		return nil, "schedule.timezone 无效"
	}
	out := map[string]any{"type": typ, "timezone": tz}
	switch typ {
	case "once":
		at := stringValue(input["at"])
		if !offsetPattern.MatchString(at) {
			return nil, "一次性 schedule.at 必须带时区"
		}
		parsed, err := time.Parse(time.RFC3339, at)
		if err != nil {
			return nil, "一次性 schedule.at 无效"
		}
		out["at"] = parsed.UTC().Format(time.RFC3339Nano)
	case "daily", "weekly":
		tm := stringValue(input["time"])
		if !hhmmPattern.MatchString(tm) {
			return nil, "schedule.time 格式为 HH:mm"
		}
		out["time"] = tm
		if typ == "weekly" {
			days := []int{}
			seen := map[int]bool{}
			for _, rawDay := range anySlice(input["weekdays"]) {
				day, ok := rawDay.(float64)
				if !ok {
					if integer, integerOK := rawDay.(int); integerOK {
						day, ok = float64(integer), true
					}
				}
				if !ok || day < 0 || day > 6 || day != float64(int(day)) {
					return nil, "schedule.weekdays 使用 0–6 的整数"
				}
				if seen[int(day)] {
					continue
				}
				seen[int(day)] = true
				days = append(days, int(day))
			}
			if len(days) == 0 {
				return nil, "weekly schedule 至少需要一个 weekdays"
			}
			out["weekdays"] = days
		}
	}
	return out, ""
}

// Path reports whether a raw path is safe to fetch.
func Path(value string) bool {
	return value != "" && strings.HasPrefix(value, "/") && !strings.Contains(value, "..") && !strings.ContainsAny(value, "#") && len(value) <= 1000
}

// Value reads a field from a row, supporting JSON pointers.
func Value(row map[string]any, field string) (any, bool) {
	if strings.HasPrefix(field, "/") {
		return jsonPointer(row, field)
	}
	value, ok := row[field]
	return value, ok
}

// FilterMatch reports whether a row satisfies every filter.
func FilterMatch(row map[string]any, filters []any) bool {
	for _, raw := range filters {
		item := asMap(raw)
		value, present := Value(row, stringValue(item["field"]))
		op := stringValue(item["operator"])
		want := item["value"]
		switch op {
		case "is_empty":
			if present && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
				return false
			}
		case "not_empty":
			if !present || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
				return false
			}
		case "eq":
			if !present || fmt.Sprint(value) != fmt.Sprint(want) {
				return false
			}
		case "neq":
			if present && fmt.Sprint(value) == fmt.Sprint(want) {
				return false
			}
		case "contains":
			if !strings.Contains(strings.ToLower(fmt.Sprint(value)), strings.ToLower(fmt.Sprint(want))) {
				return false
			}
		case "gt", "gte", "lt", "lte":
			left := floatValue(value)
			right := floatValue(want)
			if left == 0 && fmt.Sprint(value) != "0" {
				left, _ = strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
			}
			if right == 0 && fmt.Sprint(want) != "0" {
				right, _ = strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(want)), 64)
			}
			if (op == "gt" && !(left > right)) || (op == "gte" && !(left >= right)) || (op == "lt" && !(left < right)) || (op == "lte" && !(left <= right)) {
				return false
			}
		}
	}
	return true
}

// Convert coerces a raw value into the declared field type.
func Convert(value any, typ string) (any, bool) {
	if value == nil {
		return nil, true
	}
	switch typ {
	case "text", "date", "email", "url", "select":
		return fmt.Sprint(value), true
	case "number":
		if n, ok := value.(float64); ok {
			return n, true
		}
		if n, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64); err == nil {
			return n, true
		}
	case "bool":
		if b, ok := value.(bool); ok {
			return b, true
		}
		switch strings.ToLower(strings.TrimSpace(fmt.Sprint(value))) {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		}
	}
	return nil, false
}

// ShouldSkip reports whether an existing written receipt makes a row redundant.
func ShouldSkip(item, row map[string]any) bool {
	return len(item) > 0 && stringValue(item["status"]) == "written" && equalJSON(asMap(item["source"]), row)
}

// DedupKey hashes the configured dedup fields of a row.
func DedupKey(row map[string]any, fields []string) string {
	values := make([]any, 0, len(fields))
	for _, field := range fields {
		value, _ := Value(row, field)
		values = append(values, value)
	}
	encoded, _ := json.Marshal(values)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// Strings extracts the non-empty string items of a raw list.
func Strings(raw any) []string {
	values := []string{}
	for _, item := range anySlice(raw) {
		if value := stringValue(item); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func asMap(raw any) map[string]any {
	if value, ok := raw.(map[string]any); ok && value != nil {
		return value
	}
	return map[string]any{}
}
func anySlice(raw any) []any {
	switch value := raw.(type) {
	case []any:
		return value
	case []map[string]any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = item
		}
		return out
	case []string:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = item
		}
		return out
	}
	return []any{}
}
func asSliceMap(raw any) []map[string]any {
	out := []map[string]any{}
	for _, v := range anySlice(raw) {
		if item, ok := v.(map[string]any); ok {
			out = append(out, item)
		}
	}
	return out
}
func stringValue(raw any) string { value, _ := raw.(string); return value }
func intValue(raw any) int {
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	}
	return 0
}
func floatValue(raw any) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		n, _ := value.Float64()
		return n
	}
	return 0
}
func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func equalJSON(a, b any) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(aa) == string(bb)
}
func jsonPointer(value any, pointer string) (any, bool) {
	current := value
	if pointer == "" {
		return current, true
	}
	for _, raw := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		switch node := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = node[part]
			if !ok {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(node) {
				return nil, false
			}
			current = node[index]
		default:
			return nil, false
		}
	}
	return current, true
}

var _ = http.StatusOK
