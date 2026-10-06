package jev

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChoiceTransportAndDependentObservation(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		for name, expected := range map[string]string{
			"Authorization": "Bearer test-key", "ai-gateway-protocol-version": "0.0.1",
			"ai-evaluation-model-specification-version": "4", "ai-model-id": DefaultModel,
		} {
			if r.Header.Get(name) != expected {
				t.Errorf("header %s=%q", name, r.Header.Get(name))
			}
		}
		var body struct {
			State     map[string]any      `json:"state"`
			Questions map[string]Question `json:"questions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		choice := "create_table"
		if requests == 2 {
			if body.State["table_id"] != "persisted-table" || body.State["receipt"] != "step-1" {
				t.Error("dependent observation lost prior tool result")
			}
			choice = "bind_ui"
		}
		if _, offered := body.Questions["next"].Criteria[choice]; !offered {
			t.Error("dependent candidate was not enumerated")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"answers":          map[string]any{"next": map[string]any{"type": "choice", "choice": choice}},
			"providerMetadata": map[string]any{"typesafe": map[string]any{"confidence": map[string]float64{"next": 0.75}}},
			"usage":            map[string]int{"inputTokens": 17},
		})
	}))
	defer server.Close()
	e := Evaluator{APIKey: "test-key", Model: DefaultModel, URL: server.URL}
	first, err := e.Evaluate(context.Background(), map[string]any{"goal": "build"}, map[string]Question{
		"next": {Type: "choice", Criteria: map[string]string{"create_table": "Create a table"}},
	})
	if err != nil || first.Answers["next"].Choice != "create_table" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	// The caller persists execution and reconstructs the next request. The
	// evaluator receives a fresh state, with no conversational tool protocol.
	second, err := e.Evaluate(context.Background(), map[string]any{"table_id": "persisted-table", "receipt": "step-1"}, map[string]Question{
		"next": {Type: "choice", Criteria: map[string]string{"bind_ui": "Bind the real table"}},
	})
	if err != nil || second.Answers["next"].Choice != "bind_ui" || !second.Answers["next"].HasConf || second.Answers["next"].Confidence != 0.75 || second.InputTokens != 17 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestRejectInvalidUpstreamResponses(t *testing.T) {
	for name, body := range map[string]string{
		"unoffered":        `{"answers":{"next":{"type":"choice","choice":"invented"}}}`,
		"missing":          `{"answers":{}}`,
		"wrong type":       `{"answers":{"next":{"type":"text","choice":"allowed"}}}`,
		"confidence":       `{"answers":{"next":{"type":"choice","choice":"allowed"}},"providerMetadata":{"typesafe":{"confidence":{"next":1.1}}}}`,
		"null confidence":  `{"answers":{"next":{"type":"choice","choice":"allowed"}},"providerMetadata":{"typesafe":{"confidence":{"next":null}}}}`,
		"extra confidence": `{"answers":{"next":{"type":"choice","choice":"allowed"}},"providerMetadata":{"typesafe":{"confidence":{"unrequested":-1}}}}`,
		"fraction tokens":  `{"answers":{"next":{"type":"choice","choice":"allowed"}},"usage":{"inputTokens":0.5}}`,
		"negative tokens":  `{"answers":{"next":{"type":"choice","choice":"allowed"}},"usage":{"inputTokens":-1}}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			e := Evaluator{APIKey: "test-key", Model: DefaultModel, URL: server.URL}
			_, err := e.Evaluate(context.Background(), nil, map[string]Question{
				"next": {Type: "choice", Criteria: map[string]string{"allowed": "Allowed"}},
			})
			if err == nil {
				t.Fatal("accepted invalid response")
			}
		})
	}
}

func TestEvaluationTimeoutAndHTTPFailure(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-release:
			}
		}))
		defer server.Close()
		defer close(release)
		e := Evaluator{APIKey: "test-key", Model: DefaultModel, URL: server.URL, Timeout: 10 * time.Millisecond}
		_, err := e.Evaluate(context.Background(), nil, map[string]Question{"next": {Type: "choice", Criteria: map[string]string{"allowed": "Allowed"}}})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("timeout=%v", err)
		}
	})
	t.Run("HTTP error does not expose response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(401)
			_, _ = w.Write([]byte("credential-like-provider-detail"))
		}))
		defer server.Close()
		e := Evaluator{APIKey: "test-key", Model: DefaultModel, URL: server.URL}
		result, err := e.Evaluate(context.Background(), nil, map[string]Question{"next": {Type: "choice", Criteria: map[string]string{"allowed": "Allowed"}}})
		if result.Status != 401 || err == nil || strings.Contains(err.Error(), "credential-like") {
			t.Fatalf("result=%+v error=%v", result, err)
		}
	})
}
