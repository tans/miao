package main

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tans/miao/internal/httpapi"
	"github.com/tans/miao/internal/pocketbase"
)

//go:embed public
var staticFiles embed.FS

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	pbURL := setting("POCKETBASE_URL", "http://127.0.0.1:8090")
	pb := pocketbase.New(pbURL, os.Getenv("POCKETBASE_SUPERUSER_EMAIL"), os.Getenv("POCKETBASE_SUPERUSER_PASSWORD"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	if _, err := pb.Request(ctx, http.MethodGet, "/api/health", nil, nil, ""); err != nil {
		cancel()
		logger.Error("PocketBase is not available", "error", err)
		os.Exit(1)
	}
	runtime, err := pb.Request(ctx, http.MethodGet, "/api/miao/runtime", nil, nil, "")
	cancel()
	if err != nil || runtime["atomic_record_events"] != true || runtime["record_version_check"] != true {
		logger.Error("PocketBase MIAO hooks are not installed; start through scripts/start.sh")
		os.Exit(1)
	}
	api := httpapi.New(pb)
	assets, err := staticFiles.ReadDir("public")
	if err != nil || len(assets) == 0 {
		logger.Error("embedded UI assets are missing")
		os.Exit(1)
	}
	handler, err := api.Handler(staticFiles)
	if err != nil {
		logger.Error("failed to initialize UI assets", "error", err)
		os.Exit(1)
	}
	port, _ := strconv.Atoi(setting("PORT", setting("MIAO_PORT", "41874")))
	if port < 1 || port > 65535 { logger.Error("invalid MIAO port"); os.Exit(1) }
	server := &http.Server{Addr: strings.TrimSpace(setting("HOST", "0.0.0.0"))+":"+strconv.Itoa(port), Handler: handler, ReadHeaderTimeout: 10*time.Second, IdleTimeout: 90*time.Second, MaxHeaderBytes: 1<<20}
	root, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	api.StartBackground(root)
	go func() {
		<-root.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	logger.Info("MIAO listening", "address", server.Addr, "runtime", "go", "persistence", "pocketbase")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) { logger.Error("MIAO stopped", "error", err); os.Exit(1) }
}

func setting(key, fallback string) string { if value := strings.TrimSpace(os.Getenv(key)); value != "" { return value }; return fallback }
