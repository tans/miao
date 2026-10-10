package httpapi

import (
	"compress/gzip"
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tans/miao/internal/settings"
)

// Shared HTTP middleware and request-level helpers. server.go keeps only the
// Server lifecycle and the top-level Handler; every cross-cutting boundary
// (transport compression, authentication, admin gating, health) lives here.

type gzipResponseWriter struct {
	http.ResponseWriter
	writer     *gzip.Writer
	statusCode int
	started    bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.statusCode != 0 {
		return
	}
	w.statusCode = status
	if status != http.StatusOK {
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if w.statusCode != 0 && w.statusCode != http.StatusOK {
		return w.ResponseWriter.Write(data)
	}
	if !w.started {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		if w.statusCode == 0 {
			w.statusCode = http.StatusOK
		}
		w.ResponseWriter.WriteHeader(w.statusCode)
		w.started = true
	}
	return w.writer.Write(data)
}

func (w *gzipResponseWriter) Close() error {
	if !w.started {
		return nil
	}
	return w.writer.Close()
}

func (w *gzipResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func acceptsGzip(r *http.Request) bool {
	gzipQ, wildcardQ := -1.0, -1.0
	for _, item := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		parts := strings.Split(strings.TrimSpace(item), ";")
		name := strings.TrimSpace(parts[0])
		q := 1.0
		for _, parameter := range parts[1:] {
			key, value, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if found && strings.EqualFold(strings.TrimSpace(key), "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					q = 0
				} else {
					q = parsed
				}
			}
		}
		switch strings.ToLower(name) {
		case "gzip":
			gzipQ = q
		case "*":
			wildcardQ = q
		}
	}
	if gzipQ >= 0 {
		return gzipQ > 0
	}
	return wildcardQ > 0
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	if err := s.PB.Health(ctx); err != nil {
		writeError(w, 503, "PocketBase unavailable")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "service": "miao", "persistence": "pocketbase", "runtime": "go", "time": nowISO()})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if token == "" {
			writeError(w, 401, "请先登录")
			return
		}
		ctx, cancel := contextTimeout(r)
		defer cancel()
		data, err := s.PB.AuthRefresh(ctx, token)
		if err != nil {
			writeError(w, 401, "请先登录")
			return
		}
		user, fresh := asMap(data["record"]), stringValue(data["token"])
		if len(user) == 0 {
			writeError(w, 401, "请先登录")
			return
		}
		if boolValue(user["disabled"]) {
			writeError(w, 403, "账号已停用，请联系管理员")
			return
		}
		registration, err := settings.ReadRegistration(ctx, s.PB)
		if err != nil {
			s.Logger.Error("registration settings lookup failed", "error", err)
			writeError(w, 503, "平台设置暂不可用")
			return
		}
		if registration.RequireEmailVerification && !boolValue(user["verified"]) {
			writeError(w, 403, "请先验证邮箱后再登录")
			return
		}
		memberships, err := s.PB.ListAll(ctx, "tenant_members", "user_id = "+pbFilterString(stringValue(user["id"])), "created")
		if err != nil {
			s.Logger.Error("workspace membership lookup failed", "error", err)
			writeError(w, 503, "工作区暂不可用")
			return
		}
		workspaces := make([]map[string]any, 0, len(memberships))
		selected := -1
		selectedID := r.Header.Get("X-Miao-Tenant-Id")
		for _, membership := range memberships {
			tenant, e := s.PB.Get(ctx, "tenants", stringValue(membership["tenant_id"]))
			if e != nil {
				continue
			}
			workspaces = append(workspaces, map[string]any{"tenant": tenant, "membership": membership})
			if selectedID != "" && stringValue(tenant["id"]) == selectedID {
				selected = len(workspaces) - 1
			}
			if selectedID == "" && selected < 0 && membership["role"] == "owner" {
				selected = len(workspaces) - 1
			}
		}
		if len(workspaces) == 0 {
			writeError(w, 403, "账号没有可访问的工作区")
			return
		}
		if selected < 0 && selectedID == "" {
			selected = 0
		}
		if selected < 0 {
			writeError(w, 403, "你没有权限访问这个工作区")
			return
		}
		current := workspaces[selected]
		visible := []map[string]any{}
		for _, item := range workspaces {
			visible = append(visible, publicTenant(asMap(item["tenant"]), stringValue(asMap(item["membership"])["role"])))
		}
		w.Header().Set("X-PocketBase-Token", fresh)
		// A saved account language wins over cookie/browser negotiation unless
		// the request carries an explicit ?lang= override.
		if override := normalizeLanguage(r.URL.Query().Get("lang")); override == "" {
			if lang := normalizeLanguage(stringValue(user["language"])); lang != "" {
				if lw, ok := w.(*languageWriter); ok {
					lw.setLanguage(lang)
				}
				r = r.WithContext(withLanguage(r.Context(), lang))
			}
		}
		id := identity{User: user, Tenant: asMap(current["tenant"]), Membership: asMap(current["membership"]), Workspaces: visible, Token: fresh, FreshToken: fresh}
		next(w, r.WithContext(withIdentity(r.Context(), id)))
	}
}

// isAdmin resolves platform admin authorization from the settings service on
// every administrative request, so console edits take effect immediately.
func (s *Server) isAdmin(ctx context.Context, email string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return false
	}
	emails, err := settings.ReadAdmins(ctx, s.PB)
	if err != nil {
		s.Logger.Error("platform admin lookup failed", "error", err)
		return false
	}
	return containsString(emails.Emails, email)
}

// adminAuth layers the platform-admin check on top of user auth. It is the
// single authorization boundary for every /api/admin route.
func (s *Server) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return s.auth(func(w http.ResponseWriter, r *http.Request) {
		id := who(r)
		ctx, cancel := contextTimeout(r)
		defer cancel()
		if !s.isAdmin(ctx, stringValue(id.User["email"])) {
			s.writeAdminAudit(r.Context(), id, "access.denied", "endpoint", clip(r.URL.Path, 64), "", 403)
			writeError(w, 403, "无权访问平台管理功能")
			return
		}
		next(w, r)
	})
}
