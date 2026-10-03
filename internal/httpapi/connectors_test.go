package httpapi

import (
	"context"
	"testing"
)

func TestNormalizeConnectorDefinitionRestrictsNetworkScope(t *testing.T) {
	definition, msg := normalizeConnectorDefinition(map[string]any{
		"base_url":      "https://example.com",
		"allowed_paths": []any{"/api/v1", "/health"},
		"max_bytes":     4096,
	})
	if msg != "" || definition["type"] != "https_fetch" || definition["max_bytes"] != 4096 {
		t.Fatalf("valid connector rejected: %v %v", definition, msg)
	}
	if !connectorPathAllowed(definition, "/api/v1/items") || connectorPathAllowed(definition, "/api/private") {
		t.Fatal("connector path prefix policy was not enforced")
	}
	for _, raw := range []map[string]any{
		{"base_url": "http://example.com"},
		{"base_url": "https://user:secret@example.com"},
		{"base_url": "https://example.com:8443"},
		{"base_url": "https://example.com/api"},
		{"base_url": "https://example.com/?token=secret"},
		{"base_url": "https://example.com", "allowed_paths": []any{"/../private"}},
	} {
		if _, msg := normalizeConnectorDefinition(raw); msg == "" {
			t.Fatalf("unsafe connector definition accepted: %v", raw)
		}
	}
}

func TestConnectorURLAndPublicAddressGuards(t *testing.T) {
	definition, msg := normalizeConnectorDefinition(map[string]any{"base_url": "https://example.com", "allowed_paths": []any{"/api"}})
	if msg != "" {
		t.Fatal(msg)
	}
	u, err := connectorURL(definition, "/api/items?cursor=1")
	if err != nil || u.String() != "https://example.com/api/items?cursor=1" {
		t.Fatalf("unexpected connector URL: %v %v", u, err)
	}
	if err := publicNetworkHost(context.Background(), "127.0.0.1"); err == nil {
		t.Fatal("loopback address was allowed")
	}
	if err := publicNetworkHost(context.Background(), "10.0.0.1"); err == nil {
		t.Fatal("private address was allowed")
	}
}

func TestConnectorExtractJSONPointer(t *testing.T) {
	extract, msg := normalizeConnectorExtract(map[string]any{
		"format": "json", "items_path": "/data/items",
		"fields": map[string]any{"name": "/title", "url": "/links/0/href", "id": "/id"},
	})
	if msg != "" {
		t.Fatal(msg)
	}
	rows, err := extractJSONRecords([]byte(`{"data":{"items":[{"id":7,"title":"One","links":[{"href":"/one"}]},{"id":8,"title":"Two"}]}}`), extract)
	if err != nil || len(rows) != 2 || rows[0]["name"] != "One" || rows[0]["url"] != "/one" || rows[1]["url"] != nil {
		t.Fatalf("unexpected extraction: %#v %v", rows, err)
	}
}

func TestConnectorExtractHTMLSelectors(t *testing.T) {
	extract, msg := normalizeConnectorExtract(map[string]any{
		"format": "html", "item_selector": "article.item",
		"fields": map[string]any{
			"title": map[string]any{"selector": ".title"},
			"url":   map[string]any{"selector": "a", "attribute": "href"},
		},
	})
	if msg != "" {
		t.Fatal(msg)
	}
	body := []byte(`<html><body><article class="item"><h2 class="title"> First item </h2><a href="/one">open</a></article><article class="item"><h2 class="title">Second</h2><a href="/two">open</a></article></body></html>`)
	rows, err := extractHTMLRecords(body, extract)
	if err != nil || len(rows) != 2 || rows[0]["title"] != "First item" || rows[1]["url"] != "/two" {
		t.Fatalf("unexpected extraction: %#v %v", rows, err)
	}
	if _, msg := normalizeConnectorExtract(map[string]any{"format": "html", "item_selector": "article .item", "fields": map[string]any{"title": map[string]any{"selector": ".title"}}}); msg == "" {
		t.Fatal("unsupported selector syntax accepted")
	}
}
