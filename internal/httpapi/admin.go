package httpapi

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

func (s *Server) routesAdmin() {
	admin := func(next http.HandlerFunc) http.HandlerFunc {
		return s.auth(func(w http.ResponseWriter, r *http.Request) {
			id := who(r)
			if !s.Admins[strings.ToLower(strings.TrimSpace(stringValue(id.User["email"]))) ] {
				s.writeAdminAudit(r.Context(), id, "access.denied", "endpoint", clip(r.URL.Path, 64), "", 403)
				writeError(w, 403, "无权访问平台管理功能")
				return
			}
			next(w, r)
		})
	}
	s.Mux.HandleFunc("GET /api/admin/overview", admin(s.adminOverview))
	s.Mux.HandleFunc("GET /api/admin/runtime", admin(s.adminRuntime))
	s.Mux.HandleFunc("GET /api/admin/users", admin(s.adminUsers))
	s.Mux.HandleFunc("PATCH /api/admin/users/{id}/status", admin(s.adminUserStatus))
	s.Mux.HandleFunc("GET /api/admin/workspaces", admin(s.adminWorkspaces))
	s.Mux.HandleFunc("GET /api/admin/apps", admin(s.adminApps))
	s.Mux.HandleFunc("GET /api/admin/usage", admin(s.adminUsage))
	s.Mux.HandleFunc("GET /api/admin/audit", admin(s.adminAudit))
	s.Mux.HandleFunc("GET /api/admin/ai", admin(s.adminAI))
	s.Mux.HandleFunc("PUT /api/admin/ai", admin(s.adminAIUpdate))
	s.Mux.HandleFunc("DELETE /api/admin/ai", admin(s.adminAIEnvironment))
}

func (s *Server) writeAdminAudit(ctx context.Context, id identity, action, targetType, targetID, reason string, status int) map[string]any {
	row, err := s.PB.Create(ctx, "platform_audit_logs", map[string]any{
		"actor_id": id.User["id"], "actor_email": id.User["email"], "action": action,
		"target_type": targetType, "target_id": clip(targetID, 64), "reason": clip(strings.TrimSpace(reason), 500), "status": status,
	})
	if err != nil { return nil }
	return row
}

func (s *Server) finishAdminAudit(ctx context.Context, audit map[string]any, action string, status int) {
	if audit != nil { _, _ = s.PB.Update(ctx, "platform_audit_logs", stringValue(audit["id"]), map[string]any{"action": action, "status": status}) }
}

func (s *Server) adminCount(ctx context.Context, collection, filter string) (int, error) {
	_, count, _, err := s.PB.List(ctx, collection, filter, "", 1, 1)
	return count, err
}

func (s *Server) adminOverview(w http.ResponseWriter, r *http.Request) {
	end := time.Now().UTC(); start := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	ctx, cancel := contextTimeout(r); defer cancel()
	users, e1 := s.adminCount(ctx, "users", "")
	disabled, e2 := s.adminCount(ctx, "users", "disabled = true")
	workspaces, e3 := s.adminCount(ctx, "tenants", "")
	apps, e4 := s.adminCount(ctx, "apps", "")
	usage, e5 := s.aggregateAdminUsage(ctx, start, end)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || e5 != nil { writeError(w, 503, "平台总览暂时不可用"); return }
	writeJSON(w, 200, map[string]any{"generated_at": end.Format(time.RFC3339Nano), "users": map[string]any{"total": users, "active": users-disabled, "disabled": disabled}, "workspaces": workspaces, "apps": apps, "ai_today": usage["totals"]})
}

func (s *Server) adminRuntime(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r); defer cancel()
	config, err := s.readAIConfig(ctx)
	if err != nil { writeError(w, 503, "运行配置状态暂时不可用"); return }
	mode := env("MIAO_REGISTRATION_MODE", "open")
	if !containsString([]string{"open", "invite", "closed"}, mode) { mode = "invalid" }
	domains := []string{}
	for _, domain := range strings.Split(os.Getenv("MIAO_ALLOWED_EMAIL_DOMAINS"), ",") { if d := strings.ToLower(strings.TrimSpace(domain)); d != "" { domains = append(domains, d) } }
	writeJSON(w, 200, map[string]any{
		"registration": map[string]any{"mode": mode, "email_verification_required": os.Getenv("MIAO_REQUIRE_EMAIL_VERIFICATION") == "true", "allowed_email_domains": domains},
		"mail": map[string]any{"configured": os.Getenv("RESEND_API_KEY") != "" && os.Getenv("MIAO_MAIL_FROM") != "", "public_url_configured": os.Getenv("MIAO_PUBLIC_URL") != ""},
		"ai": map[string]any{"provider": config.Provider, "model": config.Model, "configured": config.Key != "", "source": config.Source, "encryption_key_ready": len(os.Getenv("MIAO_SETTINGS_ENCRYPTION_KEY")) >= 32},
	})
}

func adminPage(r *http.Request) (int, int) { return queryInt(r, "page", 1, 1, 1000000), queryInt(r, "perPage", 25, 1, 100) }
func adminSearch(q, field string) string { q = strings.TrimSpace(q); if q == "" { return "" }; return field + " ~ " + pbFilterString(clip(q, 120)) }
func appendFilter(parts []string, value string) []string { if value != "" { return append(parts, value) }; return parts }
func adminPageJSON(items []map[string]any, page, perPage, total int) map[string]any { return pageResult(items, page, perPage, total) }

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	page, perPage := adminPage(r); filters := []string{}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" { v := pbFilterString(clip(q, 120)); filters = append(filters, "(email ~ "+v+" || name ~ "+v+")") }
	switch r.URL.Query().Get("status") { case "active": filters = append(filters, "disabled = false"); case "disabled": filters = append(filters, "disabled = true") }
	ctx, cancel := contextTimeout(r); defer cancel(); rows, total, _, err := s.PB.List(ctx, "users", listFilter(filters...), "-created", page, perPage)
	if err != nil { writeError(w, 503, "用户列表暂时不可用"); return }
	items := make([]map[string]any, 0, len(rows)); for _, row := range rows { items = append(items, map[string]any{"id": row["id"], "email": row["email"], "name": row["name"], "verified": boolValue(row["verified"]), "disabled": boolValue(row["disabled"]), "created_at": row["created"], "updated_at": row["updated"]}) }
	writeJSON(w, 200, adminPageJSON(items, page, perPage, total))
}

func (s *Server) adminUserStatus(w http.ResponseWriter, r *http.Request) {
	id := who(r); body := mapBody(r); disabled, ok := body["disabled"].(bool); reason := strings.TrimSpace(stringValue(body["reason"])); target := pathID(r, "id")
	reject := func(action string, status int, msg string) { s.writeAdminAudit(r.Context(), id, action, "user", target, reason, status); writeError(w, status, msg) }
	if !ok { reject("user.status.rejected", 400, "账号状态无效"); return }
	if len([]rune(reason)) < 5 || len([]rune(reason)) > 500 { reject("user.status.rejected", 400, "请填写 5 到 500 个字符的操作原因"); return }
	if disabled && target == stringValue(id.User["id"]) { reject("user.status.rejected", 409, "不能停用当前登录的平台管理员账号"); return }
	ctx, cancel := contextTimeout(r); defer cancel(); user, err := s.PB.Get(ctx, "users", target)
	if err != nil { reject("user.status.rejected", 404, "用户不存在"); return }
	if disabled && s.Admins[strings.ToLower(stringValue(user["email"]))] {
		available := 0
		for email := range s.Admins { row, e := s.PB.Find(ctx, "users", "email = "+pbFilterString(email)); if e == nil && !boolValue(row["disabled"]) && (!s.RequireVerification || boolValue(row["verified"])) { available++ } }
		if available <= 1 { reject("user.status.rejected", 409, "必须至少保留一个可用的平台管理员账号"); return }
	}
	audit := s.writeAdminAudit(ctx, id, "user.status.change_requested", "user", target, reason, 102)
	if audit == nil { writeError(w, 503, "平台审计服务暂不可用，未执行账号状态变更"); return }
	if _, err = s.PB.Update(ctx, "users", target, map[string]any{"disabled": disabled}); err != nil { s.finishAdminAudit(ctx, audit, "user.status.failed", 503); writeError(w, 503, "账号状态更新失败"); return }
	s.finishAdminAudit(ctx, audit, fmt.Sprintf("user.%s", map[bool]string{true:"disabled", false:"enabled"}[disabled]), 200)
	writeJSON(w, 200, map[string]any{"ok": true, "id": target, "disabled": disabled})
}

func (s *Server) adminWorkspaces(w http.ResponseWriter, r *http.Request) {
	page, perPage := adminPage(r); filter := adminSearch(r.URL.Query().Get("q"), "name")
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" { v := pbFilterString(clip(q,120)); filter = "(name ~ "+v+" || slug ~ "+v+")" }
	ctx, cancel := contextTimeout(r); defer cancel(); rows, total, _, err := s.PB.List(ctx, "tenants", filter, "-created", page, perPage)
	if err != nil { writeError(w, 503, "工作区列表暂时不可用"); return }
	items := make([]map[string]any,0,len(rows)); for _, row := range rows { owner, _ := s.PB.Get(ctx,"users",stringValue(row["owner_id"])); members,_:=s.adminCount(ctx,"tenant_members","tenant_id = "+pbFilterString(stringValue(row["id"]))); apps,_:=s.adminCount(ctx,"apps","tenant_id = "+pbFilterString(stringValue(row["id"]))); var ownerView any; if len(owner)>0 {ownerView=map[string]any{"id":owner["id"],"name":owner["name"],"email":owner["email"],"disabled":boolValue(owner["disabled"])} }; items=append(items,map[string]any{"id":row["id"],"name":row["name"],"slug":row["slug"],"owner":ownerView,"member_count":members,"app_count":apps,"created_at":row["created"]}) }
	writeJSON(w,200,adminPageJSON(items,page,perPage,total))
}

func (s *Server) adminApps(w http.ResponseWriter, r *http.Request) {
	page,perPage:=adminPage(r);filters:=appendFilter(nil,adminSearch(r.URL.Query().Get("q"),"name"));switch r.URL.Query().Get("archived"){case "true":filters=append(filters,"archived = true");case "false":filters=append(filters,"archived = false")}
	ctx,cancel:=contextTimeout(r);defer cancel();rows,total,_,err:=s.PB.List(ctx,"apps",listFilter(filters...),"-updated",page,perPage);if err!=nil{writeError(w,503,"应用目录暂时不可用");return}
	items:=make([]map[string]any,0,len(rows));for _,row:=range rows{tenant,_:=s.PB.Get(ctx,"tenants",stringValue(row["tenant_id"]));tables,_:=s.adminCount(ctx,"app_collections",listFilter("tenant_id = "+pbFilterString(stringValue(row["tenant_id"])),"app_id = "+pbFilterString(stringValue(row["id"]))));var tenantView any;if len(tenant)>0{tenantView=map[string]any{"id":tenant["id"],"name":tenant["name"]}};items=append(items,map[string]any{"id":row["id"],"name":row["name"],"tenant":tenantView,"archived":boolValue(row["archived"]),"restricted":boolValue(row["restricted"]),"table_count":tables,"created_at":row["created"],"updated_at":row["updated"]})}
	writeJSON(w,200,adminPageJSON(items,page,perPage,total))
}

func (s *Server) aggregateAdminUsage(ctx context.Context, from, to time.Time) (map[string]any,error) {
	filter:="created >= "+pbFilterString(from.Format(time.RFC3339Nano))+" && created <= "+pbFilterString(to.Format(time.RFC3339Nano))
	rows,err:=s.PB.ListAll(ctx,"ai_usage",filter,"created,id");if err!=nil{return nil,err}
	totals:=map[string]int{"requests":0,"successes":0,"errors":0,"pending":0,"input_tokens":0,"output_tokens":0}; tenants:=map[string]map[string]any{}
	for _,row:=range rows{tid:=stringValue(row["tenant_id"]);current:=tenants[tid];if current==nil{current=map[string]any{"tenant_id":tid,"requests":0,"successes":0,"errors":0,"pending":0,"input_tokens":0,"output_tokens":0};tenants[tid]=current};status:=intValue(row["status"]);for key,delta:=range map[string]int{"requests":1,"input_tokens":intValue(row["input_tokens"]),"output_tokens":intValue(row["output_tokens"])}{totals[key]+=delta;current[key]=intValue(current[key])+delta};bucket:="pending";if status>=400{bucket="errors"}else if status>=200&&status<400{bucket="successes"};totals[bucket]++;current[bucket]=intValue(current[bucket])+1}
	items:=make([]map[string]any,0,len(tenants));for _,row:=range tenants{items=append(items,row)};return map[string]any{"totals":totals,"byTenant":items},nil
}

func (s *Server) adminUsage(w http.ResponseWriter,r *http.Request){
	end:=time.Now().UTC();if raw:=r.URL.Query().Get("to");raw!=""{var err error;end,err=time.Parse(time.RFC3339,raw);if err!=nil{writeError(w,400,"用量查询时间范围无效");return}}
	start:=end.Add(-6*24*time.Hour);if raw:=r.URL.Query().Get("from");raw!=""{var err error;start,err=time.Parse(time.RFC3339,raw);if err!=nil{writeError(w,400,"用量查询时间范围无效");return}}
	if start.After(end){writeError(w,400,"用量查询时间范围无效");return};if end.Sub(start)>31*24*time.Hour{writeError(w,400,"单次用量查询范围最多为 31 天");return}
	page,perPage:=adminPage(r);ctx,cancel:=contextTimeout(r);defer cancel();aggregate,err:=s.aggregateAdminUsage(ctx,start,end);if err!=nil{writeError(w,503,"平台用量暂时不可用");return};rows:=asSliceMap(aggregate["byTenant"]);sort.Slice(rows,func(i,j int)bool{return intValue(rows[i]["requests"])>intValue(rows[j]["requests"])});total:=len(rows);from:=(page-1)*perPage;if from>total{from=total};to:=from+perPage;if to>total{to=total};items:=make([]map[string]any,0,to-from);for _,row:=range rows[from:to]{tenant,_:=s.PB.Get(ctx,"tenants",stringValue(row["tenant_id"]));if len(tenant)>0{row["tenant"]=map[string]any{"id":tenant["id"],"name":tenant["name"]}}else{row["tenant"]=nil};items=append(items,row)}
	writeJSON(w,200,map[string]any{"from":start.Format(time.RFC3339Nano),"to":end.Format(time.RFC3339Nano),"totals":aggregate["totals"],"items":items,"page":page,"perPage":perPage,"totalItems":total,"totalPages":(total+perPage-1)/perPage})
}

func (s *Server) adminAudit(w http.ResponseWriter,r *http.Request){
	page,perPage:=adminPage(r);filters:=appendFilter(nil,func()string{if v:=r.URL.Query().Get("targetType");v!=""{return "target_type = "+pbFilterString(v)};return ""}());filters=appendFilter(filters,func()string{if v:=r.URL.Query().Get("targetId");v!=""{return "target_id = "+pbFilterString(v)};return ""}())
	ctx,cancel:=contextTimeout(r);defer cancel();rows,total,_,err:=s.PB.List(ctx,"platform_audit_logs",listFilter(filters...),"-created",page,perPage);if err!=nil{writeError(w,503,"平台审计暂时不可用");return};items:=make([]map[string]any,0,len(rows));for _,row:=range rows{items=append(items,map[string]any{"id":row["id"],"actor_id":row["actor_id"],"actor_email":row["actor_email"],"action":row["action"],"target_type":row["target_type"],"target_id":row["target_id"],"reason":row["reason"],"status":row["status"],"created_at":row["created"]})};writeJSON(w,200,adminPageJSON(items,page,perPage,total))
}

func (s *Server) adminAI(w http.ResponseWriter,r *http.Request){ctx,cancel:=contextTimeout(r);defer cancel();config,err:=s.readAIConfig(ctx);if err!=nil{writeError(w,503,"AI 服务配置暂时不可用");return};hint:="";if len(config.Key)>0{tail:=config.Key;if len(tail)>4{tail=tail[len(tail)-4:]};hint="••••••"+tail};writeJSON(w,200,map[string]any{"configured":config.Key!="","source":config.Source,"provider":config.Provider,"model":config.Model,"key_hint":hint,"encryption_ready":len(os.Getenv("MIAO_SETTINGS_ENCRYPTION_KEY"))>=32})}

func encryptAdminKey(secret,key string)(string,error){if len(secret)<32{return "",fmt.Errorf("encryption key is not configured")};hash:=sha256.Sum256([]byte(secret));block,err:=aes.NewCipher(hash[:]);if err!=nil{return "",err};gcm,err:=cipher.NewGCM(block);if err!=nil{return "",err};nonce:=make([]byte,gcm.NonceSize());if _,err=rand.Read(nonce);err!=nil{return "",err};sealed:=gcm.Seal(nil,nonce,[]byte(key),nil);overhead:=gcm.Overhead();tag:=sealed[len(sealed)-overhead:];ciphertext:=sealed[:len(sealed)-overhead];enc:=base64.RawURLEncoding;return "v1."+enc.EncodeToString(nonce)+"."+enc.EncodeToString(tag)+"."+enc.EncodeToString(ciphertext),nil}

func(s *Server)adminAIUpdate(w http.ResponseWriter,r *http.Request){
	id:=who(r);key:=strings.TrimSpace(stringValue(mapBody(r)["api_key"]));if len(key)<16||len(key)>2000||strings.ContainsAny(key,"\r\n"){s.writeAdminAudit(r.Context(),id,"ai_key.rotation.rejected","setting","ai_gateway_api_key","",400);writeError(w,400,"AI 服务密钥格式无效");return};audit:=s.writeAdminAudit(r.Context(),id,"ai_key.rotation.requested","setting","ai_gateway_api_key","",102);if audit==nil{writeError(w,503,"平台审计服务暂不可用，未修改 AI 配置");return};value,err:=encryptAdminKey(os.Getenv("MIAO_SETTINGS_ENCRYPTION_KEY"),key);if err!=nil{s.finishAdminAudit(r.Context(),audit,"ai_key.rotation.failed",503);writeError(w,503,"请先配置 MIAO_SETTINGS_ENCRYPTION_KEY（至少 32 个字符）");return};ctx,cancel:=contextTimeout(r);defer cancel();row,err:=s.PB.Find(ctx,"platform_settings","name = "+pbFilterString("ai_gateway_api_key"));if err==nil{_,err=s.PB.Update(ctx,"platform_settings",stringValue(row["id"]),map[string]any{"value":value,"updated_by":id.User["id"]})}else{_,err=s.PB.Create(ctx,"platform_settings",map[string]any{"name":"ai_gateway_api_key","value":value,"updated_by":id.User["id"]})};if err!=nil{s.finishAdminAudit(ctx,audit,"ai_key.rotation.failed",503);writeError(w,503,"AI 服务配置更新失败");return};s.finishAdminAudit(ctx,audit,"ai_key.rotated",200);writeJSON(w,200,map[string]any{"ok":true,"configured":true})
}

func (s *Server) adminAIEnvironment(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	audit := s.writeAdminAudit(r.Context(), id, "ai_key.environment_selection.requested", "setting", "ai_gateway_api_key", "", 102)
	if audit == nil { writeError(w, 503, "平台审计服务暂不可用，未修改 AI 配置"); return }
	ctx, cancel := contextTimeout(r); defer cancel()
	row, err := s.PB.Find(ctx, "platform_settings", "name = "+pbFilterString("ai_gateway_api_key"))
	if err == nil {
		err = s.PB.Delete(ctx, "platform_settings", stringValue(row["id"]))
	} else {
		var pbErr *pocketbase.Error
		if !errors.As(err, &pbErr) || pbErr.Status != http.StatusNotFound {
			s.finishAdminAudit(ctx, audit, "ai_key.environment_selection.failed", 503)
			writeError(w, 503, "无法切换到服务器环境中的 AI 密钥")
			return
		}
		err = nil
	}
	if err != nil {
		s.finishAdminAudit(ctx, audit, "ai_key.environment_selection.failed", 503)
		writeError(w, 503, "无法切换到服务器环境中的 AI 密钥")
		return
	}
	source := "none"
	if os.Getenv("AI_GATEWAY_API_KEY") != "" { source = "environment" }
	s.finishAdminAudit(ctx, audit, "ai_key.environment_selected", 200)
	writeJSON(w, 200, map[string]any{"ok": true, "source": source})
}
