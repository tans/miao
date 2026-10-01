package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tans/miao/internal/httpapi"
	"github.com/tans/miao/internal/runtime"
)

//go:embed public
var staticFiles embed.FS

var version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(os.Args[1:]); err != nil {
		slog.Error("MIAO stopped", "error", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	command := "serve"
	if len(args) > 0 {
		command = args[0]
	}
	if command == "version" {
		fmt.Println("MIAO", version, "(embedded PocketBase 0.40.4)")
		return nil
	}
	if command == "restore" && (len(args) != 3 || args[2] != "--confirm") {
		return errors.New("usage: miao restore miao_backup_*.zip --confirm; stop the service and make a current backup first")
	}
	if command != "serve" && command != "backup" && command != "restore" {
		return errors.New("usage: miao [serve|version|backup|restore ARCHIVE --confirm]")
	}
	if command != "restore" && len(args) > 1 {
		return errors.New("unexpected command arguments; configure the runtime through MIAO environment variables")
	}
	root := setting("MIAO_DATA_DIR", "data")
	if command == "restore" {
		previous, err := runtime.Restore(context.Background(), root, args[1])
		if err == nil {
			fmt.Println("Restored and isolated historical tasks. Service remains stopped. Previous data:", previous)
		}
		return err
	}
	app, err := runtime.New(root)
	if err != nil {
		return err
	}
	closeRuntime := true
	defer func() {
		if closeRuntime {
			app.Close()
		}
	}()
	if command == "backup" {
		days, err := strconv.Atoi(setting("MIAO_BACKUP_RETENTION_DAYS", "30"))
		if err != nil || days < 1 {
			return errors.New("MIAO_BACKUP_RETENTION_DAYS must be a positive integer")
		}
		target, err := app.Backup(context.Background(), setting("MIAO_BACKUP_DIR", ""), days)
		if err == nil {
			fmt.Println("Backup saved:", target)
		}
		return err
	}
	api := httpapi.New(app.Store)
	handler, err := api.Handler(staticFiles)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(setting("PORT", setting("MIAO_PORT", "41874")))
	if err != nil || port < 1 || port > 65535 {
		return errors.New("invalid MIAO port")
	}
	server := &http.Server{Addr: net.JoinHostPort(setting("HOST", "0.0.0.0"), strconv.Itoa(port)), Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	api.StartBackground(rootCtx)
	shutdownDone := make(chan error, 1)
	go func() {
		<-rootCtx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		shutdownDone <- errors.Join(server.Shutdown(shutdown), api.StopBackground(shutdown))
	}()
	slog.Info("MIAO listening", "address", server.Addr, "runtime", "go", "persistence", "embedded-pocketbase", "version", version)
	err = server.ListenAndServe()
	stop()
	shutdownErr := <-shutdownDone
	// If shutdown timed out, leave connections to OS cleanup when main exits.
	// Closing SQLite while a worker or request is still active would race writes.
	if shutdownErr != nil {
		closeRuntime = false
	}
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	return errors.Join(err, shutdownErr)
}
func setting(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
