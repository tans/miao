package httpapi

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Single-table CSV export: a member-readable stream of one table's records.
// XLSX stays out for now; CSV already opens in Excel, and the JSON workspace
// export keeps covering backup use.

// exportRowBudget caps one export; larger volumes belong in a backup.
const exportRowBudget = 20000

func (s *Server) exportTable(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	id := who(r)
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	table, err := s.tableForRequest(ctx, r, app)
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	fields, err := exportFields(table, r.URL.Query().Get("fields"))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	parts := []string{"tenant_id = " + pbFilterString(stringValue(id.Tenant["id"])), "app_id = " + pbFilterString(stringValue(app["id"]))}
	from, to, err := exportCreatedRange(r)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if from != "" {
		parts = append(parts, "created >= "+pbFilterString(from))
	}
	if to != "" {
		parts = append(parts, "created <= "+pbFilterString(to))
	}
	sortName := r.URL.Query().Get("sort")
	allowed := map[string]bool{"created": true, "updated": true}
	for _, field := range fields {
		allowed[stringValue(field["name"])] = true
	}
	if !allowed[strings.TrimPrefix(sortName, "-")] {
		sortName = "-created"
	}
	labels := s.exportLabels(ctx, app, stringValue(id.Tenant["id"]), fields)
	writer := csv.NewWriter(w)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"miao-"+url.PathEscape(stringValue(table["slug"]))+".csv\"")
	// BOM keeps Excel reading the file as UTF-8.
	w.Write([]byte{0xEF, 0xBB, 0xBF})
	header := []string{"id", "created_at"}
	for _, field := range fields {
		header = append(header, defaultString(stringValue(field["label"]), stringValue(field["name"])))
	}
	writer.Write(header)
	for page := 1; page <= exportRowBudget/1000; page++ {
		rows, _, _, err := s.PB.List(ctx, stringValue(table["pb_collection"]), listFilter(parts...), sortName, page, 1000)
		if err != nil {
			return
		}
		for _, row := range rows {
			record := []string{stringValue(row["id"]), exportText(row["created"])}
			for _, field := range fields {
				record = append(record, exportCell(row, field, labels))
			}
			writer.Write(record)
		}
		if len(rows) < 1000 {
			break
		}
	}
	writer.Flush()
}

// exportFields resolves the requested field subset in table order; an empty
// request selects every defined field and an unknown name fails loudly.
func exportFields(table map[string]any, raw string) ([]map[string]any, error) {
	defined := asSliceMap(table["fields"])
	if strings.TrimSpace(raw) == "" {
		return defined, nil
	}
	requested := map[string]bool{}
	for _, name := range strings.Split(raw, ",") {
		if name = strings.TrimSpace(name); name != "" {
			requested[name] = true
		}
	}
	selected := []map[string]any{}
	for _, field := range defined {
		if requested[stringValue(field["name"])] {
			selected = append(selected, field)
			delete(requested, stringValue(field["name"]))
		}
	}
	for name := range requested {
		return nil, fmt.Errorf("字段「%s」不存在", name)
	}
	return selected, nil
}

// exportCreatedRange validates the optional created-time window as RFC3339.
func exportCreatedRange(r *http.Request) (string, string, error) {
	parse := func(name string) (string, error) {
		raw := strings.TrimSpace(r.URL.Query().Get(name))
		if raw == "" {
			return "", nil
		}
		value, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return "", fmt.Errorf("%s 必须是 RFC3339 时间", name)
		}
		return value.UTC().Format(time.RFC3339Nano), nil
	}
	from, err := parse("from")
	if err != nil {
		return "", "", err
	}
	to, err := parse("to")
	return from, to, err
}

// exportLabels resolves member and relation fields to display labels, capped
// like the published UI at 100 target rows per relation field.
func (s *Server) exportLabels(ctx context.Context, app map[string]any, tenantID string, fields []map[string]any) map[string]map[string]string {
	labels := map[string]map[string]string{}
	needs := map[string]bool{}
	for _, field := range fields {
		switch stringValue(field["type"]) {
		case "member":
			needs["member"] = true
		case "relation":
			needs[stringValue(field["target"])] = true
		}
	}
	if needs["member"] {
		tenant, err := s.PB.Get(ctx, "tenants", tenantID)
		if err == nil {
			members, err := s.PB.ListAll(ctx, "tenant_members", "tenant_id = "+pbFilterString(tenantID), "created")
			if err == nil {
				if choices, err := s.appMemberChoices(ctx, app, tenant, members); err == nil {
					ids := map[string]string{}
					for _, choice := range choices {
						ids[stringValue(choice["id"])] = stringValue(choice["label"])
					}
					labels["__member__"] = ids
				}
			}
		}
	}
	tables, err := s.appTables(ctx, app, tenantID)
	if err != nil {
		return labels
	}
	for _, field := range fields {
		target := stringValue(field["target"])
		if stringValue(field["type"]) != "relation" || !needs[target] || labels[target] != nil {
			continue
		}
		var resolved map[string]any
		for _, candidate := range tables {
			if stringValue(candidate["slug"]) == target {
				resolved = candidate
				break
			}
		}
		if resolved == nil {
			continue
		}
		labelField := ""
		for _, candidate := range asSliceMap(resolved["fields"]) {
			if stringValue(candidate["type"]) == "text" {
				labelField = stringValue(candidate["name"])
				break
			}
		}
		rows, _, _, err := s.PB.List(ctx, stringValue(resolved["pb_collection"]), listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(stringValue(app["id"]))), "-created", 1, 100)
		if err != nil || labelField == "" {
			continue
		}
		ids := map[string]string{}
		for _, row := range rows {
			ids[stringValue(row["id"])] = stringValue(row[labelField])
		}
		labels[target] = ids
	}
	return labels
}

func exportCell(row, field map[string]any, labels map[string]map[string]string) string {
	value := row[stringValue(field["name"])]
	switch stringValue(field["type"]) {
	case "member":
		if label := labels["__member__"][stringValue(value)]; label != "" {
			return label
		}
	case "relation":
		if label := labels[stringValue(field["target"])][stringValue(value)]; label != "" {
			return label
		}
	}
	return exportText(value)
}

func exportText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, exportText(item))
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprint(typed)
	}
}
