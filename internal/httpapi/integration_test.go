package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pocketbase/pocketbase/core"
	"github.com/tans/miao/internal/harness"
	"github.com/tans/miao/internal/pocketbase"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/tans/miao/internal/runtime"
)

type integrationFixture struct {
	t                                          *testing.T
	runtime                                    *runtime.Runtime
	api                                        *Server
	root, token, userID, tenantID, base, table string
}

func TestLogoutRevokesIssuedAuthToken(t *testing.T) {
	f := newIntegration(t)
	token := f.token
	f.request(token, "POST", "/api/auth/logout", nil, 200)
	f.request(token, "GET", "/api/apps", nil, 401)
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
	return map[string]any{"schema_version": 3, "title": "业务", "pages": []any{map[string]any{"id": "customers", "title": "客户", "data_sources": []any{map[string]any{"id": "customers_source", "collection": "customers", "fields": []any{"name", "status", "file"}, "actions": []any{map[string]any{"id": "complete", "label": "完成", "set": map[string]any{"status": "done"}}}}}, "spec": map[string]any{"root": "page", "elements": map[string]any{"page": map[string]any{"type": "Page", "props": map[string]any{"title": "客户"}, "children": []any{"table"}}, "table": map[string]any{"type": "RecordTable", "props": map[string]any{"source": "customers_source"}, "children": []any{}}}}}}}
}
func TestBackendCatalogAndSpec(t *testing.T) {
	f := newIntegration(t)
	catalog := f.request(f.token, "GET", f.base+"/backend/catalog", nil, 200)
	if intValue(catalog["schema_version"]) != 1 || len(anySlice(catalog["capabilities"])) == 0 || asMap(catalog["constraints"])["no_scripts"] != true || stringValue(asMap(catalog["jev"])["upstream_commit"]) != jevUpstreamCommit || asMap(catalog["jev"])["evaluation_protocol"] != "v4" {
		t.Fatalf("invalid backend catalog: %#v", catalog)
	}
	spec := f.request(f.token, "GET", f.base+"/backend/spec", nil, 200)
	if len(anySlice(spec["tables"])) != 1 || asMap(anySlice(spec["tables"])[0])["logical_id"] != "customers" || stringValue(asMap(spec["jev"])["upstream_commit"]) != jevUpstreamCommit {
		t.Fatalf("invalid backend spec: %#v", spec)
	}
}

func TestHarnessUIComposeCreatesValidatedDraftOnly(t *testing.T) {
	f := newIntegration(t)
	appID := f.base[len("/api/apps/"):]
	options, err := backendHarnessCandidates(context.Background(), f.api.PB, f.tenantID, appID, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	var candidate harness.CandidateOption
	for _, option := range options {
		if option.Capability == "ui.compose" {
			candidate = option
			break
		}
	}
	if candidate.ID == "" {
		t.Fatal("ui.compose candidate missing")
	}
	run := harness.NewRun(f.tenantID, appID, f.userID, "compose customer page", map[string]any{"candidate_ids": []any{candidate.ID}})
	definition := f.definition()
	candidate.Input = map[string]any{"definition": definition}
	result, err := f.api.executeHarness(context.Background(), run, &harness.Candidate{ID: candidate.ID, Capability: candidate.Capability, Write: true, Input: candidate.Input})
	if err != nil {
		t.Fatal(err)
	}
	if asMap(result)["published"] != false {
		t.Fatalf("compose unexpectedly published: %#v", result)
	}
	version, err := f.api.PB.Get(context.Background(), "app_versions", stringValue(asMap(result)["version"]))
	if err != nil || stringValue(version["published_at"]) != "" {
		t.Fatalf("draft was not persisted as unpublished: %#v %v", version, err)
	}
}

func TestHarnessLoopAndEventsSurvivePocketBaseRestart(t *testing.T) {
	f := newIntegration(t)
	store := pocketHarnessStore{s: f.api}
	run := harness.NewRun(f.tenantID, f.base[len("/api/apps/"):], f.userID, "durable loop", map[string]any{"goal": "persist"})
	if err := store.Create(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	owned, err := store.Acquire(context.Background(), run.ID, "integration-owner", time.Now(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	owned.State, owned.Phase = harness.StateWaiting, "confirmation"
	owned.Loop = &harness.LoopState{Decisions: 2, ModelRequests: 1, Steps: []harness.Step{{ID: run.ID + ":1", Candidate: harness.Candidate{ID: "candidate", Capability: "records.query"}, Result: harness.StepResult{Outcome: harness.OutcomeContinue, Value: map[string]any{"ok": true}}, CompletedAt: time.Now().UTC()}}}
	owned.Sequence++
	event := harness.Event{RunID: run.ID, Sequence: owned.Sequence, Type: "durable_test", Data: map[string]any{"step": "candidate"}, CreatedAt: time.Now().UTC()}
	if err := store.Commit(context.Background(), owned, &event); err != nil {
		t.Fatal(err)
	}
	if err := store.Release(context.Background(), run.ID, "integration-owner"); err != nil {
		t.Fatal(err)
	}
	f.restart()
	loaded, err := (pocketHarnessStore{s: f.api}).Load(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != harness.StateWaiting || loaded.Loop == nil || len(loaded.Loop.Steps) != 1 || loaded.Loop.Steps[0].ID != run.ID+":1" {
		t.Fatalf("durable loop was not restored: %+v", loaded)
	}
	events, err := (pocketHarnessStore{s: f.api}).Events(context.Background(), run.ID, 0, 20)
	if err != nil || len(events) != 1 || events[0].Type != "durable_test" {
		t.Fatalf("durable event was not restored: events=%+v err=%v", events, err)
	}
}

func TestBackendPlanAppearsInHarnessAndAppliesConfirmedTable(t *testing.T) {
	f := newIntegration(t)
	appID := f.base[len("/api/apps/"):]
	candidates := f.request(f.token, "GET", f.base+"/backend/candidates", nil, 200)
	var createID string
	for _, raw := range anySlice(candidates["candidates"]) {
		candidate := asMap(raw)
		if candidate["capability"] == "collections.create" {
			createID = stringValue(candidate["id"])
			break
		}
	}
	if createID == "" {
		t.Fatal("collections.create candidate missing")
	}
	options, err := backendHarnessCandidates(context.Background(), f.api.PB, f.tenantID, appID, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	foundQuery := false
	for _, option := range options {
		foundQuery = foundQuery || option.Capability == "records.query"
	}
	if !foundQuery {
		t.Fatal("shared harness did not expose executable candidates")
	}
	plan := f.request(f.token, "POST", f.base+"/backend/plans", map[string]any{"operations": []any{map[string]any{"candidate_id": createID, "input": map[string]any{"name": "线索", "slug": "leads", "fields": []any{map[string]any{"name": "title", "type": "text", "required": true}}}}}}, 201)
	applied := f.request(f.token, "POST", f.base+"/backend/plans/"+stringValue(plan["id"])+"/apply", map[string]any{"confirm": true, "expected_revision": plan["revision"]}, 200)
	if applied["status"] != "applied" {
		t.Fatalf("backend plan was not applied: %#v", applied)
	}
	response := f.response(f.token, "GET", f.base+"/collections", nil, 200)
	var tables []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &tables); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, table := range tables {
		found = found || stringValue(table["slug"]) == "leads"
	}
	if !found {
		t.Fatal("confirmed backend plan did not create leads table")
	}
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
	scope, err := f.api.conversationScope(context.Background(), identity{User: map[string]any{"id": f.userID}, Tenant: map[string]any{"id": f.tenantID, "owner_id": f.userID}, Membership: map[string]any{"role": "owner"}})
	if err != nil {
		t.Fatal(err)
	}
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
func TestAgentConversationClearsWhenAppLeavesPermissionScope(t *testing.T) {
	f := newIntegration(t)
	scope, err := f.api.conversationScope(context.Background(), identity{User: map[string]any{"id": f.userID}, Tenant: map[string]any{"id": f.tenantID, "owner_id": f.userID}, Membership: map[string]any{"role": "owner"}})
	if err != nil {
		t.Fatal(err)
	}
	f.request(f.token, "PUT", "/api/agent/conversation", map[string]any{
		"scope": scope, "expected_revision": 0, "checkpoint": "AQID",
		"messages": []any{map[string]any{"role": "user", "content": "仅在当前应用继续"}},
	}, 200)
	f.request(f.token, "PATCH", f.base, map[string]any{"archived": true}, 200)
	cleared := f.request(f.token, "GET", "/api/agent/conversation", nil, 200)
	if cleared["conversation"] != nil {
		t.Fatalf("conversation remained after its app left the permission scope: %#v", cleared)
	}
	if _, err := f.api.PB.Find(context.Background(), "agent_sessions", listFilter("tenant_id = "+pbFilterString(f.tenantID), "user_id = "+pbFilterString(f.userID))); err == nil {
		t.Fatal("stale agent session was not deleted")
	}
}

func (f *integrationFixture) publicContentSetup() map[string]any {
	f.t.Helper()
	f.request(f.token, "PATCH", f.base+"/collections/customers", map[string]any{"fields": []any{map[string]any{"name": "name", "type": "text", "required": true}, map[string]any{"name": "status", "type": "select", "options": []any{"new", "done"}}, map[string]any{"name": "slug", "type": "text"}, map[string]any{"name": "file", "type": "file"}}}, 200)
	definition := f.definition()
	page := asMap(anySlice(definition["pages"])[0])
	source := asMap(anySlice(page["data_sources"])[0])
	source["fields"] = []any{"name", "status", "slug", "file"}
	return definition
}

func publicContentGrant(slug string) map[string]any {
	return map[string]any{"enabled": true, "confirm": true, "slug": slug, "pages": []any{map[string]any{"id": "customers", "reads": []any{map[string]any{"source": "customers_source", "table": "customers", "fields": []any{"name", "slug"}, "status_field": "status", "published_value": "done", "slug_field": "slug", "seo_title_field": "name"}}}}}
}

func TestPublicPublicationScopesCurrentPublishedData(t *testing.T) {
	f := newIntegration(t)
	definition := f.publicContentSetup()
	version := f.request(f.token, "POST", f.base+"/versions", map[string]any{"definition": definition}, 201)
	f.request(f.token, "POST", f.base+"/versions/"+stringValue(version["id"])+"/publish", map[string]any{"expected_published_version_id": nil}, 200)
	f.request(f.token, "POST", f.base+"/collections/customers/records", map[string]any{"data": map[string]any{"name": "Public name", "status": "done", "slug": "public-name"}}, 201)
	profile := publicContentGrant("public-catalog")
	unconfirmed := map[string]any{"enabled": true, "slug": "unconfirmed", "pages": profile["pages"]}
	f.request(f.token, "PUT", f.base+"/publication", unconfirmed, 403)
	managerToken := f.member("manager", false)
	f.request(managerToken, "PUT", f.base+"/publication", profile, 403)
	savedPublication := f.request(f.token, "PUT", f.base+"/publication", profile, 200)
	if savedPublication["url"] != "/s/public-catalog" || savedPublication["enabled"] != true {
		t.Fatalf("publication configuration was not saved: %#v", savedPublication)
	}

	runtime := f.request("", "GET", "/api/public/public-catalog/runtime", nil, 200)
	source := asMap(asMap(runtime["sources"])["customers_source"])
	items := anySlice(source["items"])
	if len(items) != 1 || runtime["read_only"] != true || source["create_form_available"] != false {
		t.Fatalf("unexpected public runtime: %#v", runtime)
	}
	item := asMap(items[0])
	data := asMap(item["data"])
	if data["name"] != "Public name" || len(data) != 2 || item["id"] != nil || source["actions"] == nil {
		t.Fatalf("public runtime leaked data or write capability: %#v", runtime)
	}
	publicRows := f.request("", "GET", "/api/public/public-catalog/records?page_id=customers&table=customers&source=customers_source", nil, 200)
	publicItem := asMap(anySlice(publicRows["items"])[0])
	if len(asMap(publicItem["data"])) != 2 || asMap(publicItem["data"])["name"] != "Public name" {
		t.Fatalf("public read grant was not field-limited: %#v", publicRows)
	}
	f.request("", "GET", "/api/public/public-catalog/records?page_id=customers&table=unknown&source=customers_source", nil, 404)
	f.request("", "POST", "/api/public/public-catalog/records?page_id=customers&table=customers&source=customers_source", map[string]any{"data": map[string]any{"name": "Injected"}}, 404)

	draft := f.request(f.token, "POST", f.base+"/versions", map[string]any{"definition": definition, "summary": "unpublished"}, 201)
	if stringValue(draft["id"]) == stringValue(version["id"]) {
		t.Fatal("expected a separate unpublished draft")
	}
	stillPublished := f.request("", "GET", "/api/public/public-catalog/runtime", nil, 200)
	if intValue(stillPublished["version"].(map[string]any)["version"]) != 1 {
		t.Fatalf("public site served a draft: %#v", stillPublished)
	}
	f.request(f.token, "PUT", f.base+"/publication", map[string]any{"enabled": false, "confirm": true}, 200)
	f.request("", "GET", "/api/public/public-catalog/runtime", nil, 404)
	siteResponse := httptest.NewRecorder()
	f.api.servePublicSite(siteResponse, httptest.NewRequest("GET", "https://miao.example/s/public-catalog", nil), fstest.MapFS{"site.html": &fstest.MapFile{Data: []byte("<title>PUBLIC_TITLE</title>")}})
	if siteResponse.Code != http.StatusNotFound {
		t.Fatalf("closed public site remained accessible: %d", siteResponse.Code)
	}
}

func TestPublicSiteServesCanonicalOpenGraphMetadata(t *testing.T) {
	f := newIntegration(t)
	definition := f.publicContentSetup()
	version := f.request(f.token, "POST", f.base+"/versions", map[string]any{"definition": definition}, 201)
	f.request(f.token, "POST", f.base+"/versions/"+stringValue(version["id"])+"/publish", map[string]any{"expected_published_version_id": nil}, 200)
	f.request(f.token, "PUT", f.base+"/publication", publicContentGrant("metadata-example"), 200)
	assets := fstest.MapFS{"public/site.html": &fstest.MapFile{Data: []byte(`<title>PUBLIC_TITLE</title><meta name="description" content="PUBLIC_DESCRIPTION"><meta property="og:url" content="PUBLIC_CANONICAL"><link rel="canonical" href="PUBLIC_CANONICAL">`)}}
	request := httptest.NewRequest("GET", "https://miao.example/s/metadata-example?page=customers", nil)
	response := httptest.NewRecorder()
	handler, err := f.api.Handler(assets)
	if err != nil {
		t.Fatal(err)
	}
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	for _, expected := range []string{"客户", "https://miao.example/s/metadata-example?page=customers", "客户 · MIAO"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("public SEO metadata missing %q: %s", expected, body)
		}
	}
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("public site response metadata is unsafe: %d %v", response.Code, response.Header())
	}
}

func TestPublicContentDraftDetailAndImageBoundaries(t *testing.T) {
	f := newIntegration(t)
	definition := f.publicContentSetup()
	version := f.request(f.token, "POST", f.base+"/versions", map[string]any{"definition": definition}, 201)
	f.request(f.token, "POST", f.base+"/versions/"+stringValue(version["id"])+"/publish", map[string]any{"expected_published_version_id": nil}, 200)
	profile := publicContentGrant("cms-check")
	read := asMap(anySlice(asMap(anySlice(profile["pages"])[0])["reads"])[0])
	read["fields"] = []any{"name", "slug", "file"}
	read["images"] = []any{"file"}
	f.request(f.token, "PUT", f.base+"/publication", profile, 200)
	imageBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAAIGNIUk0AAHomAACAhAAA+gAAAIDoAAB1MAAA6mAAADqYAAAXcJy6UTwAAAAGYktHRAD/AP8A/6C9p5MAAAAHdElNRQfqCgUHEyEurD2eAAAAJXRFWHRkYXRlOmNyZWF0ZQAyMDI2LTEwLTA1VDA3OjE5OjMzKzAwOjAwntw1DgAAACV0RVh0ZGF0ZTptb2RpZnkAMjAyNi0xMC0wNVQwNzoxOTozMyswMDowMO+BjbIAAAAodEVYdGRhdGU6dGltZXN0YW1wADIwMjYtMTAtMDVUMDc6MTk6MzMrMDA6MDC4lKxtAAAADElEQVQI12P4z8AAAAMBAQAY3Y2wAAAAAElFTkSuQmCC")
	if err != nil {
		t.Fatal(err)
	}
	draft := f.request(f.token, "POST", f.base+"/collections/customers/records", map[string]any{"data": map[string]any{"name": "Private draft", "status": "new", "slug": "private-draft"}}, 201)
	file := f.request(f.token, "POST", f.base+"/files", map[string]any{"name": "draft.png", "base64": base64.StdEncoding.EncodeToString(imageBytes)}, 201)
	draft = f.request(f.token, "POST", f.base+"/files/"+stringValue(file["id"])+"/attach", map[string]any{"table": "customers", "record_id": draft["id"], "field": "file", "expected_updated_at": draft["updated_at"]}, 200)
	listing := "/api/public/cms-check/records?page_id=customers&table=customers&source=customers_source"
	if rows := f.request("", "GET", listing, nil, 200); intValue(rows["totalItems"]) != 0 {
		t.Fatalf("draft appeared in public list: %#v", rows)
	}
	f.request("", "GET", "/api/public/cms-check/records/private-draft?page_id=customers&table=customers&source=customers_source", nil, 404)
	image := "/api/public/cms-check/images/customers/customers_source/customers/" + stringValue(draft["id"]) + "/file"
	f.response("", "GET", image, nil, 404)
	assets := fstest.MapFS{"public/site.html": &fstest.MapFile{Data: []byte("<title>PUBLIC_TITLE</title>PUBLIC_BODY")}}
	handler, err := f.api.Handler(assets)
	if err != nil {
		t.Fatal(err)
	}
	publicHTML := func(path string, code int) string {
		t.Helper()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", "https://miao.example"+path, nil))
		if response.Code != code {
			t.Fatalf("%s: status=%d body=%s", path, response.Code, response.Body.String())
		}
		return response.Body.String()
	}
	if body := publicHTML("/s/cms-check/customers", 200); strings.Contains(body, "Private draft") {
		t.Fatal("draft leaked into indexable HTML")
	}
	publicHTML("/s/cms-check/customers/customers_source/private-draft", 404)
	updated := f.request(f.token, "PATCH", f.base+"/collections/customers/records/"+stringValue(draft["id"]), map[string]any{"data": map[string]any{"name": "Public news", "status": "done"}, "expected_updated_at": draft["updated_at"]}, 200)
	if rows := f.request("", "GET", listing, nil, 200); intValue(rows["totalItems"]) != 1 {
		t.Fatalf("published content missing: %#v", rows)
	}
	if detail := f.request("", "GET", "/api/public/cms-check/records/private-draft?page_id=customers&table=customers&source=customers_source", nil, 200); asMap(detail["data"])["name"] != "Public news" {
		t.Fatalf("public detail stale: %#v", detail)
	}
	if body := publicHTML("/s/cms-check/customers/customers_source/private-draft", 200); !strings.Contains(body, "Public news") {
		t.Fatal("published body missing from HTML")
	}
	f.response("", "GET", image, nil, 200)
	f.request(f.token, "PATCH", f.base+"/collections/customers/records/"+stringValue(draft["id"]), map[string]any{"data": map[string]any{"name": "Updated news"}, "expected_updated_at": updated["updated_at"]}, 200)
	if body := publicHTML("/s/cms-check/customers/customers_source/private-draft", 200); !strings.Contains(body, "Updated news") || strings.Contains(body, "Public news") {
		t.Fatalf("content not refreshed: %s", body)
	}
}

func TestPublicAnonymousEndpointsAreRateLimited(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/public/example/runtime", nil)
	request.RemoteAddr = "198.51.100.44:34567"
	for i := 0; i < 120; i++ {
		response := httptest.NewRecorder()
		if !allowPublicRequest(response, request) || response.Code != 200 {
			t.Fatalf("request %d was unexpectedly limited: %d", i+1, response.Code)
		}
	}
	response := httptest.NewRecorder()
	if allowPublicRequest(response, request) || response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" {
		t.Fatalf("request limit did not return 429 with Retry-After: %d %v", response.Code, response.Header())
	}
}

func TestApplicationSourceVersionsAreRejected(t *testing.T) {
	f := newIntegration(t)
	for _, input := range []map[string]any{
		{"format": "html", "source": map[string]any{"index.html": "<html></html>"}},
		{"manifest": map[string]any{"entry": "index.html"}},
		{"capabilities": []any{"records.read"}},
	} {
		f.request(f.token, "POST", f.base+"/versions", input, 400)
	}
}

func TestGoGenericBusinessActionIsTransactionalAndIdempotent(t *testing.T) {
	f := newIntegration(t)
	second := f.request(f.token, "POST", f.base+"/collections", map[string]any{
		"name": "跟进记录", "slug": "activities",
		"fields": []any{map[string]any{"name": "note", "type": "text", "required": true}},
	}, 201)
	_ = second
	row := f.row("张三")
	current := f.request(f.token, "GET", f.base+"/collections/customers/records/"+stringValue(row["id"]), nil, 200)
	definition := map[string]any{
		"inputs":     []any{map[string]any{"name": "note", "type": "text", "required": true}},
		"conditions": []any{map[string]any{"table": "customers", "record_id": row["id"], "field": "status", "op": "eq", "value": "new"}},
		"steps": []any{
			map[string]any{"id": "finish", "operation": "update", "table": "customers", "record_id": row["id"], "expected_updated_at": current["updated_at"], "data": map[string]any{"status": "done"}},
			map[string]any{"id": "activity", "operation": "create", "table": "activities", "data": map[string]any{"note": "$note"}},
		},
	}
	action := f.request(f.token, "POST", f.base+"/actions", map[string]any{"name": "完成并记录跟进", "definition": definition}, 201)
	f.request(f.token, "POST", f.base+"/actions/"+stringValue(action["id"])+"/enable", map[string]any{"confirm": true}, 200)
	result := f.request(f.token, "POST", f.base+"/actions/"+stringValue(action["id"])+"/execute", map[string]any{"idempotency_key": "order-1", "input": map[string]any{"note": "已完成"}}, 200)
	if stringValue(result["status"]) != "completed" || len(anySlice(result["steps"])) != 2 {
		t.Fatal(result)
	}
	repeated := f.request(f.token, "POST", f.base+"/actions/"+stringValue(action["id"])+"/execute", map[string]any{"idempotency_key": "order-1", "input": map[string]any{"note": "不同内容"}}, 200)
	if !equalJSON(result, repeated) {
		t.Fatalf("idempotent result changed: %v vs %v", result, repeated)
	}
	activities := f.request(f.token, "GET", f.base+"/collections/activities/records", nil, 200)
	if intValue(activities["totalItems"]) != 1 {
		t.Fatal(activities)
	}

	rollback := map[string]any{"steps": []any{
		map[string]any{"id": "first", "operation": "update", "table": "customers", "record_id": row["id"], "expected_updated_at": asMap(result["steps"].([]any)[0])["record"].(map[string]any)["updated_at"], "data": map[string]any{"status": "new"}},
		map[string]any{"id": "second", "operation": "update", "table": "customers", "record_id": row["id"], "expected_updated_at": "stale", "data": map[string]any{"status": "done"}},
	}}
	failedAction := f.request(f.token, "POST", f.base+"/actions", map[string]any{"name": "冲突动作", "definition": rollback}, 201)
	f.request(f.token, "POST", f.base+"/actions/"+stringValue(failedAction["id"])+"/enable", map[string]any{"confirm": true}, 200)
	f.request(f.token, "POST", f.base+"/actions/"+stringValue(failedAction["id"])+"/execute", map[string]any{"idempotency_key": "rollback-1"}, 409)
	after := f.request(f.token, "GET", f.base+"/collections/customers/records/"+stringValue(row["id"]), nil, 200)
	if asMap(after["data"])["status"] != "done" {
		t.Fatalf("transaction did not roll back: %v", after)
	}
}

func TestGoGenericWorkflowTransitionIsGuardedAndIdempotent(t *testing.T) {
	f := newIntegration(t)
	row := f.row("待处理")
	workflow := f.request(f.token, "POST", f.base+"/workflows", map[string]any{
		"name": "通用状态流",
		"definition": map[string]any{
			"table": "customers", "state_field": "status",
			"states":      []any{map[string]any{"id": "new", "label": "待处理"}, map[string]any{"id": "done", "label": "已完成"}},
			"transitions": []any{map[string]any{"id": "finish", "label": "完成", "from": "new", "to": "done"}},
		},
	}, 201)
	f.request(f.token, "POST", f.base+"/workflows/"+stringValue(workflow["id"])+"/enable", map[string]any{"confirm": true, "expected_revision": 1}, 200)
	result := f.request(f.token, "POST", f.base+"/workflows/"+stringValue(workflow["id"])+"/transition", map[string]any{
		"transition_id": "finish", "record_id": row["id"], "expected_updated_at": row["updated_at"], "idempotency_key": "finish-1",
	}, 200)
	repeated := f.request(f.token, "POST", f.base+"/workflows/"+stringValue(workflow["id"])+"/transition", map[string]any{
		"transition_id": "finish", "record_id": row["id"], "expected_updated_at": "stale", "idempotency_key": "finish-1",
	}, 200)
	if !equalJSON(result, repeated) {
		t.Fatalf("workflow transition was not idempotent: %v vs %v", result, repeated)
	}
	current := f.request(f.token, "GET", f.base+"/collections/customers/records/"+stringValue(row["id"]), nil, 200)
	f.request(f.token, "POST", f.base+"/workflows/"+stringValue(workflow["id"])+"/transition", map[string]any{
		"transition_id": "finish", "record_id": row["id"], "expected_updated_at": current["updated_at"], "idempotency_key": "finish-2",
	}, 409)
}

func TestHarnessRunIsPrivateToAccountAndWorkspace(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	memberToken := f.member("editor", false)
	member, err := f.api.PB.Find(ctx, "users", `email = "editor@example.invalid"`)
	if err != nil {
		t.Fatal(err)
	}
	run, err := f.api.PB.Create(ctx, "miao_harness_runs", map[string]any{"tenant_id": f.tenantID, "app_id": f.base[len("/api/apps/"):], "user_id": member["id"], "prompt": "private run", "input": map[string]any{}, "state": "waiting_confirmation", "phase": "confirmation", "sequence": 1, "version": 1, "candidate": map[string]any{"id": "private-candidate", "capability": "records.query", "write": false}, "authority": map[string]any{}, "result": map[string]any{}, "cancel_requested": false})
	if err != nil {
		t.Fatal(err)
	}
	f.request(f.token, "GET", "/api/agent/runs/"+stringValue(run["id"]), nil, 404)
	f.request(memberToken, "GET", "/api/agent/runs/"+stringValue(run["id"]), nil, 200)
}

func TestGoConnectorAndTaskAuthorization(t *testing.T) {
	f := newIntegration(t)
	connector := f.request(f.token, "POST", f.base+"/connectors", map[string]any{
		"name":       "公开数据源",
		"definition": map[string]any{"base_url": "https://example.com", "allowed_paths": []any{"/api"}, "max_bytes": 4096},
	}, 201)
	f.request(f.token, "POST", f.base+"/connectors/"+stringValue(connector["id"])+"/enable", map[string]any{"confirm": true, "expected_revision": 1}, 200)
	task := f.request(f.token, "POST", f.base+"/tasks", map[string]any{"name": "读取外部数据", "definition": map[string]any{
		"goal": "读取外部数据并核对", "execution": "agent", "trigger": map[string]any{"type": "manual"},
		"scope": map[string]any{
			"tables":        []any{map[string]any{"table": "customers", "read_fields": []any{"status"}, "write_fields": []any{}}},
			"connector_ids": []any{connector["id"]},
		},
	}}, 201)
	connectorAuthorized := false
	for _, raw := range anySlice(asMap(asMap(task["definition"])["scope"])["connector_ids"]) {
		connectorAuthorized = connectorAuthorized || stringValue(raw) == stringValue(connector["id"])
	}
	if !connectorAuthorized {
		t.Fatalf("connector authorization was not retained: %v", task)
	}
	f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/enable", map[string]any{"confirm": true, "expected_revision": 1}, 200)
	invalid := f.response(f.token, "POST", f.base+"/connectors/"+stringValue(connector["id"])+"/fetch", map[string]any{"path": "/private", "idempotency_key": "blocked-1"}, 400)
	if !strings.Contains(invalid.Body.String(), "允许范围") {
		t.Fatalf("connector path was not rejected: %s", invalid.Body.String())
	}
}

func TestGoTaskBusinessActionUsesTaskAuthorityAndIsIdempotent(t *testing.T) {
	f := newIntegration(t)
	row := f.row("待处理")
	definition := map[string]any{"steps": []any{map[string]any{
		"id": "finish", "operation": "update", "table": "customers", "record_id": row["id"],
		"expected_updated_at": row["updated_at"], "data": map[string]any{"status": "done"},
	}}}
	action := f.request(f.token, "POST", f.base+"/actions", map[string]any{"name": "后台完成", "definition": definition}, 201)
	f.request(f.token, "POST", f.base+"/actions/"+stringValue(action["id"])+"/enable", map[string]any{"confirm": true}, 200)
	task := f.request(f.token, "POST", f.base+"/tasks", map[string]any{"name": "自动完成", "definition": map[string]any{
		"goal": "完成客户", "execution": "agent", "trigger": map[string]any{"type": "manual"},
		"scope": map[string]any{"tables": []any{map[string]any{"table": "customers", "read_fields": []any{"status"}, "write_fields": []any{"status"}}}, "action_ids": []any{action["id"]}},
	}}, 201)
	f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/enable", map[string]any{"confirm": true, "expected_revision": 1}, 200)
	queued := f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/run", map[string]any{"expected_revision": 1, "request_id": "action-run"}, 202)
	run, err := f.api.PB.Get(context.Background(), "miao_runs", stringValue(queued["id"]))
	if err != nil {
		t.Fatal(err)
	}
	result, err := f.api.executeTaskBusinessAction(context.Background(), run, map[string]any{"action_id": action["id"], "idempotency_key": "task-action-1"}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := f.api.executeTaskBusinessAction(context.Background(), run, map[string]any{"action_id": action["id"], "idempotency_key": "task-action-1"}, func() error { return nil })
	if err != nil || result != repeated {
		t.Fatalf("background action was not idempotent: %q %q %v", result, repeated, err)
	}
	stored, err := f.api.PB.Get(context.Background(), f.table, stringValue(row["id"]))
	if err != nil || stored["status"] != "done" {
		t.Fatalf("background action did not update the record: %v %v", stored, err)
	}
}

func TestCRMMemberSwitchesWorkspacesWithoutSharingPrivateConversation(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	secondary := f.request("", "POST", "/api/auth/register", map[string]any{"name": "CRM teammate", "email": "crm-teammate@example.invalid", "password": "Smoke-Password-2026"}, 201)
	memberToken := stringValue(secondary["token"])
	secondaryTenant := asMap(secondary["tenant"])
	secondaryApp := f.request(memberToken, "POST", "/api/apps", map[string]any{"name": "Private app"}, 201)
	secondaryAppID := stringValue(secondaryApp["id"])
	thread, err := f.api.PB.Create(ctx, "agent_threads", map[string]any{"tenant_id": secondaryTenant["id"], "app_id": secondaryAppID, "user_id": asMap(secondary["user"])["id"], "title": "private"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.api.PB.Create(ctx, "agent_messages", map[string]any{"tenant_id": secondaryTenant["id"], "thread_id": thread["id"], "user_id": asMap(secondary["user"])["id"], "role": "user", "content": "private workspace message"}); err != nil {
		t.Fatal(err)
	}
	invite := f.request(f.token, "POST", "/api/workspace/invites", map[string]any{"email": "crm-teammate@example.invalid"}, 201)
	parts := strings.SplitN(stringValue(invite["invite_url"]), "?invite=", 2)
	if len(parts) != 2 || parts[1] == "" {
		t.Fatalf("invite URL missing token: %#v", invite)
	}
	f.request(memberToken, "POST", "/api/invites/accept", map[string]any{"token": parts[1]}, 200)
	call := func(tenantID, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+memberToken)
		req.Header.Set("X-Miao-Tenant-Id", tenantID)
		response := httptest.NewRecorder()
		f.api.ServeHTTP(response, req)
		return response
	}
	joined := call(f.tenantID, "/api/me")
	if joined.Code != 200 {
		t.Fatalf("joined workspace unavailable: %d %s", joined.Code, joined.Body.String())
	}
	var joinedData map[string]any
	if err := json.Unmarshal(joined.Body.Bytes(), &joinedData); err != nil {
		t.Fatal(err)
	}
	apps := anySlice(joinedData["apps"])
	if len(apps) != 1 || asMap(apps[0])["id"] != f.base[len("/api/apps/"):] {
		t.Fatalf("wrong apps in joined workspace: %#v", joinedData["apps"])
	}
	if response := call(f.tenantID, "/api/apps/"+secondaryAppID); response.Code != 404 {
		t.Fatalf("cross-workspace app access returned %d", response.Code)
	}
	original := call(stringValue(secondaryTenant["id"]), "/api/agent/threads/"+stringValue(thread["id"])+"/messages")
	if original.Code != 200 || !strings.Contains(original.Body.String(), "private workspace message") {
		t.Fatalf("private conversation did not remain in original workspace: %d %s", original.Code, original.Body.String())
	}
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
	if err != nil || count != 1 {
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
	hasBackgroundAudit := false
	for _, item := range history {
		hasBackgroundAudit = hasBackgroundAudit || item["source"] == "background"
	}
	if err != nil || len(history) != 2 || !hasBackgroundAudit {
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
	for _, collection := range []string{"miao_runs"} {
		_, count, _, err := f.api.PB.List(ctx, collection, "", "", 1, 10)
		if err != nil || count != 1 {
			t.Fatalf("%s: %d %v", collection, count, err)
		}
	}
	_, changeCount, _, err := f.api.PB.List(ctx, "miao_record_changes", "", "", 1, 10)
	if err != nil || changeCount != 2 {
		t.Fatalf("miao_record_changes: %d %v", changeCount, err)
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
	migrations := core.AppMigrations.Items()
	rollbackCount := 0
	for index, migration := range migrations {
		if migration.File == "20261010000000_file_image_mime_types.js" {
			rollbackCount = len(migrations) - index
			break
		}
	}
	if rollbackCount == 0 {
		t.Fatal("file protection migration boundary is missing")
	}
	if _, err := runner.Down(rollbackCount); err != nil {
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

func TestGoAutomationDeliveryRetriesPersistedEvent(t *testing.T) {
	f := newIntegration(t)
	ctx := context.Background()
	if _, err := f.api.PB.Create(ctx, "automation_rules", map[string]any{"tenant_id": f.tenantID, "app_id": f.base[len("/api/apps/"):], "created_by": f.userID, "name": "新增客户", "enabled": true, "definition": map[string]any{"trigger": "record_created", "table": "customers", "recipient_id": f.userID}}); err != nil {
		t.Fatal(err)
	}
	hookID := f.runtime.App.OnRecordCreate("automation_notifications").BindFunc(func(e *core.RecordEvent) error { return errors.New("injected notification failure") })
	row := f.row("张三")
	f.runtime.App.OnRecordCreate("automation_notifications").Unbind(hookID)
	runs, err := f.api.PB.ListAll(ctx, "automation_runs", "status = \"pending\"", "")
	if err != nil || len(runs) != 1 || asMap(runs[0]["result"])["source"] == nil {
		t.Fatalf("pending event snapshot was not persisted: %v %v", runs, err)
	}
	f.api.retryPendingAutomationRuns(ctx)
	run, err := f.api.PB.Get(ctx, "automation_runs", stringValue(runs[0]["id"]))
	if err != nil || run["status"] != "delivered" {
		t.Fatalf("pending automation was not retried: %v %v", run, err)
	}
	changes, err := f.api.PB.ListAll(ctx, "miao_record_changes", "record_id = "+pbFilterString(stringValue(row["id"])), "")
	if err != nil || len(changes) != 1 || changes[0]["event"] != "created" {
		t.Fatalf("transactional automation outbox row is missing: %v %v", changes, err)
	}
	f.api.retryPendingRecordAutomations(ctx)
	change, err := f.api.PB.Get(ctx, "miao_record_changes", stringValue(changes[0]["id"]))
	if err != nil || !boolValue(change["automation_processed"]) {
		t.Fatalf("automation outbox event was not acknowledged: %v %v", change, err)
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
	// A request ID belongs to one task revision; revising the task must not reuse the old run.
	f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/pause", nil, 200)
	f.request(f.token, "PATCH", f.base+"/tasks/"+stringValue(task["id"]), map[string]any{"expected_revision": 1, "definition": task["definition"]}, 200)
	f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/enable", map[string]any{"confirm": true, "expected_revision": 2}, 200)
	revised := f.request(f.token, "POST", f.base+"/tasks/"+stringValue(task["id"])+"/run", map[string]any{"expected_revision": 2, "request_id": "budget-check"}, 202)
	if revised["id"] == queued["id"] {
		t.Fatal("revised task reused an old run")
	}
	last, err := f.api.PB.Get(ctx, f.table, stringValue(rows[2]["id"]))
	if err != nil || last["status"] != "new" {
		t.Fatalf("third record changed: %v %v", last, err)
	}
}
