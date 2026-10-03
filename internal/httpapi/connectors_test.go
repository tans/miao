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
