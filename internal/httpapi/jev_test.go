package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEvaluateJevRejectsUnavailableChoice(t *testing.T) {
	f := newIntegration(t)
	t.Setenv("AI_GATEWAY_API_KEY", "test-key")
	t.Setenv("MIAO_JEV_API_KEY", "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"answers":{"candidate_id":{"type":"choice","choice":"not-allowed"}}}`))
	}))
	defer server.Close()
	previousURL := jevEvaluationURL
	jevEvaluationURL = server.URL
	t.Cleanup(func() { jevEvaluationURL = previousURL })

	_, err := f.api.evaluateJev(context.Background(), f.tenantID, f.userID, f.base[len("/api/apps/"):], map[string]any{"prompt": "choose"}, map[string]jevQuestion{
		"candidate_id": {Type: "choice", Criteria: map[string]string{"allowed": "The only legal candidate"}},
	})
	if err == nil || !strings.Contains(err.Error(), "unavailable choice") {
		t.Fatalf("evaluateJev accepted an unavailable choice: %v", err)
	}
}

func TestJevDoesNotSendCapiCredentialsToGateway(t *testing.T) {
	f := newIntegration(t)
	t.Setenv("MIAO_AI_PROVIDER", "capi")
	t.Setenv("AI_GATEWAY_API_KEY", "capi-only-key")
	t.Setenv("MIAO_JEV_API_KEY", "")
	_, err := f.api.evaluateJev(context.Background(), f.tenantID, f.userID, "", nil, map[string]jevQuestion{
		"next": {Type: "choice", Criteria: map[string]string{"allowed": "Allowed"}},
	})
	if err == nil || !strings.Contains(err.Error(), "MIAO_JEV_API_KEY") {
		t.Fatalf("capi credential was accepted for Jev: %v", err)
	}
}
