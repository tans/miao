package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// Global search scans text fields of every table inside the apps the caller
// can read. Each table contributes at most searchPerTable matches per page and
// the whole request shares one timeout, so a large workspace returns fewer
// results instead of a slow full scan.
const (
	searchPerTable = 5
	searchMaxItems = 50
	searchTimeout  = 8 * time.Second
)

func (s *Server) globalSearch(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	tenantID := stringValue(id.Tenant["id"])
	query := clip(strings.TrimSpace(r.URL.Query().Get("q")), 120)
	page := queryInt(r, "page", 1, 1, 20)
	if query == "" {
		writeJSON(w, 200, pageResult([]map[string]any{}, 1, searchPerTable, 0))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), searchTimeout)
	defer cancel()
	apps, err := s.visibleApps(ctx, id, false)
	if err != nil {
		writeError(w, 503, "全局搜索暂时不可用")
		return
	}
	lowerQuery := strings.ToLower(query)
	items := []map[string]any{}
	total := 0
	truncated := false
	for _, app := range apps {
		appID, appName := stringValue(app["id"]), stringValue(app["name"])
		tables, err := s.appTables(ctx, app, tenantID)
		if err != nil {
			continue
		}
		for _, table := range tables {
			alternatives := []string{}
			for _, field := range asSliceMap(table["fields"]) {
				if contains([]string{"text", "email", "url"}, field["type"]) {
					alternatives = append(alternatives, stringValue(field["name"])+" ~ "+pbFilterString(query))
				}
			}
			if len(alternatives) == 0 {
				continue
			}
			filter := listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID), "("+strings.Join(alternatives, " || ")+")")
			rows, rowTotal, _, err := s.PB.List(ctx, stringValue(table["pb_collection"]), filter, "-updated", page, searchPerTable)
			if err != nil {
				continue
			}
			total += rowTotal
			for _, row := range rows {
				items = append(items, map[string]any{
					"app_id": appID, "app_name": appName,
					"table": stringValue(table["slug"]), "table_name": stringValue(table["name"]),
					"snippet": searchSnippet(row, asSliceMap(table["fields"]), lowerQuery),
					"record":  publicRecord(row),
				})
			}
			if len(items) >= searchMaxItems {
				truncated = true
				break
			}
		}
		if truncated {
			break
		}
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "perPage": searchPerTable, "totalItems": total, "truncated": truncated})
}

// searchSnippet returns the first text field whose value contains the query,
// so the UI can show why a record matched without extra requests.
func searchSnippet(row map[string]any, fields []map[string]any, lowerQuery string) string {
	for _, field := range fields {
		if !contains([]string{"text", "email", "url"}, field["type"]) {
			continue
		}
		value := strings.TrimSpace(stringValue(row[stringValue(field["name"])]))
		if value != "" && strings.Contains(strings.ToLower(value), lowerQuery) {
			return clip(stringValue(field["label"])+"："+value, 80)
		}
	}
	return ""
}
