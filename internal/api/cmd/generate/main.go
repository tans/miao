// Command generate writes the checked-in API documentation artifacts.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tans/miao/internal/api"
)

func main() {
	root := findRoot()
	output := filepath.Join(root, "docs", "generated")
	if err := os.MkdirAll(output, 0o755); err != nil {
		panic(err)
	}
	files := map[string]any{
		"openapi.json":       api.OpenAPI(),
		"api-schema.json":    map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://miao.local/schema/api.json", "title": "MIAO API schemas", "$defs": api.Schemas()},
		"config-schema.json": api.ConfigSchema(),
	}
	for name, value := range files {
		data, err := api.JSON(value)
		if err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(output, name), data, 0o644); err != nil {
			panic(err)
		}
	}
	if err := os.WriteFile(filepath.Join(output, "api.md"), []byte(api.Markdown()), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("generated API documentation in %s\n", output)
}

func findRoot() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("go.mod not found")
		}
	}
}
