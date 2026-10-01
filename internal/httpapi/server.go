package httpapi

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/tans/miao/internal/pocketbase"
)

type Server struct {
	PB                  *pocketbase.Client
	Mux                 *http.ServeMux
	Logger              *slog.Logger
	Admins              map[string]bool
	RegistrationMode    string
	AllowedDomains      map[string]bool
	RequireVerification bool

	background      *backgroundState
	workerMu        sync.Mutex
	workerID        string
	workerLockID    string
	workerRecovered bool
	activeRun       string
	activeCancel    context.CancelFunc
	workerWG        sync.WaitGroup
}

func New(pb *pocketbase.Client) *Server {
	admins, domains := map[string]bool{}, map[string]bool{}
	for _, email := range strings.Split(os.Getenv("MIAO_ADMIN_EMAILS"), ",") {
		if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
			admins[email] = true
		}
	}
	for _, domain := range strings.Split(os.Getenv("MIAO_ALLOWED_EMAIL_DOMAINS"), ",") {
		if domain = strings.ToLower(strings.TrimSpace(domain)); domain != "" {
			domains[domain] = true
		}
	}
	workerID, _ := randomToken()
	s := &Server{PB: pb, Mux: http.NewServeMux(), Logger: slog.Default(), Admins: admins, AllowedDomains: domains, workerID: workerID,
		RegistrationMode: env("MIAO_REGISTRATION_MODE", "open"), RequireVerification: os.Getenv("MIAO_REQUIRE_EMAIL_VERIFICATION") == "true", background: &backgroundState{}}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.Mux.ServeHTTP(w, r) }

func (s *Server) Handler(assets fs.FS) (http.Handler, error) {
	root, err := fs.Sub(assets, "public")
	if err != nil {
		return nil, err
	}
	static := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.Mux.ServeHTTP(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "." || name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(root, name); err != nil {
			if strings.Contains(path.Base(name), ".") {
				http.NotFound(w, r)
				return
			}
			r2 := r.Clone(r.Context())
			u := *r.URL
			// FileServer redirects /index.html to ./; serving / preserves the
			// browser's deep link and query while selecting the same index file.
			u.Path, u.RawPath = "/", ""
			r2.URL = &u
			static.ServeHTTP(w, r2)
			return
		}
		static.ServeHTTP(w, r)
	}), nil
}

type backgroundState struct {
	once   sync.Once
	cancel context.CancelFunc
	done   chan struct{}
}

func (s *Server) StartBackground(parent context.Context) {
	if s.background == nil {
		s.background = &backgroundState{}
	}
	s.background.once.Do(func() {
		ctx, cancel := context.WithCancel(parent)
		s.background.cancel = cancel
		s.background.done = make(chan struct{})
		go func() {
			defer close(s.background.done)
			ticker := time.NewTicker(20 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					scanCtx, stop := context.WithTimeout(ctx, 45*time.Second)
					s.scanDueAutomation(scanCtx)
					stop()
					s.runQueuedTasks(ctx)
				}
			}
		}()
	})
}

// StopBackground waits for the scan loop before waiting for its spawned worker.
// The database remains open until both have saved their interruption state.
func (s *Server) StopBackground(ctx context.Context) error {
	if s.background.cancel == nil {
		return nil
	}
	s.background.cancel()
	select {
	case <-s.background.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	done := make(chan struct{})
	go func() { s.workerWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	lock, err := s.PB.Find(ctx, "miao_runtime_locks", "owner = "+pbFilterString(s.workerID))
	if err == nil {
		return s.PB.Delete(ctx, "miao_runtime_locks", stringValue(lock["id"]))
	}
	var missing *pocketbase.Error
	if errors.As(err, &missing) && missing.Status == 404 {
		return nil
	}
	return err
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
		w.ResponseWriter.WriteHeader(code)
	}
}
func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

func (s *Server) routes() {
	s.Mux.HandleFunc("GET /api/health", s.health)
	s.registerAuthRoutes()
	s.routesApps()
	s.routesAgent()
	s.routesVersions()
	s.routesFiles()
	s.routesOperations()
	s.routesAutomations()
	s.routesTasks()
	s.routesAdmin()
	s.Mux.HandleFunc("/api/fx/gateway", s.auth(s.fxGateway))
	s.Mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "接口不存在") })
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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
		if s.RequireVerification && !boolValue(user["verified"]) {
			writeError(w, 403, "请先验证邮箱后再登录")
			return
		}
		memberships, _, _, err := s.PB.List(ctx, "tenant_members", "user_id = "+pbFilterString(stringValue(user["id"])), "created", 1, 200)
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
		id := identity{User: user, Tenant: asMap(current["tenant"]), Membership: asMap(current["membership"]), Workspaces: visible, Token: fresh, FreshToken: fresh}
		next(w, r.WithContext(withIdentity(r.Context(), id)))
	}
}

func (s *Server) appPermission(ctx context.Context, app map[string]any, id identity) string {
	if app["tenant_id"] != id.Tenant["id"] {
		return ""
	}
	if id.Membership["role"] == "owner" && id.Tenant["owner_id"] == id.User["id"] {
		return "owner"
	}
	permission, err := s.PB.Find(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "user_id = "+pbFilterString(stringValue(id.User["id"]))))
	if err != nil {
		var missing *pocketbase.Error
		if errors.As(err, &missing) && missing.Status == 404 && !boolValue(app["restricted"]) {
			return "editor"
		}
		return ""
	}
	return stringValue(permission["role"])
}

func (s *Server) appForRequest(ctx context.Context, r *http.Request) (map[string]any, string, error) {
	id := who(r)
	app, err := s.PB.Get(ctx, "apps", pathID(r, "id"))
	if err != nil {
		return nil, "", err
	}
	if app["tenant_id"] != id.Tenant["id"] {
		return nil, "", context.Canceled
	}
	role := s.appPermission(ctx, app, id)
	if role == "" {
		return nil, "", context.Canceled
	}
	app["permission"] = role
	return app, role, nil
}

func canPublishAppRole(role string) bool { return role == "owner" || role == "publisher" }
func canManageAppRole(role string) bool {
	return role == "owner" || role == "manager" || role == "publisher"
}

func (s *Server) appCanBatch(ctx context.Context, app map[string]any, id identity) bool {
	if app["tenant_id"] != id.Tenant["id"] {
		return false
	}
	if id.Membership["role"] == "owner" && id.Tenant["owner_id"] == id.User["id"] {
		return true
	}
	permission, err := s.PB.Find(ctx, "app_members", listFilter("tenant_id = "+pbFilterString(stringValue(id.Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "user_id = "+pbFilterString(stringValue(id.User["id"]))))
	return err == nil && boolValue(permission["can_batch"]) && stringValue(permission["role"]) != "viewer"
}

func (s *Server) appTables(ctx context.Context, app map[string]any, tenantID string) []map[string]any {
	if tenantID == "" {
		tenantID = stringValue(app["tenant_id"])
	}
	rows, _ := s.PB.ListAll(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(tenantID), "app_id = "+pbFilterString(stringValue(app["id"]))), "created")
	return rows
}

func (s *Server) tableForRequest(ctx context.Context, r *http.Request, app map[string]any) (map[string]any, error) {
	return s.PB.Find(ctx, "app_collections", listFilter("tenant_id = "+pbFilterString(stringValue(who(r).Tenant["id"])), "app_id = "+pbFilterString(stringValue(app["id"])), "slug = "+pbFilterString(pathID(r, "slug"))))
}

func (s *Server) requireOwner(w http.ResponseWriter, id identity) bool {
	if id.Membership["role"] != "owner" || id.Tenant["owner_id"] != id.User["id"] {
		writeError(w, 403, "只有工作区所有者可以管理此设置")
		return false
	}
	return true
}
