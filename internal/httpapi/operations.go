package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/tans/miao/internal/pocketbase"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

var jobLocks sync.Map

func lockJob(id string) func() {
	value, _ := jobLocks.LoadOrStore(id, &sync.Mutex{})
	mutex := value.(*sync.Mutex)
	mutex.Lock()
	return mutex.Unlock
}

func (s *Server) routesOperations() {
	s.Mux.HandleFunc("POST /api/apps/{id}/query", s.auth(s.queryRecords))
	s.Mux.HandleFunc("POST /api/apps/{id}/batch-plans", s.auth(s.createBatchPlan))
	s.Mux.HandleFunc("GET /api/apps/{id}/batch-plans/{jobId}", s.auth(s.getBatchPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/batch-plans/{jobId}/commit", s.auth(s.commitBatchPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/import-plans", s.auth(s.createImportPlan))
	s.Mux.HandleFunc("GET /api/apps/{id}/import-plans/{planId}", s.auth(s.getImportPlan))
	s.Mux.HandleFunc("POST /api/apps/{id}/import-plans/{planId}/commit", s.auth(s.commitImportPlan))
}

func (s *Server) operationTable(ctx context.Context, r *http.Request, input map[string]any) (map[string]any, map[string]any, error) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, nil, err
	}
	slug := defaultString(stringValue(input["table"]), r.URL.Query().Get("table"))
	if slug == "" {
		return app, nil, fmt.Errorf("missing table")
	}
	table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(slug)))
	return app, table, err
}

func buildRecordFilter(conditions any, fields []map[string]any, tenantID, appID string) (string, error) {
	list := anySlice(conditions)
	if len(list) > 8 {
		return "", fmt.Errorf("查询条件最多 8 项")
	}
	parts := []string{"tenant_id = " + pbFilterString(tenantID), "app_id = " + pbFilterString(appID)}
	for _, raw := range list {
		cond := asMap(raw)
		name, op := stringValue(cond["field"]), stringValue(cond["op"])
		field := findField(fields, name)
		if field == nil || !contains([]string{"eq", "contains", "before", "after", "empty"}, op) {
			return "", fmt.Errorf("查询字段或操作符无效")
		}
		typ := stringValue(field["type"])
		if (op == "before" || op == "after") && typ != "date" {
			return "", fmt.Errorf("日期条件必须使用日期字段")
		}
		if op == "contains" && !contains([]string{"text", "email", "url"}, typ) {
			return "", fmt.Errorf("包含查询只能用于文本字段")
		}
		if op == "empty" {
			parts = append(parts, name+` = ""`)
			continue
		}
		value := cond["value"]
		if typ == "number" {
			n, ok := numeric(value)
			if !ok {
				return "", fmt.Errorf("查询值类型无效")
			}
			value = strconv.FormatFloat(n, 'f', -1, 64)
		}
		if typ == "bool" {
			if value == true || value == "true" {
				value = "true"
			} else if value == false || value == "false" {
				value = "false"
			} else {
				return "", fmt.Errorf("查询值类型无效")
			}
		}
		if typ != "number" && typ != "bool" {
			text, ok := value.(string)
			if !ok {
				return "", fmt.Errorf("查询值类型无效")
			}
			value = pbFilterString(text)
		} else {
			value = fmt.Sprint(value)
		}
		operator := map[string]string{"eq": "=", "contains": "~", "before": "<", "after": ">"}[op]
		parts = append(parts, name+" "+operator+" "+fmt.Sprint(value))
	}
	return listFilter(parts...), nil
}

func numeric(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, n == n
	case int:
		return float64(n), true
	case string:
		parsed, err := strconv.ParseFloat(n, 64)
		return parsed, err == nil && parsed == parsed && !math.IsInf(parsed, 0)
	}
	return 0, false
}

func (s *Server) queryRecords(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	input := mapBody(r)
	app, table, err := s.operationTable(ctx, r, input)
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	filter, err := buildRecordFilter(input["conditions"], asSliceMap(table["fields"]), stringValue(who(r).Tenant["id"]), stringValue(app["id"]))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	page := queryIntFrom(input, "page", 1, 1, 100000)
	rows, total, pages, err := s.PB.List(ctx, stringValue(table["pb_collection"]), filter, "-created", page, 25)
	if err != nil {
		writeError(w, 503, "记录查询暂时不可用")
		return
	}
	items := []map[string]any{}
	for _, row := range rows {
		item := publicRecord(row)
		item["updated_at"] = row["updated"]
		delete(item, "created_at")
		items = append(items, item)
	}
	writeJSON(w, 200, map[string]any{"items": items, "totalItems": total, "page": page, "totalPages": pages, "conditions": anySlice(input["conditions"])})
}

func (s *Server) createBatchPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	input := mapBody(r)
	app, table, err := s.operationTable(ctx, r, input)
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	id := who(r)
	if !s.appCanBatch(ctx, app, id) {
		writeError(w, 403, "没有批量修改权限")
		return
	}
	change := asMap(input["change"])
	field := findField(asSliceMap(table["fields"]), stringValue(change["field"]))
	if field == nil || field["type"] == "file" || field["type"] == "relation" {
		writeError(w, 400, "批量修改字段无效")
		return
	}
	if msg := validateData(map[string]any{stringValue(field["name"]): change["value"]}, []map[string]any{field}, true); msg != "" {
		writeError(w, 400, "批量修改值无效")
		return
	}
	filter, err := buildRecordFilter(input["conditions"], asSliceMap(table["fields"]), stringValue(id.Tenant["id"]), stringValue(app["id"]))
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	rows, total, _, err := s.PB.List(ctx, stringValue(table["pb_collection"]), filter, "created", 1, 101)
	if err != nil {
		writeError(w, 503, "目标记录检查失败")
		return
	}
	if total < 1 || total > 100 {
		writeError(w, 400, "请把目标范围缩小到 1–100 条记录")
		return
	}
	targets := []map[string]any{}
	sample := []map[string]any{}
	for i, row := range rows {
		targets = append(targets, map[string]any{"id": row["id"], "updated_at": row["updated"]})
		if i < 10 {
			sample = append(sample, publicRecord(row))
		}
	}
	job, err := s.PB.Create(ctx, "batch_jobs", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "user_id": id.User["id"], "status": "planned", "plan": map[string]any{"kind": "batch", "table": table["slug"], "fields": table["fields"], "conditions": anySlice(input["conditions"]), "change": change, "targets": targets}})
	if err != nil {
		writeError(w, 503, "批量计划创建失败")
		return
	}
	writeJSON(w, 201, map[string]any{"plan_id": job["id"], "table": table["slug"], "count": len(targets), "change": change, "sample": sample, "status": "planned"})
}

func (s *Server) ownedBatchJob(ctx context.Context, r *http.Request, idParam string, kind string) (map[string]any, bool) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, false
	}
	job, err := s.PB.Get(ctx, "batch_jobs", pathID(r, idParam))
	id := who(r)
	if err != nil || job["tenant_id"] != id.Tenant["id"] || job["app_id"] != app["id"] || job["user_id"] != id.User["id"] || (stringValue(asMap(job["plan"])["kind"]) == "import") != (kind == "import") {
		return nil, false
	}
	return job, true
}

func (s *Server) getBatchPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	job, ok := s.ownedBatchJob(ctx, r, "jobId", "batch")
	if !ok {
		writeError(w, 404, "批量计划不存在")
		return
	}
	plan := asMap(job["plan"])
	writeJSON(w, 200, map[string]any{"id": job["id"], "status": job["status"], "count": len(anySlice(plan["targets"])), "change": plan["change"], "result": job["result"]})
}

func (s *Server) commitBatchPlan(w http.ResponseWriter, r *http.Request) {
	s.commitRecordPlan(w, r, "jobId", "batch")
}

func (s *Server) createImportPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	input := mapBody(r)
	app, table, err := s.operationTable(ctx, r, input)
	if err != nil {
		writeError(w, 404, "数据表不存在")
		return
	}
	id := who(r)
	if !s.appCanBatch(ctx, app, id) || boolValue(app["archived"]) {
		writeError(w, 403, "导入需要批量写入权限")
		return
	}
	rows, ok := input["rows"].([]any)
	if !ok || len(rows) < 1 || len(rows) > 100 {
		writeError(w, 400, "导入需要真实数据表及 1–100 行已映射的数据")
		return
	}
	raw, _ := json.Marshal(rows)
	if len(raw) > 500000 {
		writeError(w, 400, "导入计划超过大小限制")
		return
	}
	fields := asSliceMap(table["fields"])
	errors := []map[string]any{}
	for i, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			errors = append(errors, map[string]any{"row": i + 1, "error": "每行必须是对象"})
			continue
		}
		msg := validateData(row, fields, false)
		if msg == "" {
			msg = validateRelations(ctx, s.PB, row, fields, stringValue(app["id"]), stringValue(id.Tenant["id"]))
		}
		if msg != "" {
			errors = append(errors, map[string]any{"row": i + 1, "error": msg})
		}
	}
	if len(errors) > 0 {
		writeJSON(w, 400, map[string]any{"error": "导入校验失败，没有写入记录", "errors": errors})
		return
	}
	plan := map[string]any{"kind": "import", "table": table["slug"], "fields": fields, "rows": rows, "targets": []any{}}
	job, err := s.PB.Create(ctx, "batch_jobs", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "user_id": id.User["id"], "status": "planned", "plan": plan})
	if err != nil {
		writeError(w, 503, "导入计划创建失败")
		return
	}
	sample := rows
	if len(sample) > 10 {
		sample = sample[:10]
	}
	writeJSON(w, 201, map[string]any{"plan_id": job["id"], "table": table["slug"], "count": len(rows), "sample": sample, "status": "planned", "note": "只新增，不自动合并；请审阅后确认。计划 15 分钟内有效。"})
}

func (s *Server) getImportPlan(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	job, ok := s.ownedBatchJob(ctx, r, "planId", "import")
	if !ok {
		writeError(w, 404, "导入计划不存在")
		return
	}
	plan := asMap(job["plan"])
	writeJSON(w, 200, map[string]any{"id": job["id"], "status": job["status"], "count": len(anySlice(plan["rows"])), "result": job["result"]})
}

func (s *Server) commitImportPlan(w http.ResponseWriter, r *http.Request) {
	s.commitRecordPlan(w, r, "planId", "import")
}

type planTarget struct {
	ID        string `json:"id"`
	UpdatedAt string `json:"updated_at"`
}
type planChange struct {
	Field string `json:"field"`
	Value any    `json:"value"`
}
type recordPlan struct {
	Kind       string           `json:"kind"`
	Table      string           `json:"table"`
	Fields     []map[string]any `json:"fields"`
	Conditions []any            `json:"conditions,omitempty"`
	Change     planChange       `json:"change,omitempty"`
	Targets    []planTarget     `json:"targets"`
	Rows       []map[string]any `json:"rows,omitempty"`
}
type planItem struct {
	Row    int    `json:"row,omitempty"`
	ID     string `json:"id,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
type planReceipt struct {
	Created    int        `json:"created"`
	Updated    int        `json:"updated"`
	Conflicted int        `json:"conflicted"`
	Failed     int        `json:"failed"`
	Items      []planItem `json:"items"`
}
type planConfirmation struct {
	Confirm bool   `json:"confirm"`
	PlanID  string `json:"plan_id"`
}

func (s *Server) commitRecordPlan(w http.ResponseWriter, r *http.Request, parameter, kind string) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	job, ok := s.ownedBatchJob(ctx, r, parameter, kind)
	if !ok {
		writeError(w, 404, "计划不存在")
		return
	}
	unlock := lockJob(stringValue(job["id"]))
	defer unlock()
	job, err := s.PB.Get(ctx, "batch_jobs", stringValue(job["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	var input planConfirmation
	if readJSON(r, &input) != nil || !input.Confirm || input.PlanID != stringValue(job["id"]) {
		writeError(w, 400, "请确认这个具体计划")
		return
	}
	if job["status"] == "completed" || job["status"] == "partial" {
		writeJSON(w, 200, map[string]any{"status": job["status"], "result": job["result"]})
		return
	}
	if job["status"] != "planned" || time.Since(parseTime(job["created"])) > 15*time.Minute {
		writeError(w, 409, "计划已过期或执行中；请查看已有回执，不要重复提交")
		return
	}
	var plan recordPlan
	encoded, err := json.Marshal(job["plan"])
	if err != nil || json.Unmarshal(encoded, &plan) != nil {
		writeError(w, 409, "计划内容无效，请重新预览")
		return
	}
	actor := who(r).actor(stringValue(job["app_id"]), kind)
	if _, err := s.authorizeWrite(ctx, s.PB, actor, true); err != nil {
		s.writeBusinessError(w, err)
		return
	}
	table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(actor.TenantID), "app_id = "+pbFilterString(actor.AppID), "slug = "+pbFilterString(plan.Table)))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	if !equalJSON(table["fields"], plan.Fields) {
		writeError(w, 409, "表结构已变化，请重新预览")
		return
	}
	if _, err := s.PB.Update(ctx, "batch_jobs", input.PlanID, map[string]any{"status": "running"}); err != nil {
		s.writeBusinessError(w, err)
		return
	}
	result := planReceipt{Items: []planItem{}}
	count := len(plan.Targets)
	if kind == "import" {
		count = len(plan.Rows)
	}
	for i := 0; i < count; i++ {
		cmd := recordWrite{Actor: actor, Table: plan.Table, Batch: true, FieldsSnapshot: plan.Fields}
		if kind == "import" {
			cmd.Data = plan.Rows[i]
		} else {
			cmd.RecordID, cmd.ExpectedUpdated = plan.Targets[i].ID, plan.Targets[i].UpdatedAt
			cmd.Data = map[string]any{plan.Change.Field: plan.Change.Value}
		}
		_, err := s.saveBusinessRecord(ctx, cmd, func(tx *pocketbase.Client, row map[string]any) error {
			next := result
			next.Items = append(append([]planItem{}, result.Items...), planItem{Row: i + 1, ID: stringValue(row["id"]), Status: "updated"})
			if kind == "import" {
				next.Created++
				next.Items[len(next.Items)-1].Status = "created"
			} else {
				next.Updated++
			}
			if _, err := tx.Update(ctx, "batch_jobs", input.PlanID, map[string]any{"result": next}); err != nil {
				return err
			}
			result = next
			return nil
		})
		if err != nil {
			var pe *pocketbase.Error
			if !errors.As(err, &pe) || pe.Status >= 500 {
				s.writeBusinessError(w, err)
				return
			}
			item := planItem{Row: i + 1, ID: cmd.RecordID, Status: "failed", Error: pe.Message}
			if pe.Status == 409 && kind == "batch" {
				result.Conflicted++
				item.Status = "conflict"
			} else {
				result.Failed++
			}
			if pe.Status == 403 {
				item.Status = "permission_changed"
				result.Failed += count - i - 1
				result.Items = append(result.Items, item)
				break
			}
			result.Items = append(result.Items, item)
		}
	}
	status := "completed"
	if result.Failed > 0 || result.Conflicted > 0 {
		status = "partial"
	}
	if _, err := s.PB.Update(ctx, "batch_jobs", input.PlanID, map[string]any{"status": status, "result": result}); err != nil {
		s.writeBusinessError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"status": status, "result": result})
}
