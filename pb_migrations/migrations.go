// Package migrations embeds the pre-launch database initialization schema.
package migrations

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/dop251/goja"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/jsvm"
)

const initialName = "20260928000000_initial.js"

//go:embed 20260928000000_initial.js
var initialScript string

func init() {
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
		}, initialName)
	})
	if _, err := vm.RunScript(initialName, initialScript); err != nil {
		panic(fmt.Errorf("initial schema %s: %w", initialName, err))
	}
}
