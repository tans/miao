package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var requestIDPattern = regexp.MustCompile("^[\\w-]{1,100}$")

func publicRun(run map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range run {
		if !containsString([]string{"snapshot", "checkpoint", "harness_loop", "harness_result", "harness_candidate", "harness_authority"}, k) {
			out[k] = v
		}
	}
	snapshot := asMap(run["snapshot"])
	out["task_name"], out["revision"], out["trigger"], out["mode"] = snapshot["name"], snapshot["revision"], snapshot["trigger"], defaultString(stringValue(snapshot["mode"]), "live")
	return out
}

func (s *Server) loadOwnedTask(ctx context.Context, w http.ResponseWriter, r *http.Request, manage bool) (map[string]any, map[string]any, bool) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return nil, nil, false
	}
	if manage && (!canPublishAppRole(s.appPermission(ctx, app, who(r))) || boolValue(app["archived"])) {
		writeError(w, 403, "需要当前应用的发布权限，且应用未归档")
		return app, nil, false
	}
	task, err := s.PB.Get(ctx, "miao_tasks", pathID(r, "taskId"))
	id := who(r)
	if err != nil || task["tenant_id"] != id.Tenant["id"] || task["app_id"] != app["id"] {
		writeError(w, 404, "任务或运行不存在")
		return app, nil, false
	}
	if manage && task["created_by"] != id.User["id"] && s.appPermission(ctx, app, id) != "owner" {
		writeError(w, 403, "由任务负责人或工作区所有者处理")
		return app, nil, false
	}
	return app, task, true
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	page := queryInt(r, "page", 1, 1, 10000)
	rows, total, _, err := s.PB.List(ctx, "miao_tasks", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "-created", page, 25)
	if err != nil {
		writeError(w, 503, "后台任务暂不可用")
		return
	}
	writeJSON(w, 200, pageResult(rows, page, 25, total))
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canPublishAppRole(role) || boolValue(app["archived"]) {
		writeError(w, 403, "需要当前应用的发布权限，且应用未归档")
		return
	}
	input := mapBody(r)
	name := strings.TrimSpace(stringValue(input["name"]))
	if name == "" || len([]rune(name)) > 160 {
		writeError(w, 400, "任务名称不能为空，最多 160 字")
		return
	}
	definition, msg := normalizeTaskDefinition(ctx, s, stringValue(who(r).Tenant["id"]), stringValue(app["id"]), input["definition"])
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	id := who(r)
	task, err := s.PB.Create(ctx, "miao_tasks", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "created_by": id.User["id"], "name": name, "definition": definition, "revision": 1, "status": "draft"})
	if err != nil {
		writeError(w, 503, "任务创建失败")
		return
	}
	writeJSON(w, 201, task)
}

func (s *Server) createTaskFromAgent(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, role, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	if !canPublishAppRole(role) || boolValue(app["archived"]) {
		writeError(w, 403, "需要当前应用的发布权限，且应用未归档")
		return
	}
	input := mapBody(r)
	goal := strings.TrimSpace(stringValue(input["goal"]))
	if goal == "" {
		writeError(w, 400, "请提供后台任务目标")
		return
	}
	if table := strings.TrimSpace(stringValue(input["table"])); table != "" {
		goal += "\n目标数据表：" + table
	}
	if recordID := strings.TrimSpace(stringValue(input["record_id"])); recordID != "" {
		goal += "\n目标记录 ID：" + recordID
	}
	goal = clip(goal, 6000)
	tenantID, appID := stringValue(who(r).Tenant["id"]), stringValue(app["id"])
	tables, err := s.PB.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(appID)), "created")
	if err != nil {
		writeError(w, 503, "后台任务暂不可用")
		return
	}
	grants := make([]any, 0, 12)
	for _, table := range tables {
		fields := make([]any, 0, 24)
		for _, field := range asSliceMap(table["fields"]) {
			if containsString([]string{"file", "relation"}, stringValue(field["type"])) {
				continue
			}
			fields = append(fields, stringValue(field["name"]))
			if len(fields) == 24 {
				break
			}
		}
		if len(fields) > 0 {
			grants = append(grants, map[string]any{"table": stringValue(table["slug"]), "read_fields": fields, "write_fields": []any{}})
		}
		if len(grants) == 12 {
			break
		}
	}
	definition, msg := normalizeTaskDefinition(ctx, s, tenantID, appID, map[string]any{
		"goal":      goal,
		"execution": "agent",
		"trigger":   map[string]any{"type": "manual", "timezone": "Asia/Shanghai"},
		"scope":     map[string]any{"tables": grants, "action_ids": []any{}, "recipient_ids": []any{}},
	})
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	name := strings.TrimSpace(stringValue(input["name"]))
	if name == "" {
		name = "后台 Agent"
	}
	if len([]rune(name)) > 160 {
		name = clip(name, 160)
	}
	id := who(r)
	task, err := s.PB.Create(ctx, "miao_tasks", map[string]any{"tenant_id": id.Tenant["id"], "app_id": app["id"], "created_by": id.User["id"], "name": name, "definition": definition, "revision": 1, "status": "draft"})
	if err != nil {
		writeError(w, 503, "任务创建失败")
		return
	}
	writeJSON(w, 201, task)
}

func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, task, ok := s.loadOwnedTask(ctx, w, r, true)
	if !ok {
		return
	}
	input := mapBody(r)
	if !containsString([]string{"draft", "paused"}, stringValue(task["status"])) || intValue(input["expected_revision"]) != intValue(task["revision"]) {
		writeError(w, 409, "先暂停任务，并读取最新版本后修改")
		return
	}
	definition, msg := normalizeTaskDefinition(ctx, s, stringValue(task["tenant_id"]), stringValue(app["id"]), input["definition"])
	if msg != "" {
		writeError(w, 400, msg)
		return
	}
	name := strings.TrimSpace(defaultString(stringValue(input["name"]), stringValue(task["name"])))
	if name == "" {
		writeError(w, 400, "任务名称不能为空")
		return
	}
	saved, err := s.PB.Update(ctx, "miao_tasks", stringValue(task["id"]), map[string]any{"name": clip(name, 160), "definition": definition, "revision": intValue(task["revision"]) + 1, "status": "draft", "next_run_at": "", "pause_reason": ""})
	if err != nil {
		writeError(w, 503, "任务更新失败")
		return
	}
	writeJSON(w, 200, saved)
}

func (s *Server) taskAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	action := pathAction(r)
	_, task, ok := s.loadOwnedTask(ctx, w, r, true)
	if !ok {
		return
	}
	input := mapBody(r)
	switch action {
	case "enable":
		if input["confirm"] != true || intValue(input["expected_revision"]) != intValue(task["revision"]) {
			writeError(w, 409, "请审阅并确认任务的最新具体版本")
			return
		}
		if _, err := taskAuthority(ctx, s, task); err != nil {
			writeError(w, 403, "任务负责人已失去权限或应用已归档")
			return
		}
		if _, msg := normalizeTaskDefinition(ctx, s, stringValue(task["tenant_id"]), stringValue(task["app_id"]), task["definition"]); msg != "" {
			writeError(w, 400, msg)
			return
		}
		definition := asMap(task["definition"])
		trigger := asMap(definition["trigger"])
		next := nextScheduledRun(trigger, time.Now())
		if trigger["type"] == "once" && next == "" {
			writeError(w, 400, "一次性任务的时间已过，请修改时间后重新确认")
			return
		}
		saved, err := s.PB.Update(ctx, "miao_tasks", stringValue(task["id"]), map[string]any{"status": "enabled", "next_run_at": next, "pause_reason": ""})
		if err != nil {
			writeError(w, 503, "任务启用失败")
			return
		}
		writeJSON(w, 200, saved)
	case "pause":
		saved, err := s.PB.Update(ctx, "miao_tasks", stringValue(task["id"]), map[string]any{"status": "paused", "next_run_at": "", "pause_reason": "用户暂停；已创建的运行可单独取消"})
		if err != nil {
			writeError(w, 503, "任务暂停失败")
			return
		}
		writeJSON(w, 200, saved)
	case "preview", "run":
		if (action == "preview" && (task["status"] == "archived" || intValue(input["expected_revision"]) != intValue(task["revision"]))) || (action == "run" && (task["status"] != "enabled" || intValue(input["expected_revision"]) != intValue(task["revision"]))) {
			writeError(w, 409, "读取并确认当前任务版本后再试运行")
			return
		}
		if _, err := taskAuthority(ctx, s, task); err != nil {
			writeError(w, 403, "任务负责人已失去权限或应用已归档")
			return
		}
		key := stringValue(input["request_id"])
		if key == "" {
			key, _ = randomToken()
		}
		if !requestIDPattern.MatchString(key) {
			writeError(w, 400, "请求标识无效")
			return
		}
		snapshotTask := task
		if action == "preview" {
			snapshotTask = previewTask(task)
		}
		run, err := enqueueTaskRun(ctx, s, snapshotTask, action+":"+strconv.Itoa(intValue(task["revision"]))+":"+key, nil)
		if err != nil {
			writeError(w, 503, "运行创建失败")
			return
		}
		writeJSON(w, 202, publicRun(run))
	case "archive":
		if input["confirm"] != true || intValue(input["expected_revision"]) != intValue(task["revision"]) {
			writeError(w, 409, "请确认当前任务版本归档，剩余运行将取消")
			return
		}
		saved, err := s.PB.Update(ctx, "miao_tasks", stringValue(task["id"]), map[string]any{"status": "archived", "next_run_at": "", "pause_reason": "用户归档"})
		if err != nil {
			writeError(w, 503, "任务归档失败")
			return
		}
		runs, err := s.PB.ListAll(ctx, "miao_runs", listFilter("task_id = "+pbFilterString(stringValue(task["id"]))), "")
		if err != nil {
			writeError(w, 503, "任务已归档，但运行取消状态暂不可用；请重试归档")
			return
		}
		for _, run := range runs {
			if containsString([]string{"queued", "running", "waiting"}, stringValue(run["status"])) {
				if _, err := s.PB.Update(ctx, "miao_runs", stringValue(run["id"]), map[string]any{"cancel_requested": true, "status": "cancelled", "error": "任务已归档", "finished_at": nowISO()}); err != nil {
					writeError(w, 503, "任务已归档，但部分运行取消失败；请重试归档")
					return
				}
				s.cancelRun(stringValue(run["id"]))
			}
		}
		writeJSON(w, 200, saved)
	case "restore":
		if task["status"] != "archived" || intValue(input["expected_revision"]) != intValue(task["revision"]) {
			writeError(w, 409, "请读取当前归档版本")
			return
		}
		saved, err := s.PB.Update(ctx, "miao_tasks", stringValue(task["id"]), map[string]any{"status": "draft", "revision": intValue(task["revision"]) + 1, "next_run_at": "", "pause_reason": ""})
		if err != nil {
			writeError(w, 503, "任务恢复失败")
			return
		}
		writeJSON(w, 200, saved)
	case "transfer":
		if task["status"] == "archived" || input["confirm"] != true || intValue(input["expected_revision"]) != intValue(task["revision"]) {
			writeError(w, 409, "确认当前版本的负责人转交，转交后需新负责人重新启用")
			return
		}
		pending, _, _, _ := s.PB.List(ctx, "miao_runs", listFilter("task_id = "+pbFilterString(stringValue(task["id"])), "(status = \"queued\" || status = \"running\" || status = \"waiting\")"), "", 1, 1)
		if len(pending) > 0 {
			writeError(w, 409, "先处理或取消原负责人尚未结束的运行")
			return
		}
		newOwner := stringValue(input["user_id"])
		changed := map[string]any{}
		for k, v := range task {
			changed[k] = v
		}
		changed["created_by"] = newOwner
		if _, err := taskAuthority(ctx, s, changed); err != nil {
			writeError(w, 400, "新负责人必须是具有当前应用发布权限的工作区成员")
			return
		}
		saved, err := s.PB.Update(ctx, "miao_tasks", stringValue(task["id"]), map[string]any{"created_by": newOwner, "status": "draft", "revision": intValue(task["revision"]) + 1, "next_run_at": "", "pause_reason": "负责人已转交，等待新负责人重新审阅授权"})
		if err != nil {
			writeError(w, 503, "负责人转交失败")
			return
		}
		writeJSON(w, 200, saved)
	case "events":
		trigger := asMap(asMap(task["definition"])["trigger"])
		if task["status"] != "enabled" || trigger["type"] != "manual" || intValue(input["expected_revision"]) != intValue(task["revision"]) {
			writeError(w, 409, "外部事件只能触发已启用的当前手动任务版本")
			return
		}
		eventID := stringValue(input["event_id"])
		eventInput := asMap(input["input"])
		encoded, _ := json.Marshal(eventInput)
		if !requestIDPattern.MatchString(eventID) || len(encoded) > 16000 {
			writeError(w, 400, "事件标识或输入无效")
			return
		}
		if _, err := taskAuthority(ctx, s, task); err != nil {
			writeError(w, 403, "任务负责人已失去权限或应用已归档")
			return
		}
		run, err := enqueueTaskRun(ctx, s, task, "external:"+strconv.Itoa(intValue(task["revision"]))+":"+eventID, eventInput)
		if err != nil {
			writeError(w, 503, "事件运行创建失败")
			return
		}
		writeJSON(w, 202, publicRun(run))
	default:
		writeError(w, 404, "任务操作不存在")
	}
}

func pathAction(r *http.Request) string {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func previewTask(task map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range task {
		out[k] = v
	}
	definition := asMap(task["definition"])
	copied := map[string]any{}
	for k, v := range definition {
		copied[k] = v
	}
	scope := asMap(definition["scope"])
	scopeCopy := map[string]any{}
	for k, v := range scope {
		scopeCopy[k] = v
	}
	scopeCopy["recipient_ids"] = []any{}
	grants := []map[string]any{}
	for _, grant := range asSliceMap(scope["tables"]) {
		g := map[string]any{}
		for k, v := range grant {
			g[k] = v
		}
		g["write_fields"] = []any{}
		grants = append(grants, g)
	}
	scopeCopy["tables"] = grants
	copied["scope"], copied["mode"] = scopeCopy, "preview"
	limits := asMap(definition["limits"])
	limitCopy := map[string]any{}
	for k, v := range limits {
		limitCopy[k] = v
	}
	limitCopy["max_writes"] = 0
	copied["limits"] = limitCopy
	out["definition"] = copied
	return out
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		writeError(w, 404, "应用不存在或你没有访问权限")
		return
	}
	page := queryInt(r, "page", 1, 1, 10000)
	rows, total, _, err := s.PB.List(ctx, "miao_runs", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "-created", page, 25)
	if err != nil {
		writeError(w, 503, "运行记录暂不可用")
		return
	}
	items := []map[string]any{}
	for _, row := range rows {
		items = append(items, publicRun(row))
	}
	writeJSON(w, 200, pageResult(items, page, 25, total))
}

func (s *Server) ownedRun(ctx context.Context, r *http.Request, manage bool) (map[string]any, bool) {
	app, _, err := s.appForRequest(ctx, r)
	if err != nil {
		return nil, false
	}
	run, err := s.PB.Get(ctx, "miao_runs", pathID(r, "runId"))
	id := who(r)
	if err != nil || run["tenant_id"] != id.Tenant["id"] || run["app_id"] != app["id"] {
		return nil, false
	}
	if manage && run["created_by"] != id.User["id"] && s.appPermission(ctx, app, id) != "owner" {
		return nil, false
	}
	return run, true
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	run, ok := s.ownedRun(ctx, r, false)
	if !ok {
		writeError(w, 404, "运行不存在")
		return
	}
	actions, err := s.PB.ListAll(ctx, "miao_actions", listFilter("tenant_id = "+pbFilterString(stringValue(run["tenant_id"])), "app_id = "+pbFilterString(stringValue(run["app_id"])), "run_id = "+pbFilterString(stringValue(run["id"]))), "created")
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	attempts, err := s.PB.ListAll(ctx, "miao_run_attempts", "run_id = "+pbFilterString(stringValue(run["id"])), "sequence")
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	out := publicRun(run)
	out["actions"], out["attempt_history"] = actions, attempts
	writeJSON(w, 200, out)
}

func (s *Server) runAction(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	action := pathAction(r)
	run, ok := s.ownedRun(ctx, r, true)
	if !ok {
		writeError(w, 404, "运行不存在")
		return
	}
	input := mapBody(r)
	unlock := lockJob(stringValue(run["id"]))
	defer unlock()
	var err error
	run, err = s.PB.Get(ctx, "miao_runs", stringValue(run["id"]))
	if err != nil {
		s.writeBusinessError(w, err)
		return
	}
	switch action {
	case "cancel":
		if !containsString([]string{"queued", "running", "waiting"}, stringValue(run["status"])) {
			writeError(w, 409, "此运行已经结束")
			return
		}
		updates := map[string]any{"cancel_requested": true}
		if run["status"] != "running" {
			updates["status"], updates["finished_at"] = "cancelled", nowISO()
		}
		saved, err := s.PB.Update(ctx, "miao_runs", stringValue(run["id"]), updates)
		if err != nil {
			writeError(w, 503, "运行取消失败")
			return
		}
		s.cancelRun(stringValue(run["id"]))
		writeJSON(w, 200, publicRun(saved))
	case "retry":
		if !containsString([]string{"failed", "partial"}, stringValue(run["status"])) || intValue(run["retry_count"]) >= 2 || input["confirm"] != true {
			writeError(w, 409, "只有失败或部分完成的运行可确认重试，最多三次尝试")
			return
		}
		if _, err := taskAuthority(ctx, s, run); err != nil {
			writeError(w, 403, "任务负责人已失去权限或应用已归档")
			return
		}
		unknown, _, _, _ := s.PB.List(ctx, "miao_actions", listFilter("run_id = "+pbFilterString(stringValue(run["id"])), "(status = \"executing\" || status = \"unknown\")"), "", 1, 1)
		if len(unknown) > 0 {
			writeError(w, 409, "存在待核实写入，不能直接重试")
			return
		}
		attempts, err := s.PB.ListAll(ctx, "miao_run_attempts", "run_id = "+pbFilterString(stringValue(run["id"])), "sequence")
		if err != nil {
			s.writeBusinessError(w, err)
			return
		}
		usedRequests := 0
		for _, attempt := range attempts {
			usedRequests += intValue(attempt["model_requests"])
		}
		snapshot := asMap(run["snapshot"])
		limits := asMap(snapshot["limits"])
		remainingRequests := intValue(limits["max_requests"]) - usedRequests
		if remainingRequests < 1 {
			writeError(w, 409, "原运行的模型请求预算已用完，请调整并重新确认任务版本")
			return
		}
		retrySnapshot := map[string]any{}
		for key, value := range snapshot {
			retrySnapshot[key] = value
		}
		retryLimits := map[string]any{}
		for key, value := range limits {
			retryLimits[key] = value
		}
		retryLimits["max_requests"] = remainingRequests
		retrySnapshot["limits"], retrySnapshot["retry_of"] = retryLimits, run["id"]
		delivery := "pending"
		if snapshot["mode"] == "preview" {
			delivery = "suppressed"
		}
		eventKey := fmt.Sprintf("retry:%s:%d", stringValue(run["id"]), intValue(run["retry_count"])+1)
		saved, err := s.PB.Create(ctx, "miao_runs", map[string]any{"tenant_id": run["tenant_id"], "app_id": run["app_id"], "task_id": run["task_id"], "created_by": run["created_by"], "event_key": eventKey, "snapshot": retrySnapshot, "status": "queued", "attempts": 0, "model_requests": 0, "retry_count": intValue(run["retry_count"]) + 1, "delivery_status": delivery})
		if err != nil {
			writeError(w, 503, "重试运行创建失败")
			return
		}
		writeJSON(w, 200, publicRun(saved))
	case "resolve":
		pending := asMap(run["pending"])
		if run["status"] != "waiting" || input["expected_updated_at"] != run["updated"] || input["confirm"] != true {
			writeError(w, 409, "待处理内容已变化，请刷新后重新确认")
			return
		}
		if _, err := taskAuthority(ctx, s, run); err != nil {
			writeError(w, 403, "任务负责人已失去权限或应用已归档")
			return
		}
		if !containsString([]string{"approval", "information", "uncertain"}, stringValue(pending["kind"])) {
			writeError(w, 409, "此待处理事项已失效")
			return
		}
		if !parseTime(pending["expires_at"]).After(time.Now()) {
			writeError(w, 409, "确认已过期，不能继续执行")
			return
		}
		if input["decision"] == "reject" {
			saved, err := s.PB.Update(ctx, "miao_runs", stringValue(run["id"]), map[string]any{"status": "cancelled", "error": "用户拒绝本次待处理事项", "finished_at": nowISO()})
			if err != nil {
				writeError(w, 503, "确认结果保存失败")
				return
			}
			writeJSON(w, 200, publicRun(saved))
			return
		}
		if input["decision"] != "approve" {
			writeError(w, 400, "请选择批准或拒绝")
			return
		}
		if pending["kind"] == "information" {
			checkpoint := asMap(run["checkpoint"])
			messages := anySlice(checkpoint["messages"])
			if len(messages) > 0 {
				last := asMap(messages[len(messages)-1])
				if stringValue(last["role"]) == "tool" {
					writeError(w, 409, "任务检查点已在等待后继续推进，请刷新运行")
					return
				}
			}
			answer := strings.TrimSpace(stringValue(input["answer"]))
			if answer == "" || len([]rune(answer)) > 6000 {
				writeError(w, 400, "请提供不超过 6000 字的补充信息")
				return
			}
			pending["answer"], pending["answered_by"] = answer, who(r).User["id"]
			saved, err := s.PB.Update(ctx, "miao_runs", stringValue(run["id"]), map[string]any{"status": "queued", "pending": pending, "error": ""})
			if err != nil {
				writeError(w, 503, "补充信息保存失败")
				return
			}
			writeJSON(w, 200, publicRun(saved))
			return
		}
		effect, err := s.PB.Get(ctx, "miao_actions", stringValue(pending["action_id"]))
		if err != nil || effect["run_id"] != run["id"] || effect["app_id"] != run["app_id"] || effect["tenant_id"] != run["tenant_id"] {
			writeError(w, 409, "动作与运行不匹配")
			return
		}
		effectInput := asMap(effect["input"])
		table, err := s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(run["tenant_id"])), "app_id = "+pbFilterString(stringValue(run["app_id"])), "slug = "+pbFilterString(stringValue(effectInput["table"]))))
		if err != nil {
			writeError(w, 409, "数据表已失效")
			return
		}
		row, err := s.PB.Get(ctx, stringValue(table["pb_collection"]), stringValue(effectInput["record_id"]))
		if err != nil || row["tenant_id"] != run["tenant_id"] || row["app_id"] != run["app_id"] {
			writeError(w, 404, "记录已失效")
			return
		}
		if pending["kind"] == "uncertain" {
			for key, value := range asMap(effectInput["data"]) {
				if !equalJSON(row[key], value) {
					writeError(w, 409, "当前记录与预期写入不一致，请拒绝本次运行并基于当前数据建立新任务")
					return
				}
			}
			result := taskVisibleRecord(row, findTaskGrant(asMap(run["snapshot"]), stringValue(effectInput["table"])))
			_, err = s.PB.Update(ctx, "miao_actions", stringValue(effect["id"]), map[string]any{"status": "done", "result": result, "evidence": effect["evidence"]})
		} else {
			if stringValue(row["updated"]) != stringValue(effectInput["expected_updated_at"]) {
				writeError(w, 409, "目标记录已变化，此预览不可批准；请拒绝并基于当前数据重新处理")
				return
			}
			_, err = s.PB.Update(ctx, "miao_actions", stringValue(effect["id"]), map[string]any{"status": "approved", "approved_by": who(r).User["id"], "approved_at": nowISO()})
		}
		if err != nil {
			writeError(w, 503, "确认结果保存失败")
			return
		}
		pending["resolved_by"], pending["decision"] = who(r).User["id"], "approved"
		saved, err := s.PB.Update(ctx, "miao_runs", stringValue(run["id"]), map[string]any{"status": "queued", "pending": pending, "error": ""})
		if err != nil {
			writeError(w, 503, "运行恢复失败")
			return
		}
		writeJSON(w, 200, publicRun(saved))
	default:
		writeError(w, 404, "运行操作不存在")
	}
}

func findTaskGrant(snapshot map[string]any, table string) map[string]any {
	for _, grant := range asSliceMap(asMap(snapshot["scope"])["tables"]) {
		if grant["table"] == table {
			return grant
		}
	}
	return nil
}

func taskVisibleRecord(row, grant map[string]any) map[string]any {
	data := map[string]any{}
	for _, name := range anySlice(grant["read_fields"]) {
		data[stringValue(name)] = row[stringValue(name)]
	}
	return map[string]any{"id": row["id"], "updated_at": row["updated"], "data": data}
}

func (s *Server) cancelRun(id string) {
	s.workerMu.Lock()
	defer s.workerMu.Unlock()
	if s.activeRun == id && s.activeCancel != nil {
		s.activeCancel()
	}
}

func enqueueTaskRun(ctx context.Context, s *Server, task map[string]any, eventKey string, input any) (map[string]any, error) {
	if existing, err := s.PB.Find(ctx, "miao_runs", listFilter("task_id = "+pbFilterString(stringValue(task["id"])), "event_key = "+pbFilterString(eventKey))); err == nil {
		return existing, nil
	}
	definition := asMap(task["definition"])
	snapshot := map[string]any{}
	for k, v := range definition {
		snapshot[k] = v
	}
	snapshot["name"], snapshot["revision"], snapshot["input"] = task["name"], task["revision"], input
	delivery := "pending"
	if definition["mode"] == "preview" {
		delivery = "suppressed"
	}
	run, err := s.PB.Create(ctx, "miao_runs", map[string]any{"tenant_id": task["tenant_id"], "app_id": task["app_id"], "task_id": task["id"], "created_by": task["created_by"], "event_key": eventKey, "snapshot": snapshot, "status": "queued", "attempts": 0, "model_requests": 0, "retry_count": 0, "delivery_status": delivery})
	if err == nil {
		return run, nil
	}
	if duplicate, e := s.PB.Find(ctx, "miao_runs", listFilter("task_id = "+pbFilterString(stringValue(task["id"])), "event_key = "+pbFilterString(eventKey))); e == nil {
		return duplicate, nil
	}
	return nil, err
}

func taskAuthority(ctx context.Context, s *Server, task map[string]any) (map[string]any, error) {
	return s.authorizeWrite(ctx, s.PB, executionActor{UserID: stringValue(task["created_by"]), TenantID: stringValue(task["tenant_id"]), AppID: stringValue(task["app_id"]), Source: "background"}, false)
}
