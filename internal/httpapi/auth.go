package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

func (s *Server) registerAuthRoutes() {
	s.Mux.HandleFunc("POST /api/auth/register", s.register)
	s.Mux.HandleFunc("POST /api/auth/login", s.login)
	s.Mux.HandleFunc("POST /api/auth/verify-email", s.verifyEmail)
	s.Mux.HandleFunc("POST /api/auth/password-reset/request", s.passwordResetRequest)
	s.Mux.HandleFunc("POST /api/auth/password-reset/confirm", s.passwordResetConfirm)
	s.Mux.HandleFunc("POST /api/auth/logout", s.auth(s.logout))
	s.Mux.HandleFunc("GET /api/me", s.auth(s.me))
	s.Mux.HandleFunc("DELETE /api/me", s.auth(s.deleteAccount))
	s.Mux.HandleFunc("POST /api/me/deactivate", s.auth(s.deactivateAccount))
	s.Mux.HandleFunc("GET /api/workspace/members", s.auth(s.listMembers))
	s.Mux.HandleFunc("PATCH /api/workspace/members/{id}", s.auth(s.updateMember))
	s.Mux.HandleFunc("DELETE /api/workspace/members/{id}", s.auth(s.removeMember))
	s.Mux.HandleFunc("GET /api/workspace/invites", s.auth(s.listInvites))
	s.Mux.HandleFunc("POST /api/workspace/invites", s.auth(s.createInvite))
	s.Mux.HandleFunc("DELETE /api/workspace/invites/{id}", s.auth(s.revokeInvite))
	s.Mux.HandleFunc("POST /api/invites/accept", s.auth(s.acceptInvite))
	s.Mux.HandleFunc("GET /api/workspace/ai-usage", s.auth(s.aiUsage))
	s.Mux.HandleFunc("PATCH /api/workspace/ai-budget", s.auth(s.aiBudget))
	s.Mux.HandleFunc("GET /api/workspace/audit", s.auth(s.audit))
	s.Mux.HandleFunc("GET /api/workspace/export", s.auth(s.exportWorkspace))
}

func randomToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func normalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }
func validEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil && strings.Contains(email, "@")
}
func isMailConfigured() bool {
	return os.Getenv("RESEND_API_KEY") != "" && os.Getenv("MIAO_MAIL_FROM") != ""
}
func publicURL(path string) string {
	return strings.TrimRight(env("MIAO_PUBLIC_URL", "http://localhost:41874"), "/") + path
}

func sendMail(ctx context.Context, to, subject, htmlBody string) error {
	if !isMailConfigured() {
		return fmt.Errorf("邮件服务尚未配置")
	}
	body := map[string]any{"from": os.Getenv("MIAO_MAIL_FROM"), "to": []string{to}, "subject": subject, "html": htmlBody}
	requestBody, _ := jsonMarshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", strings.NewReader(string(requestBody)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+os.Getenv("RESEND_API_KEY"))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("mail provider returned %d", res.StatusCode)
	}
	return nil
}

func (s *Server) issueAccountToken(ctx context.Context, user map[string]any, kind, continuation string) (map[string]any, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	hours := 1
	if kind == "verify" {
		hours = 24
	}
	expires := time.Now().Add(time.Duration(hours) * time.Hour).UTC().Format(time.RFC3339Nano)
	accountToken, err := s.PB.Create(ctx, "account_tokens", map[string]any{"user_id": user["id"], "token_hash": hashToken(token), "kind": kind, "expires_at": expires})
	if err != nil {
		return nil, err
	}
	path := "/reset-password?token=" + url.QueryEscape(token)
	subject, label := "重置你的 MIAO 密码", "重置密码"
	if kind == "verify" {
		path = "/verify-email?token=" + url.QueryEscape(token)
		subject, label = "验证你的 MIAO 邮箱", "验证邮箱"
		if continuation != "" {
			path += "&invite=" + url.QueryEscape(continuation)
		}
	}
	href := html.EscapeString(publicURL(path))
	message := fmt.Sprintf("<p>你好 %s，</p><p>请在 %d 小时内使用以下链接%s：</p><p><a href=\"%s\">%s</a></p><p>如果这不是你的操作，请忽略此邮件。</p>", html.EscapeString(stringValue(user["name"])), hours, label, href, label)
	if isMailConfigured() {
		if err := sendMail(ctx, stringValue(user["email"]), subject, message); err != nil {
			_ = s.PB.Delete(ctx, "account_tokens", stringValue(accountToken["id"]))
			return nil, err
		}
		return map[string]any{"sent": true}, nil
	}
	return map[string]any{"sent": false, "href": href}, nil
}

func (s *Server) findAccountToken(ctx context.Context, token, kind string) (map[string]any, error) {
	if token == "" || len(token) > 128 {
		return nil, fmt.Errorf("链接无效")
	}
	row, err := s.PB.Find(ctx, "account_tokens", listFilter("token_hash = "+pbFilterString(hashToken(token)), "kind = "+pbFilterString(kind)))
	if err != nil || !parseTime(row["expires_at"]).After(time.Now()) {
		return nil, fmt.Errorf("链接已失效或已使用")
	}
	return row, nil
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	input := mapBody(r)
	email, password, name := normalizeEmail(stringValue(input["email"])), stringValue(input["password"]), strings.TrimSpace(stringValue(input["name"]))
	if !validEmail(email) {
		writeError(w, 400, "请输入有效邮箱")
		return
	}
	if len(password) < 8 {
		writeError(w, 400, "密码至少 8 位")
		return
	}
	if name == "" {
		writeError(w, 400, "请输入姓名")
		return
	}
	if !containsString([]string{"open", "invite", "closed"}, s.RegistrationMode) {
		writeError(w, 503, "注册策略配置无效")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	inviteToken := stringValue(input["invite_token"])
	var invite map[string]any
	if inviteToken != "" {
		invite, _ = s.PB.Find(ctx, "tenant_invites", listFilter("token_hash = "+pbFilterString(hashToken(inviteToken)), `status = "pending"`))
	}
	if s.RegistrationMode == "closed" || s.RegistrationMode == "invite" && (invite == nil || normalizeEmail(stringValue(invite["email"])) != email || !parseTime(invite["expires_at"]).After(time.Now())) {
		writeError(w, 403, "当前仅允许受邀邮箱注册")
		return
	}
	if len(s.AllowedDomains) > 0 && !s.AllowedDomains[strings.SplitN(email, "@", 2)[1]] {
		writeError(w, 403, "此邮箱域名暂不允许注册")
		return
	}
	if s.RequireVerification && !isMailConfigured() {
		writeError(w, 503, "邮箱验证已开启，但邮件服务尚未配置")
		return
	}

	var user, tenant map[string]any
	err := s.PB.Transaction(ctx, func(pb *pocketbase.Client) error {
		var err error
		user, err = pb.Create(ctx, "users", map[string]any{"email": email, "password": password, "passwordConfirm": password, "name": name})
		if err != nil {
			return err
		}
		tenant, err = pb.Create(ctx, "tenants", map[string]any{"owner_id": user["id"], "name": name + " 的工作区", "slug": cleanTenantSlug(name) + "-" + clip(stringValue(user["id"]), 6)})
		if err != nil {
			return err
		}
		_, err = pb.Create(ctx, "tenant_members", map[string]any{"tenant_id": tenant["id"], "user_id": user["id"], "role": "owner"})
		return err
	})
	if err != nil {
		if user == nil {
			writeError(w, 409, "该邮箱已注册或注册信息无效")
		} else {
			s.Logger.Error("workspace registration failed", "error", err)
			writeError(w, 503, "账号服务暂不可用")
		}
		return
	}
	if s.RequireVerification {
		result, err := s.issueAccountToken(ctx, user, "verify", inviteToken)
		if err != nil {
			writeError(w, 503, "验证邮件发送失败")
			return
		}
		result["requires_verification"], result["email"] = true, email
		writeJSON(w, 201, result)
		return
	}
	auth, err := s.PB.AuthPassword(ctx, email, password)
	if err != nil {
		writeError(w, 503, "账号服务暂不可用")
		return
	}
	writeJSON(w, 201, map[string]any{"token": auth["token"], "user": publicUser(asMap(auth["record"])), "tenant": publicTenant(tenant, "owner"), "needs_onboarding": true})
}

func cleanTenantSlug(name string) string {
	b := strings.Builder{}
	lastDash := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	value := strings.Trim(b.String(), "-")
	if value == "" {
		return "workspace"
	}
	return value
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	input := mapBody(r)
	email := normalizeEmail(stringValue(input["email"]))
	ctx, cancel := contextTimeout(r)
	defer cancel()
	auth, err := s.PB.AuthPassword(ctx, email, stringValue(input["password"]))
	if err != nil {
		writeError(w, 401, "邮箱或密码不正确")
		return
	}
	user := asMap(auth["record"])
	if boolValue(user["disabled"]) {
		writeError(w, 403, "账号已停用，请联系管理员")
		return
	}
	if s.RequireVerification && !boolValue(user["verified"]) {
		writeError(w, 403, "请先验证邮箱后再登录")
		return
	}
	memberships, _, _, err := s.PB.List(ctx, "tenant_members", "user_id = "+pbFilterString(stringValue(user["id"])), "created", 1, 200)
	if err != nil || len(memberships) == 0 {
		writeError(w, 403, "账号没有可访问的工作区")
		return
	}
	sort.SliceStable(memberships, func(i, j int) bool { return memberships[i]["role"] == "owner" && memberships[j]["role"] != "owner" })
	selected := memberships[0]
	tenant, err := s.PB.Get(ctx, "tenants", stringValue(selected["tenant_id"]))
	if err != nil {
		writeError(w, 403, "账号没有可访问的工作区")
		return
	}
	_, appTotal, _, _ := s.PB.List(ctx, "apps", listFilter("tenant_id = "+pbFilterString(stringValue(tenant["id"])), "archived = false"), "-updated", 1, 1)
	workspaces, _ := s.listWorkspaces(ctx, stringValue(user["id"]))
	writeJSON(w, 200, map[string]any{"token": auth["token"], "user": publicUser(user), "tenant": publicTenant(tenant, stringValue(selected["role"])), "workspaces": workspaces, "needs_onboarding": appTotal == 0})
}

func (s *Server) listWorkspaces(ctx context.Context, userID string) ([]map[string]any, error) {
	rows, _, _, err := s.PB.List(ctx, "tenant_members", "user_id = "+pbFilterString(userID), "created", 1, 200)
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, membership := range rows {
		tenant, e := s.PB.Get(ctx, "tenants", stringValue(membership["tenant_id"]))
		if e == nil {
			out = append(out, publicTenant(tenant, stringValue(membership["role"])))
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i]["role"] == "owner" && out[j]["role"] != "owner" })
	return out, nil
}

func (s *Server) verifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	input := mapBody(r)
	token := stringValue(input["token"])
	row, err := s.findAccountToken(ctx, token, "verify")
	if err != nil {
		writeError(w, 400, "验证链接已失效或已使用")
		return
	}
	if _, err = s.PB.Update(ctx, "users", stringValue(row["user_id"]), map[string]any{"verified": true}); err != nil {
		writeError(w, 503, "邮箱验证暂时不可用")
		return
	}
	_ = s.PB.Delete(ctx, "account_tokens", stringValue(row["id"]))
	if continuation := stringValue(input["invite_token"]); continuation != "" {
		writeJSON(w, 200, map[string]any{"ok": true, "invite_token": continuation})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) passwordResetRequest(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	email := normalizeEmail(stringValue(mapBody(r)["email"]))
	if user, err := s.PB.Find(ctx, "users", "email = "+pbFilterString(email)); err == nil && !boolValue(user["disabled"]) && isMailConfigured() {
		_, _ = s.issueAccountToken(ctx, user, "reset", "")
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": "如果该邮箱已注册，密码重置邮件将发送到邮箱。"})
}

func (s *Server) passwordResetConfirm(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextTimeout(r)
	defer cancel()
	input := mapBody(r)
	password := stringValue(input["password"])
	if len(password) < 8 {
		writeError(w, 400, "密码至少 8 位")
		return
	}
	row, err := s.findAccountToken(ctx, stringValue(input["token"]), "reset")
	if err != nil {
		writeError(w, 400, "重置链接已失效或已使用")
		return
	}
	if _, err = s.PB.Update(ctx, "users", stringValue(row["user_id"]), map[string]any{"password": password, "passwordConfirm": password}); err != nil {
		writeError(w, 503, "密码更新失败")
		return
	}
	tokens, _ := s.PB.ListAll(ctx, "account_tokens", listFilter("user_id = "+pbFilterString(stringValue(row["user_id"])), `kind = "reset"`), "")
	for _, item := range tokens {
		_ = s.PB.Delete(ctx, "account_tokens", stringValue(item["id"]))
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	apps, _ := s.PB.ListAll(ctx, "apps", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "archived = false"), "-updated")
	visible := []map[string]any{}
	for _, app := range apps {
		role := s.appPermission(ctx, app, id)
		if role != "" {
			app["permission"] = role
			visible = append(visible, publicApp(app))
		}
	}
	config, _ := s.readAIConfig(ctx)
	writeJSON(w, 200, map[string]any{"user": publicUser(id.User), "tenant": publicTenant(id.Tenant, stringValue(id.Membership["role"])), "workspaces": id.Workspaces, "apps": visible, "ai_configured": config.Key != "", "is_platform_admin": s.Admins[strings.ToLower(stringValue(id.User["email"]))]})
}

func (s *Server) checkLastAdmin(ctx context.Context, email string) bool {
	if !s.Admins[strings.ToLower(email)] {
		return false
	}
	users, err := s.PB.ListAll(ctx, "users", "", "")
	if err != nil {
		return true
	}
	n := 0
	for _, user := range users {
		if s.Admins[strings.ToLower(stringValue(user["email"]))] && !boolValue(user["disabled"]) && boolValue(user["verified"]) {
			n++
		}
	}
	return n <= 1
}
func (s *Server) verifyPassword(ctx context.Context, email, password string) bool {
	_, err := s.PB.AuthPassword(ctx, email, password)
	return err == nil
}

func (s *Server) deactivateAccount(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	input := mapBody(r)
	if !s.verifyPassword(ctx, stringValue(id.User["email"]), stringValue(input["password"])) {
		writeError(w, 401, "密码不正确")
		return
	}
	if s.checkLastAdmin(ctx, stringValue(id.User["email"])) {
		writeError(w, 409, "当前账号是最后一个可用的平台管理员，不能停用；请先配置并验证另一位平台管理员")
		return
	}
	if _, err := s.PB.Update(ctx, "users", stringValue(id.User["id"]), map[string]any{"disabled": true}); err != nil {
		writeError(w, 503, "账号停用失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	input := mapBody(r)
	if stringValue(input["confirm"]) != stringValue(id.User["email"]) {
		writeError(w, 400, "请准确输入账号邮箱以确认删除")
		return
	}
	if !s.verifyPassword(ctx, stringValue(id.User["email"]), stringValue(input["password"])) {
		writeError(w, 401, "密码不正确")
		return
	}
	if s.checkLastAdmin(ctx, stringValue(id.User["email"])) {
		writeError(w, 409, "当前账号是最后一个可用的平台管理员，不能删除；请先配置并验证另一位平台管理员")
		return
	}
	owned, _ := s.PB.ListAll(ctx, "tenants", "owner_id = "+pbFilterString(stringValue(id.User["id"])), "")
	for _, tenant := range owned {
		s.deleteTenantData(ctx, stringValue(tenant["id"]))
	}
	for _, collection := range []string{"tenant_members", "account_tokens", "app_members", "agent_threads", "agent_messages", "automation_notifications"} {
		rows, _ := s.PB.ListAll(ctx, collection, "user_id = "+pbFilterString(stringValue(id.User["id"])), "")
		for _, row := range rows {
			_ = s.PB.Delete(ctx, collection, stringValue(row["id"]))
		}
	}
	_ = s.PB.Delete(ctx, "users", stringValue(id.User["id"]))
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) deleteTenantData(ctx context.Context, tenantID string) {
	tables, _ := s.PB.ListAll(ctx, "app_collections", "tenant_id = "+pbFilterString(tenantID), "")
	for _, table := range tables {
		_ = s.PB.DeleteCollection(ctx, stringValue(table["pb_collection"]))
	}
	for _, name := range []string{"apps", "app_versions", "tenant_invites", "tenant_members", "app_members", "ai_usage", "audit_logs", "miao_run_attempts", "miao_actions", "miao_runs", "miao_tasks", "agent_threads", "agent_messages", "batch_jobs", "automation_rules", "automation_runs", "automation_notifications", "app_files", "miao_record_changes", "app_collections"} {
		rows, _ := s.PB.ListAll(ctx, name, "tenant_id = "+pbFilterString(tenantID), "")
		for _, row := range rows {
			_ = s.PB.Delete(ctx, name, stringValue(row["id"]))
		}
	}
	_ = s.PB.Delete(ctx, "tenants", tenantID)
}

func (s *Server) listMembers(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	rows, _ := s.PB.ListAll(ctx, "tenant_members", "tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "created")
	members := []map[string]any{}
	for _, membership := range rows {
		user, e := s.PB.Get(ctx, "users", stringValue(membership["user_id"]))
		if e == nil {
			item := publicUser(user)
			item["role"], item["membership_id"] = membership["role"], membership["id"]
			members = append(members, item)
		}
	}
	canManage := id.Membership["role"] == "owner" || id.Membership["role"] == "admin"
	writeJSON(w, 200, map[string]any{"members": members, "can_manage": canManage, "can_edit_roles": id.Membership["role"] == "owner"})
}

func (s *Server) listInvites(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireManager(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	rows, _ := s.PB.ListAll(ctx, "tenant_invites", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), `status = "pending"`), "-created")
	out := []map[string]any{}
	for _, row := range rows {
		out = append(out, map[string]any{"id": row["id"], "email": row["email"], "expires_at": row["expires_at"], "created_at": row["created"]})
	}
	writeJSON(w, 200, out)
}

func (s *Server) requireManager(w http.ResponseWriter, id identity) bool {
	role := stringValue(id.Membership["role"])
	if role != "owner" && role != "admin" {
		writeError(w, 403, "只有工作区所有者或管理员可以管理成员")
		return false
	}
	if role == "owner" && id.Tenant["owner_id"] != id.User["id"] {
		writeError(w, 403, "只有工作区所有者可以管理成员")
		return false
	}
	return true
}

func (s *Server) createInvite(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireManager(w, id) {
		return
	}
	email := normalizeEmail(stringValue(mapBody(r)["email"]))
	if !validEmail(email) {
		writeError(w, 400, "请输入有效邮箱")
		return
	}
	if email == normalizeEmail(stringValue(id.User["email"])) {
		writeError(w, 400, "你已经是此工作区的所有者")
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	user, _ := s.PB.Find(ctx, "users", "email = "+pbFilterString(email))
	if user != nil {
		if _, err := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "user_id = "+pbFilterString(stringValue(user["id"])))); err == nil {
			writeError(w, 409, "此用户已在工作区中")
			return
		}
	}
	existing, _ := s.PB.ListAll(ctx, "tenant_invites", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), `status = "pending"`), "")
	active := 0
	for _, invite := range existing {
		if normalizeEmail(stringValue(invite["email"])) == email && parseTime(invite["expires_at"]).After(time.Now()) {
			writeError(w, 409, "此邮箱已有待接受的邀请")
			return
		}
		if parseTime(invite["expires_at"]).After(time.Now()) {
			active++
		}
	}
	if active >= 50 {
		writeError(w, 429, "待接受邀请已达上限，请先撤销或等待现有邀请过期")
		return
	}
	token, err := randomToken()
	if err != nil {
		writeError(w, 503, "邀请创建失败")
		return
	}
	expires := time.Now().Add(72 * time.Hour).UTC().Format(time.RFC3339Nano)
	invite, err := s.PB.Create(ctx, "tenant_invites", map[string]any{"tenant_id": id.Tenant["id"], "email": email, "token_hash": hashToken(token), "expires_at": expires, "invited_by": id.User["id"], "status": "pending"})
	if err != nil {
		writeError(w, 503, "邀请创建失败")
		return
	}
	href := publicURL("/?invite=" + url.QueryEscape(token))
	emailed := false
	if isMailConfigured() {
		if sendMail(ctx, email, "加入 "+stringValue(id.Tenant["name"])+" 工作区", "<p>"+html.EscapeString(stringValue(id.User["name"]))+" 邀请你加入「"+html.EscapeString(stringValue(id.Tenant["name"]))+"」工作区。</p><p><a href=\""+html.EscapeString(href)+"\">接受邀请</a></p><p>链接 72 小时内有效。</p>") == nil {
			emailed = true
		}
	}
	writeJSON(w, 201, map[string]any{"id": invite["id"], "email": email, "expires_at": expires, "invite_url": href, "emailed": emailed})
}

func (s *Server) revokeInvite(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireManager(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	invite, err := s.PB.Get(ctx, "tenant_invites", pathID(r, "id"))
	if err != nil || invite["tenant_id"] != id.Tenant["id"] || invite["status"] != "pending" {
		writeError(w, 404, "邀请不存在")
		return
	}
	if _, err = s.PB.Update(ctx, "tenant_invites", stringValue(invite["id"]), map[string]any{"status": "revoked"}); err != nil {
		writeError(w, 503, "邀请撤销失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	m, err := s.PB.Get(ctx, "tenant_members", pathID(r, "id"))
	if err != nil || m["tenant_id"] != id.Tenant["id"] || m["role"] == "owner" {
		writeError(w, 404, "成员不存在")
		return
	}
	_ = s.PB.Delete(ctx, "tenant_members", stringValue(m["id"]))
	perms, _ := s.PB.ListAll(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "user_id = "+pbFilterString(stringValue(m["user_id"]))), "")
	for _, p := range perms {
		_ = s.PB.Delete(ctx, "app_members", stringValue(p["id"]))
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) updateMember(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireManager(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	m, err := s.PB.Get(ctx, "tenant_members", pathID(r, "id"))
	if err != nil || m["tenant_id"] != id.Tenant["id"] || m["role"] == "owner" {
		writeError(w, 404, "成员不存在")
		return
	}
	input := mapBody(r)
	if _, ok := input["disabled"]; ok {
		writeError(w, 400, "工作区成员管理不能停用全局账号；如需移出此空间，请移除成员")
		return
	}
	role, ok := input["role"]
	if !ok {
		writeError(w, 400, "没有需要更新的成员设置")
		return
	}
	if id.Membership["role"] != "owner" {
		writeError(w, 403, "只有所有者可以调整成员角色")
		return
	}
	if role != "admin" && role != "member" {
		writeError(w, 400, "成员角色无效")
		return
	}
	if _, err = s.PB.Update(ctx, "tenant_members", stringValue(m["id"]), map[string]any{"role": role}); err != nil {
		writeError(w, 503, "成员设置更新失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "role": role})
}

func (s *Server) acceptInvite(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	token := stringValue(mapBody(r)["token"])
	if token == "" || len(token) > 128 {
		writeError(w, 400, "邀请链接无效")
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	invite, err := s.PB.Find(ctx, "tenant_invites", "token_hash = "+pbFilterString(hashToken(token)))
	if err != nil || invite["status"] != "pending" || !parseTime(invite["expires_at"]).After(time.Now()) {
		writeError(w, 400, "邀请已失效或已被撤销")
		return
	}
	if normalizeEmail(stringValue(invite["email"])) != normalizeEmail(stringValue(id.User["email"])) {
		writeError(w, 403, "请使用受邀邮箱登录或注册后接受邀请")
		return
	}
	membership, err := s.PB.Find(ctx, "tenant_members", listFilter("tenant_id = "+pbFilterString(stringValue(invite["tenant_id"])), "user_id = "+pbFilterString(stringValue(id.User["id"]))))
	if err != nil {
		membership, err = s.PB.Create(ctx, "tenant_members", map[string]any{"tenant_id": invite["tenant_id"], "user_id": id.User["id"], "role": "member"})
		if err != nil {
			writeError(w, 503, "加入工作区失败")
			return
		}
	}
	_, _ = s.PB.Update(ctx, "tenant_invites", stringValue(invite["id"]), map[string]any{"status": "accepted"})
	tenant, err := s.PB.Get(ctx, "tenants", stringValue(invite["tenant_id"]))
	if err != nil {
		writeError(w, 503, "工作区暂不可用")
		return
	}
	writeJSON(w, 200, map[string]any{"tenant": publicTenant(tenant, stringValue(membership["role"]))})
}

func (s *Server) aiUsage(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := contextTimeout(r)
	defer cancel()
	start := time.Now().UTC()
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	filter := listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "created >= "+pbFilterString(start.Format(time.RFC3339Nano)))
	rows, _ := s.PB.ListAll(ctx, "ai_usage", filter, "")
	in, out := 0, 0
	for _, row := range rows {
		in += intValue(row["input_tokens"])
		out += intValue(row["output_tokens"])
	}
	writeJSON(w, 200, map[string]any{"day": start.Format("2006-01-02"), "requests": len(rows), "input_tokens": in, "output_tokens": out, "daily_limit": intValue(id.Tenant["ai_daily_limit"]), "can_manage": id.Membership["role"] == "owner"})
}

func (s *Server) aiBudget(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	n := intValue(mapBody(r)["daily_limit"])
	if n < 0 || n > 100000 {
		writeError(w, 400, "每日请求预算必须是 0 到 100000 的整数；0 表示不限制")
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	_, err := s.PB.Update(ctx, "tenants", stringValue(id.Tenant["id"]), map[string]any{"ai_daily_limit": n})
	if err != nil {
		writeError(w, 503, "预算更新失败")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "daily_limit": n})
}

func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	if !s.requireOwner(w, id) {
		return
	}
	ctx, cancel := contextTimeout(r)
	defer cancel()
	page := queryInt(r, "page", 1, 1, 100000)
	rows, total, pages, err := s.PB.List(ctx, "audit_logs", "tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "-created", page, 100)
	if err != nil {
		writeError(w, 503, "审计记录暂不可用")
		return
	}
	items := []map[string]any{}
	for _, row := range rows {
		items = append(items, map[string]any{"id": row["id"], "actor_email": row["actor_email"], "action": row["action"], "route": row["route"], "target_id": row["target_id"], "status": row["status"], "created_at": row["created"]})
	}
	writeJSON(w, 200, map[string]any{"items": items, "page": page, "totalItems": total, "totalPages": pages})
}

func (s *Server) exportWorkspace(w http.ResponseWriter, r *http.Request) {
	id := who(r)
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	apps, _ := s.PB.ListAll(ctx, "apps", "tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "created")
	exported := []map[string]any{}
	for _, app := range apps {
		role := s.appPermission(ctx, app, id)
		if role == "" {
			continue
		}
		tables := s.appTables(ctx, app, stringValue(id.Tenant["id"]))
		tableOut := []map[string]any{}
		for _, table := range tables {
			rows, _ := s.PB.ListAll(ctx, stringValue(table["pb_collection"]), listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "created")
			records := []map[string]any{}
			for _, row := range rows {
				records = append(records, publicRecord(row))
			}
			tableOut = append(tableOut, map[string]any{"name": table["name"], "slug": table["slug"], "fields": table["fields"], "records": records})
		}
		versions, _ := s.PB.ListAll(ctx, "app_versions", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"]))), "version")
		exported = append(exported, map[string]any{"name": app["name"], "description": app["description"], "archived": app["archived"], "published_version_id": app["published_version_id"], "versions": versions, "tables": tableOut})
	}
	w.Header().Set("Content-Disposition", "attachment; filename=\"miao-workspace-"+stringValue(id.Tenant["id"])+".json\"")
	writeJSON(w, 200, map[string]any{"exported_at": nowISO(), "workspace": publicTenant(id.Tenant, stringValue(id.Membership["role"])), "apps": exported})
}
