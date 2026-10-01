// Package runtime owns the embedded PocketBase lifecycle and local operations.
package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/pocketbase/pocketbase/core"
	_ "github.com/pocketbase/pocketbase/migrations"
	store "github.com/tans/miao/internal/pocketbase"
	_ "github.com/tans/miao/pb_migrations"
)

type Runtime struct {
	App   core.App
	Store *store.Client
	Root  string
	lock  *os.File
}

func lockData(root string) (*os.File, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(root, "miao.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("MIAO data directory is in use; stop the service before offline operations: %w", err)
	}
	return lock, nil
}
func openApp(dataDir string) (core.App, *store.Client, error) {
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: dataDir, EncryptionEnv: "MIAO_SETTINGS_ENCRYPTION_KEY"})
	store.BindBusinessEvents(app)
	if err := app.Bootstrap(); err != nil {
		app.ResetBootstrapState()
		return nil, nil, err
	}
	if err := app.RunAppMigrations(); err != nil {
		app.ResetBootstrapState()
		return nil, nil, fmt.Errorf("application migrations: %w", err)
	}
	client, err := store.New(app)
	if err != nil {
		app.ResetBootstrapState()
		return nil, nil, err
	}
	return app, client, nil
}
func New(root string) (*Runtime, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	lock, err := lockData(root)
	if err != nil {
		return nil, err
	}
	app, client, err := openApp(filepath.Join(root, "pb_data"))
	if err != nil {
		lock.Close()
		return nil, err
	}
	return &Runtime{App: app, Store: client, Root: root, lock: lock}, nil
}
func (r *Runtime) Close() error {
	err := r.App.ResetBootstrapState()
	if r.lock != nil {
		if closeErr := r.lock.Close(); err == nil {
			err = closeErr
		}
		r.lock = nil
	}
	return err
}
