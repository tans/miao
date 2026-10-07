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

//go:embed 20260928000000_initial.js
var initialScript string

//go:embed 20261007000000_app_view_count.js
var appViewCountScript string

//go:embed 20261007010000_user_language.js
var userLanguageScript string

// scripts pairs each embedded migration with the filename it registers under;
// the guarded view-count script upgrades data directories created before it.
var scripts = []struct{ name, source string }{
	{"20260928000000_initial.js", initialScript},
	{"20261007000000_app_view_count.js", appViewCountScript},
	{"20261007010000_user_language.js", userLanguageScript},
}

func init() {
	for _, script := range scripts {
		register(script.name, script.source)
	}
}

// register runs one migration script in its own VM whose migrate() helper
// files the hooks under the script's filename, matching the jsvm plugin.
func register(name, source string) {
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
		}, name)
	})
	if _, err := vm.RunScript(name, source); err != nil {
		panic(fmt.Errorf("migration %s: %w", name, err))
	}
}
