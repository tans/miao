package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	"github.com/tans/miao/internal/pocketbase"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tans/miao/internal/runtime"
)

type integrationFixture struct {
	t                                          *testing.T
	runtime                                    *runtime.Runtime
	api                                        *Server
	root, token, userID, tenantID, base, table string
}

func newIntegration(t *testing.T) *integrationFixture {
	t.Helper()
	for _, key := range []string{"POCKETBASE_SUPERUSER_EMAIL", "POCKETBASE_SUPERUSER_PASSWORD", "MIAO_SETTINGS_ENCRYPTION_KEY", "MIAO_ALLOWED_EMAIL_DOMAINS", "MIAO_ADMIN_EMAILS", "RESEND_API_KEY", "MIAO_REQUIRE_EMAIL_VERIFICATION"} {
		t.Setenv(key, "")
	}
	t.Setenv("MIAO_REGISTRATION_MODE", "open")
	f := &integrationFixture{t: t, root: t.TempDir()}
	f.open()
	t.Cleanup(func() {
		if f.runtime != nil {
			if err := f.runtime.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	registered := f.request("", "POST", "/api/auth/register", map[string]any{"name": "Owner", "email": "owner@example.invalid", "password": "Smoke-Password-2026"}, 201)
	f.token, f.userID, f.tenantID = stringValue(registered["token"]), stringValue(asMap(registered["user"])["id"]), stringValue(asMap(registered["tenant"])["id"])
	app := f.request(f.token, "POST", "/api/apps", map[string]any{"name": "客户项目"}, 201)
	f.base = "/api/apps/" + stringValue(app["id"])
	f.request(f.token, "POST", f.base+"/collections", map[string]any{"name": "客户", "slug": "customers", "fields": []any{map[string]any{"name": "name", "type": "text", "required": true}, map[string]any{"name": "status", "type": "select", "options": []any{"new", "done"}}, map[string]any{"name": "file", "type": "file"}}}, 201)
	metadata, err := f.api.PB.Find(context.Background(), "app_collections", "app_id = "+pbFilterString(stringValue(app["id"])))
	if err != nil {
		t.Fatal(err)
	}
	f.table = stringValue(metadata["pb_collection"])
	return f
}
func (f *integrationFixture) open() {
	f.t.Helper()
	r, err := runtime.New(f.root)
	if err != nil {
		f.t.Fatal(err)
	}
	f.runtime = r
	f.api = New(r.Store)
}
func (f *integrationFixture) restart() {
	f.t.Helper()
	if err := f.runtime.Close(); err != nil {
		f.t.Fatal(err)
	}
	f.runtime = nil
	f.open()
}
func (f *integrationFixture) response(token, method, url string, body any, expected int) *httptest.ResponseRecorder {
	f.t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		f.t.Fatal(err)
	}
	req := httptest.NewRequest(method, url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	f.api.ServeHTTP(response, req)
	if response.Code != expected {
		f.t.Fatalf("%s %s: status=%d expected=%d body=%s", method, url, response.Code, expected, response.Body.String())
	}
	return response
}
func (f *integrationFixture) request(token, method, url string, body any, expected int) map[string]any {
	f.t.Helper()
	response := f.response(token, method, url, body, expected)
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		f.t.Fatal(err)
	}
	return result
}
func (f *integrationFixture) row(name string) map[string]any {
	f.t.Helper()
	return f.request(f.token, "POST", f.base+"/collections/customers/records", map[string]any{"data": map[string]any{"name": name, "status": "new"}}, 201)
}
func (f *integrationFixture) member(role string, batch bool) string {
	f.t.Helper()
	ctx := context.Background()
	pb := f.api.PB
	user, err := pb.Create(ctx, "users", map[string]any{"name": role, "email": role + "@example.invalid", "password": "Smoke-Password-2026", "passwordConfirm": "Smoke-Password-2026", "verified": true})
	if err != nil {
		f.t.Fatal(err)
	}
	for collection, body := range map[string]map[string]any{"tenant_members": {"tenant_id": f.tenantID, "user_id": user["id"], "role": "member"}, "app_members": {"tenant_id": f.tenantID, "app_id": f.base[len("/api/apps/"):], "user_id": user["id"], "role": role, "can_batch": batch}} {
		if _, err := pb.Create(ctx, collection, body); err != nil {
			f.t.Fatal(err)
		}
	}
	login := f.request("", "POST", "/api/auth/login", map[string]any{"email": role + "@example.invalid", "password": "Smoke-Password-2026"}, 200)
	return stringValue(login["token"])
}
func (f *integrationFixture) definition() map[string]any {
	return map[string]any{"schema_version": 2, "title": "业务", "pages": []any{map[string]any{"id": "customers", "title": "客户", "collection": "customers", "fields": []any{"name", "status", "file"}, "actions": []any{map[string]any{"id": "complete", "label": "完成", "set": map[string]any{"status": "done"}}}}}}
}
func TestGoBusinessLifecycle(t *testing.T) {
	f := newIntegration(t)
	f.request("", "GET", f.base, nil, 401)
	f.request("invalid", "GET", f.base, nil, 401)
	version := f.request(f.token, "POST", f.base+"/versions", map[string]any{"definition": f.definition()}, 201)
	f.request(f.token, "POST", f.base+"/versions/"+stringValue(version["id"])+"/publish", map[string]any{"expected_published_version_id": nil}, 200)
	file := f.request(f.token, "POST", f.base+"/files", map[string]any{"name": "客户.csv", "base64": base64.StdEncoding.EncodeToString([]byte("name,status\n张三,new\n李四,new"))}, 201)
	content := f.request(f.token, "GET", f.base+"/files/"+stringValue(file["id"])+"/content", nil, 200)
	plan := f.request(f.token, "POST", f.base+"/import-plans", map[string]any{"table": "customers", "rows": content["rows"]}, 201)
	commit := f.base + "/import-plans/" + stringValue(plan["plan_id"]) + "/commit"
	f.request(f.token, "POST", commit, map[string]any{"confirm": false}, 400)
	for i := 0; i < 2; i++ {
		result := f.request(f.token, "POST", commit, map[string]any{"confirm": true, "plan_id": plan["plan_id"]}, 200)
		if intValue(asMap(result["result"])["created"]) != 2 {
			t.Fatal(result)
		}
	}
	runtime := f.request(f.token, "GET", f.base+"/runtime", nil, 200)
	if intValue(runtime["total_items"]) != 2 {
		t.Fatal(runtime)
	}
	row := asMap(anySlice(runtime["items"])[0])
	path := f.base + "/collections/customers/records/" + stringValue(row["id"])
	current := f.request(f.token, "GET", path, nil, 200)
	action := map[string]any{"ui_page": "customers", "record_id": row["id"], "expected_updated_at": current["updated_at"], "expected_version_id": version["id"], "confirm": true}
	completed := f.request(f.token, "POST", f.base+"/runtime/actions/complete", action, 200)
	f.request(f.token, "POST", f.base+"/runtime/actions/complete", action, 409)
	changes := f.request(f.token, "GET", f.base+"/record-changes?record_id="+stringValue(row["id"]), nil, 200)
	change := asMap(anySlice(changes["items"])[0])
	if change["actor_id"] != f.userID || asMap(change["before"])["status"] != "new" || asMap(change["after"])["status"] != "done" {
		t.Fatal(change)
	}
	f.request(f.token, "POST", f.base+"/record-changes/"+stringValue(change["id"])+"/restore", map[string]any{"confirm": true, "expected_updated_at": asMap(completed["record"])["updated_at"]}, 200)
	attachment := f.request(f.token, "POST", f.base+"/files", map[string]any{"name": "notes.txt", "base64": base64.StdEncoding.EncodeToString([]byte("交付说明"))}, 201)
	current = f.request(f.token, "GET", path, nil, 200)
	attached := f.request(f.token, "POST", f.base+"/files/"+stringValue(attachment["id"])+"/attach", map[string]any{"table": "customers", "record_id": row["id"], "field": "file", "expected_updated_at": current["updated_at"]}, 200)
	if stringValue(asMap(attached["data"])["file"]) == "" {
		t.Fatal(attached)
	}
	download := f.response(f.token, "GET", path+"/files/file", nil, 200)
	if download.Body.String() != "交付说明" {
		t.Fatal(download.Body.String())
	}
	f.request(f.token, "PUT", f.base+"/context", map[string]any{"content": "客户交付", "expected_revision": 0}, 200)
	f.request(f.token, "PUT", f.base+"/context", map[string]any{"content": "stale", "expected_revision": 0}, 409)
	scope := f.api.conversationScope(context.Background(), identity{User: map[string]any{"id": f.userID}, Tenant: map[string]any{"id": f.tenantID, "owner_id": f.userID}, Membership: map[string]any{"role": "owner"}})
	f.request(f.token, "PUT", "/api/agent/conversation", map[string]any{"scope": scope, "expected_revision": 0, "checkpoint": "AQID", "messages": []any{map[string]any{"role": "user", "content": "继续交付"}}}, 200)
	f.restart()
	if intValue(f.request(f.token, "GET", f.base+"/runtime", nil, 200)["total_items"]) != 2 {
		t.Fatal("records lost after restart")
	}
	saved := f.request(f.token, "GET", "/api/agent/conversation", nil, 200)
	if asMap(saved["conversation"])["checkpoint"] != "AQID" {
		t.Fatal(saved)
	}
	f.request(f.token, "GET", f.base+"/files/"+stringValue(file["id"])+"/content", nil, 200)
	f.response(f.token, "GET", path+"/files/file", nil, 200)
}
func TestGoPermissionsAndDeniedWrites(t *testing.T) {
	f := newIntegration(t)
	row := f.row("张三")
	for _, role := range []string{"viewer", "editor", "publisher"} {
		t.Run(role, func(t *testing.T) {
			token := f.member(role, false)
			expected := 403
			if role == "publisher" {
				expected = 201
			}
			f.request(token, "POST", f.base+"/versions", map[string]any{"definition": f.definition()}, expected)
			f.request(token, "POST", f.base+"/batch-plans", map[string]any{"table": "customers", "conditions": []any{}, "change": map[string]any{"field": "status", "value": "done"}}, 403)
			f.request(token, "DELETE", f.base, map[string]any{"confirm": true}, 403)
			if role == "viewer" {
				f.request(token, "PATCH", f.base+"/collections/customers/records/"+stringValue(row["id"]), map[string]any{"data": map[string]any{"status": "done"}, "expected_updated_at": row["updated_at"]}, 403)
			}
			if f.request(token, "GET", "/api/agent/conversation", nil, 200)["conversation"] != nil {
				t.Fatal("private conversation exposed")
			}
		})
	}
	outsider := f.request("", "POST", "/api/auth/register", map[string]any{"name": "Other", "email": "other@example.invalid", "password": "Smoke-Password-2026"}, 201)
	f.request(stringValue(outsider["token"]), "GET", f.base, nil, 404)
	f.request(stringValue(outsider["token"]), "DELETE", f.base, map[string]any{"confirm": true}, 404)
	current := f.request(f.token, "GET", f.base+"/collections/customers/records/"+stringValue(row["id"]), nil, 200)
	if asMap(current["data"])["status"] != "new" {
		t.Fatal("denied request changed data")
	}
	_, count, _, err := f.api.PB.List(context.Background(), "miao_record_changes", "", "", 1, 10)
	if err != nil || count != 0 {
		t.Fatalf("denied request created audit: %d %v", count, err)
	}
	// Raw PocketBase routes are private even for a logged-in business owner.
	for _, url := range []string{"/api/collections/users/records", "/api/files/token", "/api/backups"} {
		f.request(f.token, http.MethodGet, url, nil, 404)
	}
}
func TestGoBatchConfirmationAndRevocation(t *testing.T) {
	f := newIntegration(t)
	row := f.row("张三")
	plan := f.request(f.token, "POST", f.base+"/batch-plans", map[string]any{"table": "customers", "conditions": []any{}, "change": map[string]any{"field": "status", "value": "done"}}, 201)
	endpoint := fmt.Sprintf("%s/batch-plans/%s/commit", f.base, plan["plan_id"])
	f.request(f.token, "POST", endpoint, map[string]any{"confirm": false, "plan_id": plan["plan_id"]}, 400)
	for i := 0; i < 2; i++ {
		result := f.request(f.token, "POST", endpoint, map[string]any{"confirm": true, "plan_id": plan["plan_id"]}, 200)
		if intValue(asMap(result["result"])["updated"]) != 1 {
			t.Fatal(result)
		}
	}
	current := f.request(f.token, "GET", f.base+"/collections/customers/records/"+stringValue(row["id"]), nil, 200)
	if asMap(current["data"])["status"] != "done" {
		t.Fatal(current)
	}
	token := f.member("publisher", true)
	plan = f.request(token, "POST", f.base+"/batch-plans", map[string]any{"table": "customers", "conditions": []any{}, "change": map[string]any{"field": "status", "value": "new"}}, 201)
	member, err := f.api.PB.Find(context.Background(), "app_members", `role = "publisher"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.api.PB.Update(context.Background(), "app_members", stringValue(member["id"]), map[string]any{"can_batch": false}); err != nil {
		t.Fatal(err)
	}
	f.request(token, "POST", fmt.Sprintf("%s/batch-plans/%s/commit", f.base, plan["plan_id"]), map[string]any{"confirm": true, "plan_id": plan["plan_id"]}, 403)
}

func (f *integrationFixture) task(trigger map[string]any) map[string]any {
	f.t.Helper()
	task := f.request(f.token, "POST", f.base+"/tasks", map[string]any{"name": "客户检查", "definition": map[string]any{"goal": "读取客户数据", "execution": "report", "trigger": trigger, "scope": map[string]any{"tables": []any{map[string]any{"table": "customers", "read_fields": []any{"name", "status"}, "write_fields": []any{}}}}}}, 201)
	f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/enable", map[string]any{"confirm": true, "expected_revision": 1}, 200)
	return task
}
func TestGoTaskEventsAndReportRestart(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	task := f.task(map[string]any{"type": "record_created", "table": "customers"})
	row := f.row("张三")
	runs, err := f.api.PB.ListAll(ctx, "miao_runs", "task_id = "+pbFilterString(stringValue(task["id"])), "")
	if err != nil || len(runs) != 1 {
		t.Fatalf("one record must enqueue one run: %v %v", runs, err)
	}
	runID := stringValue(runs[0]["id"])
	f.restart()
	lease, ok := f.api.ensureWorkerLease(ctx)
	if !ok {
		t.Fatal("lease not acquired")
	}
	f.api.executeTaskRun(ctx, runs[0], lease)
	result := f.request(f.token, "GET", f.base+"/runs/"+runID, nil, 200)
	if result["status"] != "completed" || !strings.Contains(stringValue(result["output"]), "张三") {
		t.Fatal(result)
	}
	if _, ok := result["checkpoint"]; ok {
		t.Fatal("checkpoint exposed")
	}
	if _, ok := result["snapshot"]; ok {
		t.Fatal("snapshot exposed")
	}
	attempts := asSliceMap(result["attempt_history"])
	if len(attempts) != 1 || attempts[0]["status"] != "completed" {
		t.Fatal(result)
	}
	current := f.request(f.token, "GET", f.base+"/collections/customers/records/"+stringValue(row["id"]), nil, 200)
	if _, err := f.api.PB.UpdateBusiness(ctx, f.table, stringValue(row["id"]), map[string]any{"status": "done"}, stringValue(current["updated_at"]), f.userID, "background"); err != nil {
		t.Fatal(err)
	}
	_, total, _, err := f.api.PB.List(ctx, "miao_runs", "", "", 1, 10)
	if err != nil || total != 1 {
		t.Fatalf("background write recursed: %d %v", total, err)
	}
	history, err := f.api.PB.ListAll(ctx, "miao_record_changes", "", "")
	if err != nil || len(history) != 1 || history[0]["source"] != "background" {
		t.Fatalf("background audit missing: %v %v", history, err)
	}
	manual := f.task(map[string]any{"type": "manual"})
	endpoint := f.base + "/tasks/" + stringValue(manual["id"]) + "/events"
	first := f.request(f.token, "POST", endpoint, map[string]any{"event_id": "external-1", "expected_revision": 1, "input": map[string]any{"source": "external"}}, 202)
	repeated := f.request(f.token, "POST", endpoint, map[string]any{"event_id": "external-1", "expected_revision": 1}, 202)
	if first["id"] != repeated["id"] {
		t.Fatal("external event was duplicated")
	}
}
func TestGoScheduleRetriesFailedEnqueue(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	task := f.task(map[string]any{"type": "once", "at": time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
	due := time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano)
	if _, err := f.api.PB.Update(ctx, "miao_tasks", stringValue(task["id"]), map[string]any{"next_run_at": due}); err != nil {
		t.Fatal(err)
	}
	hookID := f.runtime.App.OnRecordCreate("miao_runs").BindFunc(func(e *core.RecordEvent) error { return errors.New("injected enqueue failure") })
	f.api.scheduleDueTasks(ctx)
	f.runtime.App.OnRecordCreate("miao_runs").Unbind(hookID)
	stored, err := f.api.PB.Get(ctx, "miao_tasks", stringValue(task["id"]))
	if err != nil || stored["next_run_at"] != due {
		t.Fatalf("failed enqueue consumed schedule: %v %v", stored, err)
	}
	f.api.scheduleDueTasks(ctx)
	runs, err := f.api.PB.ListAll(ctx, "miao_runs", "task_id = "+pbFilterString(stringValue(task["id"])), "")
	if err != nil || len(runs) != 1 || asMap(asMap(runs[0]["snapshot"])["input"])["scheduled_at"] != due {
		t.Fatalf("due run was not retried: %v %v", runs, err)
	}
	f.api.scheduleDueTasks(ctx)
	runs, err = f.api.PB.ListAll(ctx, "miao_runs", "task_id = "+pbFilterString(stringValue(task["id"])), "")
	if err != nil || len(runs) != 1 {
		t.Fatalf("scheduled run duplicated: %v %v", runs, err)
	}
}
func TestGoAtomicRecordEventsAndAuditRollback(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	_, err := f.api.PB.Create(ctx, f.table, map[string]any{"tenant_id": f.tenantID, "app_id": f.base[len("/api/apps/"):]})
	var invalid *pocketbase.Error
	if !errors.As(err, &invalid) || invalid.Status != 400 || invalid.Data["name"] == nil {
		t.Fatalf("validation failure must be a definite rejection: %v", err)
	}
	f.task(map[string]any{"type": "record_created", "table": "customers"})
	hookID := f.runtime.App.OnRecordCreate("miao_runs").BindFunc(func(e *core.RecordEvent) error { return errors.New("injected enqueue failure") })
	f.request(f.token, "POST", f.base+"/collections/customers/records", map[string]any{"data": map[string]any{"name": "rollback", "status": "new"}}, 503)
	f.runtime.App.OnRecordCreate("miao_runs").Unbind(hookID)
	_, count, _, err := f.api.PB.List(ctx, f.table, "", "", 1, 10)
	if err != nil || count != 0 {
		t.Fatalf("record committed without its event: %d %v", count, err)
	}
	row := f.row("张三")
	f.task(map[string]any{"type": "status_changed", "table": "customers", "field": "status", "from": "new", "to": "done"})
	hookID = f.runtime.App.OnRecordCreate("miao_record_changes").BindFunc(func(e *core.RecordEvent) error { return errors.New("injected audit failure") })
	_, err = f.api.PB.UpdateBusiness(ctx, f.table, stringValue(row["id"]), map[string]any{"status": "done"}, stringValue(row["updated_at"]), f.userID, "interactive")
	if err == nil {
		t.Fatal("failed audit was ignored")
	}
	f.runtime.App.OnRecordCreate("miao_record_changes").Unbind(hookID)
	stored, err := f.api.PB.Get(ctx, f.table, stringValue(row["id"]))
	if err != nil || stored["status"] != "new" || stored["updated"] != row["updated_at"] {
		t.Fatalf("update escaped rollback: %v %v", stored, err)
	}
	_, count, _, err = f.api.PB.List(ctx, "miao_runs", "", "", 1, 10)
	if err != nil || count != 1 {
		t.Fatalf("failed update queued a run: %d %v", count, err)
	}
	_, err = f.api.PB.UpdateBusiness(ctx, f.table, stringValue(row["id"]), map[string]any{"tenant_id": "other"}, stringValue(row["updated_at"]), f.userID, "interactive")
	var denied *pocketbase.Error
	if !errors.As(err, &denied) || denied.Status != 403 {
		t.Fatalf("tenant move accepted: %v", err)
	}
}
func TestGoConcurrentRecordUpdates(t *testing.T) {
	f := newIntegration(t)
	row := f.row("张三")
	ctx := context.Background()
	f.task(map[string]any{"type": "status_changed", "table": "customers", "field": "status", "from": "new", "to": "done"})
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			_, err := f.api.PB.UpdateBusiness(ctx, f.table, stringValue(row["id"]), map[string]any{"status": "done"}, stringValue(row["updated_at"]), f.userID, "interactive")
			results <- err
		}()
	}
	close(start)
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else {
			var e *pocketbase.Error
			if errors.As(err, &e) && e.Status == 409 {
				conflict++
			} else {
				t.Fatal(err)
			}
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("lost-update protection failed: successes=%d conflicts=%d", success, conflict)
	}
	for _, collection := range []string{"miao_record_changes", "miao_runs"} {
		_, count, _, err := f.api.PB.List(ctx, collection, "", "", 1, 10)
		if err != nil || count != 1 {
			t.Fatalf("%s: %d %v", collection, count, err)
		}
	}
}
func TestGoInterruptedWriteWaitsForReview(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	task := f.task(map[string]any{"type": "manual"})
	run := f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/events", map[string]any{"event_id": "interrupted", "expected_revision": 1}, 202)
	if _, err := f.api.PB.Update(ctx, "miao_runs", stringValue(run["id"]), map[string]any{"status": "running"}); err != nil {
		t.Fatal(err)
	}
	for collection, body := range map[string]map[string]any{
		"miao_run_attempts": {"tenant_id": f.tenantID, "app_id": task["app_id"], "run_id": run["id"], "sequence": 1, "status": "running"},
		"miao_actions":      {"tenant_id": f.tenantID, "app_id": task["app_id"], "run_id": run["id"], "action_key": "write-1", "tool": "update_record", "input": map[string]any{"table": "customers"}, "status": "executing", "evidence": map[string]any{"before": "known"}},
	} {
		if _, err := f.api.PB.Create(ctx, collection, body); err != nil {
			t.Fatal(err)
		}
	}
	f.restart()
	lease, ok := f.api.ensureWorkerLease(ctx)
	if !ok {
		t.Fatal("lease not acquired")
	}
	f.api.recoverInterruptedRuns(ctx, lease)
	stored, err := f.api.PB.Get(ctx, "miao_runs", stringValue(run["id"]))
	if err != nil || stored["status"] != "waiting" || asMap(stored["pending"])["kind"] != "uncertain" {
		t.Fatalf("unknown effect was replayed: %v %v", stored, err)
	}
	actions, err := f.api.PB.ListAll(ctx, "miao_actions", "", "")
	if err != nil || len(actions) != 1 || actions[0]["status"] != "unknown" {
		t.Fatalf("evidence was lost: %v %v", actions, err)
	}
}
func TestGoMigrationReapplyProtectsExistingFiles(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	schema, err := f.api.PB.Collection(ctx, f.table)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range asSliceMap(schema["fields"]) {
		if field["type"] == "file" {
			field["protected"] = false
		}
	}
	if _, err := f.api.PB.UpdateCollection(ctx, f.table, schema); err != nil {
		t.Fatal(err)
	}
	runner := core.NewMigrationsRunner(f.runtime.App, core.AppMigrations)
	if _, err := runner.Down(1); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Up(); err != nil {
		t.Fatal(err)
	}
	schema, err = f.api.PB.Collection(ctx, f.table)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range asSliceMap(schema["fields"]) {
		if field["type"] == "file" && field["protected"] != true {
			t.Fatal("existing attachment is public")
		}
	}
	if applied, err := runner.Up(); err != nil || len(applied) != 0 {
		t.Fatalf("migration ran twice: %v %v", applied, err)
	}
}

func TestGoFixedAutomationIsIdempotent(t *testing.T) {
	for _, action := range []bool{false, true} {
		t.Run(fmt.Sprint(action), func(t *testing.T) {
			f := newIntegration(t)
			ctx := context.Background()
			definition := map[string]any{"trigger": "record_created", "table": "customers", "recipient_id": f.userID}
			if action {
				definition["action"] = map[string]any{"type": "set_field", "field": "status", "value": "done"}
			}
			if _, err := f.api.PB.Create(ctx, "automation_rules", map[string]any{"tenant_id": f.tenantID, "app_id": f.base[len("/api/apps/"):], "created_by": f.userID, "name": "新增客户", "enabled": true, "definition": definition}); err != nil {
				t.Fatal(err)
			}
			row := f.row("张三")
			stored, err := f.api.PB.Get(ctx, f.table, stringValue(row["id"]))
			if err != nil {
				t.Fatal(err)
			}
			// Retry the exact committed event, not the updated action's new timestamp.
			source := map[string]any{"id": row["id"], "updated": row["updated_at"], "name": "张三", "status": "new"}
			f.api.processRecordAutomation(ctx, f.tenantID, f.base[len("/api/apps/"):], "customers", "created", nil, source)
			runs, err := f.api.PB.ListAll(ctx, "automation_runs", "", "")
			if err != nil || len(runs) != 1 || runs[0]["status"] != "delivered" {
				t.Fatalf("automation duplicated: %v %v", runs, err)
			}
			_, notifications, _, err := f.api.PB.List(ctx, "automation_notifications", "", "", 1, 10)
			want := 1
			if action {
				want = 0
				if stored["status"] != "done" {
					t.Fatal(stored)
				}
			}
			if err != nil || notifications != want {
				t.Fatalf("notifications=%d want=%d err=%v", notifications, want, err)
			}
		})
	}
}
func TestGoAppDeletionRollsBackAndKeepsOtherApps(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	f.row("张三")
	task := f.task(map[string]any{"type": "manual"})
	other := f.request(f.token, "POST", "/api/apps", map[string]any{"name": "保留应用"}, 201)
	ids := []string{f.base[len("/api/apps/"):], stringValue(other["id"])}
	threads := []map[string]any{}
	for _, appID := range ids {
		thread, err := f.api.PB.Create(ctx, "agent_threads", map[string]any{"tenant_id": f.tenantID, "app_id": appID, "user_id": f.userID, "title": "对话"})
		if err != nil {
			t.Fatal(err)
		}
		threads = append(threads, thread)
		if _, err := f.api.PB.Create(ctx, "agent_messages", map[string]any{"tenant_id": f.tenantID, "thread_id": thread["id"], "user_id": f.userID, "role": "user", "content": "保留隐私"}); err != nil {
			t.Fatal(err)
		}
	}
	f.request(f.token, "DELETE", f.base, map[string]any{"confirm": false}, 400)
	hookID := f.runtime.App.OnRecordDelete("apps").BindFunc(func(e *core.RecordEvent) error { return errors.New("injected deletion failure") })
	f.request(f.token, "DELETE", f.base, map[string]any{"confirm": true}, 503)
	f.runtime.App.OnRecordDelete("apps").Unbind(hookID)
	f.request(f.token, "GET", f.base+"/collections/customers/records", nil, 200)
	if _, err := f.api.PB.Get(ctx, "miao_tasks", stringValue(task["id"])); err != nil {
		t.Fatal("failed deletion removed task", err)
	}
	removed := f.request(f.token, "DELETE", f.base, map[string]any{"confirm": true}, 200)
	if intValue(removed["deleted_tables"]) != 1 || intValue(removed["deleted_records"]) != 1 {
		t.Fatal(removed)
	}
	f.request(f.token, "GET", "/api/apps/"+stringValue(other["id"]), nil, 200)
	if _, err := f.api.PB.Get(ctx, "miao_tasks", stringValue(task["id"])); err == nil {
		t.Fatal("deleted app retained task")
	}
	messages, err := f.api.PB.ListAll(ctx, "agent_messages", "", "")
	if err != nil || len(messages) != 1 || messages[0]["thread_id"] != threads[1]["id"] {
		t.Fatalf("message cleanup crossed app boundary: %v %v", messages, err)
	}
}
func TestGoPermissionLookupFailsClosed(t *testing.T) {
	f := newIntegration(t)
	token := f.member("editor", false)
	f.request(token, "GET", f.base, nil, 200)
	if err := f.api.PB.DeleteCollection(context.Background(), "app_members"); err != nil {
		t.Fatal(err)
	}
	f.request(token, "GET", f.base, nil, 404)
	f.request(token, "POST", f.base+"/collections/customers/records", map[string]any{"data": map[string]any{"name": "越权"}}, 404)
}
func TestGoRegistrationPolicyAndRollback(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	f.api.RegistrationMode = "closed"
	f.request("", "POST", "/api/auth/register", map[string]any{"name": "Closed", "email": "closed@example.invalid", "password": "Smoke-Password-2026"}, 403)
	f.api.RegistrationMode = "open"
	hookID := f.runtime.App.OnRecordCreate("tenant_members").BindFunc(func(e *core.RecordEvent) error { return errors.New("injected membership failure") })
	f.request("", "POST", "/api/auth/register", map[string]any{"name": "Rollback", "email": "rollback@example.invalid", "password": "Smoke-Password-2026"}, 503)
	f.runtime.App.OnRecordCreate("tenant_members").Unbind(hookID)
	if rows, err := f.api.PB.ListAll(ctx, "users", `email = "rollback@example.invalid"`, ""); err != nil || len(rows) != 0 {
		t.Fatalf("failed registration left a user: %v %v", rows, err)
	}
	_, count, _, err := f.api.PB.List(ctx, "tenants", "", "", 1, 10)
	if err != nil || count != 1 {
		t.Fatalf("failed registration left a workspace: %d %v", count, err)
	}
	f.request("", "POST", "/api/auth/login", map[string]any{"email": "owner@example.invalid", "password": "bad-password"}, 401)
	if _, err := f.api.PB.Update(ctx, "users", f.userID, map[string]any{"disabled": true}); err != nil {
		t.Fatal(err)
	}
	f.request(f.token, "GET", f.base, nil, 403)
	f.request("", "POST", "/api/auth/login", map[string]any{"email": "owner@example.invalid", "password": "Smoke-Password-2026"}, 403)
}

func TestGoWorkerCancellationSavesInterruption(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	task := f.task(map[string]any{"type": "manual"})
	run := f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/events", map[string]any{"event_id": "shutdown", "expected_revision": 1}, 202)
	lease, ok := f.api.ensureWorkerLease(ctx)
	if !ok {
		t.Fatal("lease not acquired")
	}
	workerContext, cancel := context.WithCancel(ctx)
	defer cancel()
	hookID := f.runtime.App.OnRecordAfterCreateSuccess("miao_run_attempts").BindFunc(func(e *core.RecordEvent) error { cancel(); return e.Next() })
	f.api.executeTaskRun(workerContext, run, lease)
	f.runtime.App.OnRecordAfterCreateSuccess("miao_run_attempts").Unbind(hookID)
	stored, err := f.api.PB.Get(ctx, "miao_runs", stringValue(run["id"]))
	if err != nil || stored["status"] != "queued" || stored["finished_at"] != "" {
		t.Fatalf("cancelled worker did not persist recovery state: %v %v", stored, err)
	}
	attempts, err := f.api.PB.ListAll(ctx, "miao_run_attempts", "", "")
	if err != nil || len(attempts) != 1 || attempts[0]["status"] != "interrupted" {
		t.Fatalf("interruption evidence lost: %v %v", attempts, err)
	}
	storedTask, err := f.api.PB.Get(ctx, "miao_tasks", stringValue(task["id"]))
	if err != nil || storedTask["status"] != "enabled" {
		t.Fatalf("shutdown revoked valid task authority: %v %v", storedTask, err)
	}
	f.api.StartBackground(ctx)
	stop, done := context.WithTimeout(ctx, 2*time.Second)
	defer done()
	if err := f.api.StopBackground(stop); err != nil {
		t.Fatal(err)
	}
}

// These regressions cover the transaction boundaries changed in issue #7.
func TestGoBusinessCommitBoundaries(t *testing.T) {
	t.Run("publication", func(t *testing.T) {
		f := newIntegration(t)
		version := f.request(f.token, "POST", f.base+"/versions", map[string]any{"definition": f.definition()}, 201)
		endpoint := f.base + "/versions/" + stringValue(version["id"]) + "/publish"
		f.request(f.token, "POST", endpoint, map[string]any{"expected_published_version_id": 123}, 400)
		hook := f.runtime.App.OnRecordUpdate("apps").BindFunc(func(e *core.RecordEvent) error { return errors.New("injected app save failure") })
		f.request(f.token, "POST", endpoint, map[string]any{"expected_published_version_id": nil}, 503)
		f.runtime.App.OnRecordUpdate("apps").Unbind(hook)
		stored, err := f.api.PB.Get(context.Background(), "app_versions", stringValue(version["id"]))
		if err != nil || stringValue(stored["published_at"]) != "" {
			t.Fatalf("failed publication marked version published: %v %v", stored, err)
		}
		f.request(f.token, "POST", endpoint, map[string]any{"expected_published_version_id": nil}, 200)
		// Repeating a successful publication must query the real tenant's records.
		if got := f.request(f.token, "POST", endpoint, map[string]any{"expected_published_version_id": version["id"]}, 200); got["status"] != "published" {
			t.Fatal(got)
		}
	})
	t.Run("access replacement", func(t *testing.T) {
		f := newIntegration(t)
		f.member("editor", false)
		ctx := context.Background()
		previous, err := f.api.PB.Find(ctx, "app_members", `role = "editor"`)
		if err != nil {
			t.Fatal(err)
		}
		hook := f.runtime.App.OnRecordCreate("app_members").BindFunc(func(e *core.RecordEvent) error { return errors.New("injected membership save failure") })
		f.request(f.token, "PUT", f.base+"/access", map[string]any{"restricted": true, "permissions": []any{map[string]any{"user_id": previous["user_id"], "role": "viewer"}}}, 503)
		f.runtime.App.OnRecordCreate("app_members").Unbind(hook)
		stored, err := f.api.PB.Get(ctx, "app_members", stringValue(previous["id"]))
		if err != nil || stored["role"] != "editor" {
			t.Fatalf("failed access replacement deleted grant: %v %v", stored, err)
		}
	})
	t.Run("import receipt", func(t *testing.T) {
		f := newIntegration(t)
		plan := f.request(f.token, "POST", f.base+"/import-plans", map[string]any{"table": "customers", "rows": []any{map[string]any{"name": "must roll back"}}}, 201)
		hook := f.runtime.App.OnRecordUpdate("batch_jobs").BindFunc(func(e *core.RecordEvent) error {
			if e.Record.Get("result") != nil && strings.Contains(fmt.Sprint(e.Record.Get("result")), "created") {
				return errors.New("injected receipt failure")
			}
			return e.Next()
		})
		f.request(f.token, "POST", f.base+"/import-plans/"+stringValue(plan["plan_id"])+"/commit", map[string]any{"confirm": true, "plan_id": plan["plan_id"]}, 503)
		f.runtime.App.OnRecordUpdate("batch_jobs").Unbind(hook)
		rows, err := f.api.PB.ListAll(context.Background(), f.table, "", "")
		if err != nil || len(rows) != 0 {
			t.Fatalf("record escaped failed receipt transaction: %v %v", rows, err)
		}
	})
}

func TestGoTaskWriteBudget(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	rows := []map[string]any{f.row("first"), f.row("second"), f.row("third")}
	task := f.request(f.token, "POST", f.base+"/tasks", map[string]any{"name": "Limited writer", "definition": map[string]any{"goal": "Complete two customers", "execution": "agent", "trigger": map[string]any{"type": "manual"}, "scope": map[string]any{"tables": []any{map[string]any{"table": "customers", "read_fields": []any{"status"}, "write_fields": []any{"status"}}}}, "limits": map[string]any{"max_writes": 2}}}, 201)
	f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/enable", map[string]any{"confirm": true, "expected_revision": 1}, 200)
	queued := f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/run", map[string]any{"expected_revision": 1, "request_id": "budget-check"}, 202)
	run, err := f.api.PB.Get(ctx, "miao_runs", stringValue(queued["id"]))
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		input := map[string]any{"table": "customers", "record_id": row["id"], "expected_updated_at": row["updated_at"], "data": map[string]any{"status": "done"}}
		_, err := f.api.updateTaskRecord(ctx, run, input, func() error { return nil })
		if i < 2 && err != nil {
			t.Fatal(err)
		}
		if i == 2 && (err == nil || !strings.Contains(err.Error(), "写入数量限制")) {
			t.Fatalf("third write escaped budget: %v", err)
		}
		if i == 0 {
			if _, err := f.api.updateTaskRecord(ctx, run, input, func() error { return nil }); err != nil {
				t.Fatalf("completed action was repeated: %v", err)
			}
		}
	}
	last, err := f.api.PB.Get(ctx, f.table, stringValue(rows[2]["id"]))
	if err != nil || last["status"] != "new" {
		t.Fatalf("third record changed: %v %v", last, err)
	}
}
