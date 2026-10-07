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
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tans/miao/internal/httpapi"
	"github.com/tans/miao/internal/pocketbase"
	"github.com/tans/miao/internal/runtime"
	"github.com/tans/miao/internal/settings"
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
	if command != "serve" && command != "backup" && command != "restore" && command != "admin-grant" {
		return errors.New("usage: miao [serve|version|backup|restore ARCHIVE --confirm|admin-grant EMAIL...]")
	}
	if command != "restore" && command != "admin-grant" && len(args) > 1 {
		return errors.New("unexpected command arguments; manage business settings through the admin console")
	}
	root := setting("MIAO_DATA_DIR", "data")
	if command == "restore" {
		previous, err := runtime.Restore(context.Background(), root, args[1])
		if err == nil {
			fmt.Println("Restored and isolated historical tasks. Service remains stopped. Previous data:", previous)
		}
		return err
	}
	if command == "admin-grant" {
		if len(args) < 2 {
			return errors.New("usage: miao admin-grant EMAIL [EMAIL...]; run while the service is stopped")
		}
		return grantAdmins(root, args[1:])
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
		offline, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		importSettings(offline, app.Store)
		policy, err := settings.ReadBackup(offline, app.Store)
		if err != nil {
			return err
		}
		target, err := app.Backup(context.Background(), policy.Directory, policy.RetentionDays)
		if err == nil {
			fmt.Println("Backup saved:", target)
		}
		return err
	}
	api := httpapi.New(app.Store)
	bootstrap, bootstrapCancel := context.WithTimeout(context.Background(), time.Minute)
	importSettings(bootstrap, app.Store)
	bootstrapCancel()
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

// importSettings performs the one-time bootstrap import of environment
// variables into platform_settings; failures are logged and retried next start.
func importSettings(ctx context.Context, store *pocketbase.Client) {
	for _, result := range settings.ImportEnvironment(ctx, store) {
		slog.Info("settings bootstrap", "group", result.Group, "result", result.Result)
	}
}

// grantAdmins is the minimal recovery entry for platform admin access. It
// merges the given emails into the effective admin list and records the change.
func grantAdmins(root string, rawEmails []string) error {
	emails, err := settings.NormalizeAdminEmails(rawEmails)
	if err != nil {
		return err
	}
	app, err := runtime.New(root)
	if err != nil {
		return err
	}
	defer app.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	importSettings(ctx, app.Store)
	current, err := settings.ReadAdmins(ctx, app.Store)
	if err != nil {
		return err
	}
	merged := append([]string{}, current.Emails...)
	for _, email := range emails {
		if !slices.Contains(merged, email) {
			merged = append(merged, email)
		}
	}
	emails, err = settings.NormalizeAdminEmails(merged)
	if err != nil {
		return err
	}
	err = app.Store.Transaction(ctx, func(tx *pocketbase.Client) error {
		if err := settings.Put(ctx, tx, settings.RowAdmins, settings.MarshalAdmins(emails), "system"); err != nil {
			return err
		}
		_, err := tx.Create(ctx, "platform_audit_logs", map[string]any{
			"actor_id": "system", "actor_email": "system@miao.local", "action": "settings.admins.granted",
			"target_type": "setting", "target_id": "admins", "reason": "通过 admin-grant 恢复入口写入", "status": 200,
		})
		return err
	})
	if err != nil {
		return err
	}
	fmt.Println("Platform admins:", strings.Join(emails, ", "))
	return nil
}
