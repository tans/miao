// Package migrations embeds immutable migrations with their original filenames,
// so an existing PocketBase data directory upgrades without reapplying them.
package migrations

import (
	"embed"
	"fmt"
	"github.com/dop251/goja"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
	"sync"
)

//go:embed *.js
var scripts embed.FS

func init() {
	entries, err := scripts.ReadDir(".")
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		data, err := scripts.ReadFile(entry.Name())
		if err != nil {
			panic(err)
		}
		vm := goja.New()
		jsvm.BindCore(vm)
		jsvm.BindDbx(vm)
		jsvm.BindOS(vm)
		jsvm.BindSecurity(vm)
		var mu sync.Mutex
		_ = vm.Set("migrate", func(up, down func(core.App) error) {
			core.AppMigrations.Register(func(app core.App) error {
				mu.Lock()
				defer mu.Unlock()
				return up(app)
			}, func(app core.App) error {
				mu.Lock()
				defer mu.Unlock()
				if down == nil {
					return nil
				}
				return down(app)
			}, entry.Name())
		})
		if _, err := vm.RunScript(entry.Name(), string(data)); err != nil {
			panic(fmt.Errorf("migration %s: %w", entry.Name(), err))
		}
	}
}
