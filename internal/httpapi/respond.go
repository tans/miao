package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type identityKey struct{}

type identity struct {
	User, Tenant, Membership map[string]any
	Workspaces               []map[string]any
	Token, FreshToken        string
}

func withIdentity(ctx context.Context, who identity) context.Context {
	return context.WithValue(ctx, identityKey{}, who)
}
func who(r *http.Request) identity { id, _ := r.Context().Value(identityKey{}).(identity); return id }
func contextTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 25*time.Second)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	// Localize at the response boundary so every handler stays in the zh-CN
	// source language while clients get their negotiated language.
	if lw, ok := w.(*languageWriter); ok {
		message = localize(lw.currentLanguage(), message)
	}
	writeJSON(w, status, map[string]any{"error": message})
}
func readJSON(r *http.Request, out any) error {
	if r.Body == nil {
		return io.EOF
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, (8<<20)+1))
	return decoder.Decode(out)
}
func mapBody(r *http.Request) map[string]any {
	out := map[string]any{}
	if readJSON(r, &out) != nil || out == nil {
		return map[string]any{}
	}
	return out
}
func stringValue(raw any) string { value, _ := raw.(string); return value }
func boolValue(raw any) bool     { value, _ := raw.(bool); return value }
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
func uniqueStrings(raw any, limit int) []string {
	out, seen := []string{}, map[string]bool{}
	for _, item := range anySlice(raw) {
		if value, ok := item.(string); ok && value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}
func queryInt(r *http.Request, key string, fallback, min, max int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < min || value > max {
		return fallback
	}
	return value
}
func queryIntFrom(input map[string]any, key string, fallback, min, max int) int {
	value := intValue(input[key])
	if value < min || value > max {
		return fallback
	}
	return value
}
func pathID(r *http.Request, key string) string { return r.PathValue(key) }
func pbFilterString(value string) string        { encoded, _ := json.Marshal(value); return string(encoded) }
func listFilter(parts ...string) string {
	values := []string{}
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			values = append(values, "("+part+")")
		}
	}
	return strings.Join(values, " && ")
}
func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func clip(value string, max int) string {
	if max > 0 && len([]rune(value)) > max {
		return string([]rune(value)[:max])
	}
	return value
}
func publicUser(user map[string]any) map[string]any {
	return map[string]any{"id": user["id"], "email": user["email"], "name": user["name"], "created_at": user["created"], "language": stringValue(user["language"])}
}
func publicTenant(tenant map[string]any, role string) map[string]any {
	return map[string]any{"id": tenant["id"], "name": tenant["name"], "slug": tenant["slug"], "role": role}
}
func pageResult(items []map[string]any, page, perPage, total int) map[string]any {
	pages := 0
	if total > 0 {
		pages = (total + perPage - 1) / perPage
	}
	return map[string]any{"items": items, "page": page, "perPage": perPage, "totalItems": total, "totalPages": pages}
}
func nowISO() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func jsonMarshal(value any) ([]byte, error) { return json.Marshal(value) }
func parseTime(raw any) time.Time {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z"} {
		if value, err := time.Parse(layout, stringValue(raw)); err == nil {
			return value
		}
	}
	return time.Time{}
}
func contains(raw, target any) bool {
	for _, item := range anySlice(raw) {
		if equalJSON(item, target) {
			return true
		}
	}
	return false
}
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func findField(fields []map[string]any, name string) map[string]any {
	for _, field := range fields {
		if field["name"] == name {
			return field
		}
	}
	return nil
}
func publicRecord(row map[string]any) map[string]any {
	data := map[string]any{}
	for key, value := range row {
		if key != "id" && key != "collectionId" && key != "collectionName" && key != "created" && key != "updated" && key != "app_id" && key != "tenant_id" {
			data[key] = value
		}
	}
	return map[string]any{"id": row["id"], "created_at": row["created"], "updated_at": row["updated"], "data": data}
}
func cleanAppSlug(value string) string {
	var out strings.Builder
	last := false
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
			last = false
		} else if !last && out.Len() > 0 {
			out.WriteByte('_')
			last = true
		}
	}
	value = strings.Trim(strings.TrimSuffix(out.String(), "_"), "_")
	return clip(value, 20)
}
func validateData(values map[string]any, fields []map[string]any, partial bool) string {
	return validateDataWithFiles(values, fields, partial, nil)
}
func validateDataWithFiles(values map[string]any, fields []map[string]any, partial bool, uploaded map[string]bool) string {
	known := map[string]map[string]any{}
	for _, field := range fields {
		known[stringValue(field["name"])] = field
	}
	for name := range values {
		if known[name] == nil {
			return "数据表中没有「" + name + "」字段"
		}
	}
	for _, field := range fields {
		name, typ, required := stringValue(field["name"]), stringValue(field["type"]), boolValue(field["required"])
		value, present := values[name]
		if !present {
			if !partial && required && !(typ == "file" && uploaded[name]) {
				return "字段「" + defaultString(stringValue(field["label"]), name) + "」不能为空"
			}
			continue
		}
		label := defaultString(stringValue(field["label"]), name)
		if required && (value == nil || value == "") && !(typ == "file" && uploaded[name]) {
			return "字段「" + label + "」不能为空"
		}
		if typ == "file" && (value == nil || value == "") && uploaded[name] {
			continue
		}
		if value == nil {
			return "字段「" + label + "」的值类型无效"
		}
		switch typ {
		case "text", "date", "email", "url", "select", "relation", "member", "file":
			if _, ok := value.(string); !ok {
				return "字段「" + label + "」的值类型无效"
			}
		case "number":
			switch v := value.(type) {
			case float64:
				if v != v {
					return "字段「" + label + "」的值类型无效"
				}
			case int, int64, json.Number:
			default:
				return "字段「" + label + "」的值类型无效"
			}
		case "bool":
			if _, ok := value.(bool); !ok {
				return "字段「" + label + "」的值类型无效"
			}
		default:
			return "暂不支持「" + typ + "」字段"
		}
		if typ == "select" && value != "" && !contains(field["options"], value) {
			return "字段「" + label + "」的选项无效"
		}
	}
	return ""
}

func stringSet(raw any) map[string]bool {
	values := map[string]bool{}
	for _, item := range anySlice(raw) {
		if value, ok := item.(string); ok && value != "" {
			values[value] = true
		}
	}
	return values
}
func validateRelations(ctx context.Context, pb interface {
	Find(context.Context, string, string) (map[string]any, error)
	Get(context.Context, string, string) (map[string]any, error)
}, values map[string]any, fields []map[string]any, appID, tenantID string) string {
	for _, field := range fields {
		if field["type"] != "relation" {
			continue
		}
		recordID := stringValue(values[stringValue(field["name"])])
		if recordID == "" {
			continue
		}
		target, err := pb.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID), "slug = "+pbFilterString(stringValue(field["target"]))))
		if err != nil {
			return "关联字段「" + stringValue(field["label"]) + "」的数据表不存在"
		}
		row, err := pb.Get(ctx, stringValue(target["pb_collection"]), recordID)
		if err != nil || row["app_id"] != appID || row["tenant_id"] != tenantID {
			return "关联字段「" + stringValue(field["label"]) + "」的记录无效"
		}
	}
	return ""
}
func validateMemberReferences(ctx context.Context, pb interface {
	Find(context.Context, string, string) (map[string]any, error)
	Get(context.Context, string, string) (map[string]any, error)
}, values map[string]any, fields []map[string]any, app map[string]any, tenantID string) string {
	for _, field := range fields {
		if field["type"] != "member" {
			continue
		}
		userID := stringValue(values[stringValue(field["name"])])
		if userID == "" {
			continue
		}
		member, err := pb.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(tenantID), "user_id = "+pbFilterString(userID)))
		if err != nil || member == nil {
			return "成员字段的账号不是工作区成员"
		}
		user, err := pb.Get(ctx, "users", userID)
		if err != nil || boolValue(user["disabled"]) {
			return "成员字段的账号不可用"
		}
		if boolValue(app["restricted"]) {
			if member["role"] == "owner" && userID == app["creator_id"] {
				continue
			}
			if _, err := pb.Find(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(stringValue(app["id"])), "user_id = "+pbFilterString(userID))); err != nil {
				return "成员字段的账号没有此应用的访问权限"
			}
		}
	}
	return ""
}

func recordData(row map[string]any) map[string]any { return asMap(publicRecord(row)["data"]) }
func equalJSON(a, b any) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return bytes.Equal(aa, bb)
}
