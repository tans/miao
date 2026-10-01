package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tans/miao/internal/httpapi"
	"github.com/tans/miao/internal/runtime"
)

func TestEmbeddedUIAndPrivatePocketBase(t *testing.T) {
	app, err := runtime.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	handler, err := httpapi.New(app.Store).Handler(staticFiles)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		url    string
		status int
	}{{"/", 200}, {"/apps/example", 200}, {"/reset-password?token=example", 200}, {"/admin/users", 200}, {"/app.js", 200}, {"/missing.js", 404}, {"/api/collections/users/records", 404}, {"/api/health", 200}} {
		request := httptest.NewRequest("GET", check.url, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != check.status {
			t.Fatalf("%s: %d want %d", check.url, response.Code, check.status)
		}
	}
}
func TestInvalidCLIAndUnconfirmedRestoreDoNotOpenData(t *testing.T) {
	root := filepath.Join(t.TempDir(), "untouched")
	t.Setenv("MIAO_DATA_DIR", root)
	for _, args := range [][]string{{"restore"}, {"restore", "missing.zip"}, {"restore", "missing.zip", "no"}, {"unsupported"}, {"serve", "--dir=other"}} {
		if err := run(args); err == nil {
			t.Fatalf("invalid arguments accepted: %v", args)
		}
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("unconfirmed command opened data", err)
	}
}
