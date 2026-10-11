package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	stdhtml "html"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/tans/miao/internal/pocketbase"
	"github.com/tans/miao/internal/settings"
)

type Server struct {
	PB     *pocketbase.Client
	Mux    *http.ServeMux
	Logger *slog.Logger

	background      *backgroundState
	workerMu        sync.Mutex
	workerID        string
	workerLockID    string
	workerRecovered bool
	activeRuns      map[string]context.CancelFunc
	activeTasks     map[string]string
	workerWG        sync.WaitGroup
}

func New(pb *pocketbase.Client) *Server {
	workerID, _ := randomToken()
	s := &Server{PB: pb, Mux: http.NewServeMux(), Logger: slog.Default(), workerID: workerID, activeRuns: map[string]context.CancelFunc{}, activeTasks: map[string]string{}, background: &backgroundState{}}
	s.registerRoutes()
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
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/s/") {
			writer, req := wrapLanguage(w, r)
			if strings.HasPrefix(r.URL.Path, "/api/") {
				s.Mux.ServeHTTP(writer, req)
			} else {
				s.servePublicSite(writer, req, root)
			}
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "." || name == "" {
			name = "index.html"
		}
		_, assetErr := fs.Stat(root, name)
		indexFallback := assetErr != nil && !strings.Contains(path.Base(name), ".")
		compressible := indexFallback || (assetErr == nil && (strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".css") || strings.HasSuffix(name, ".html")))
		if compressible {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Add("Vary", "Accept-Encoding")
			if acceptsGzip(r) && r.Method != http.MethodHead && r.Header.Get("Range") == "" {
				w.Header().Del("Content-Length")
				gz := gzip.NewWriter(w)
				gzipWriter := &gzipResponseWriter{ResponseWriter: w, writer: gz}
				defer gzipWriter.Close()
				w = gzipWriter
			}
		}
		if assetErr != nil {
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
			s.serveIndex(w, r2, root)
			return
		}
		if name == "index.html" {
			s.serveIndex(w, r, root)
			return
		}
		static.ServeHTTP(w, r)
	}), nil
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request, root fs.FS) {
	data, err := fs.ReadFile(root, "index.html")
	if err != nil {
		http.Error(w, "index unavailable", http.StatusInternalServerError)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	branding := settings.Branding{Title: "MIAO · 企业内部工具"}
	if resolved, readErr := settings.ReadBranding(ctx, s.PB); readErr == nil {
		branding = resolved
	}
	data = bytes.ReplaceAll(data, []byte("MIAO_TITLE"), []byte(stdhtml.EscapeString(branding.Title)))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
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
					for _, stage := range []struct {
						name string
						run  func(context.Context)
					}{
						{"scanDueAutomation", func(ctx context.Context) {
							scanCtx, stop := context.WithTimeout(ctx, 45*time.Second)
							s.scanDueAutomation(scanCtx)
							stop()
						}},
						{"runQueuedTasks", s.runQueuedTasks},
						{"runQueuedHarness", s.runQueuedHarness},
					} {
						s.runScanStage(ctx, stage.name, stage.run)
					}
				}
			}
		}()
	})
}

// runScanStage isolates one background scan stage: a panic in a single scan
// is logged and the loop continues with the next tick instead of dying.
func (s *Server) runScanStage(ctx context.Context, name string, run func(context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			s.Logger.Error("background scan stage panicked", "stage", name, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	run(ctx)
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
