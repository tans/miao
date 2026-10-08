package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/tans/miao/internal/harness"
)

const (
	ssePollInterval = 250 * time.Millisecond
	sseHeartbeat    = 15 * time.Second
)

func isTerminalHarnessState(state harness.State) bool {
	switch state {
	case harness.StateCompleted, harness.StateFailed, harness.StateCancelled, harness.StateBudgetExhausted, harness.StateUnknown, harness.StateUnsupported:
		return true
	default:
		return false
	}
}

func (s *Server) streamHarnessRun(w http.ResponseWriter, r *http.Request) {
	run, ok := s.ownedHarness(r.Context(), r)
	if !ok {
		writeError(w, http.StatusNotFound, "运行不存在")
		return
	}
	flusher, ok := prepareSSE(w)
	if !ok {
		return
	}
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	if after < 0 {
		after = 0
	}
	if _, err := w.Write([]byte("retry: 1000\n\n")); err != nil {
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(ssePollInterval)
	defer ticker.Stop()
	heartbeat := time.NewTicker(sseHeartbeat)
	defer heartbeat.Stop()
	for {
		if err := s.sseHarnessRunBatch(w, flusher, r.Context(), r, run.ID, &after); err != nil {
			return
		}
		current, owned := s.ownedHarness(r.Context(), r)
		if !owned {
			return
		}
		if isTerminalHarnessState(current.State) || current.State == harness.StateWaiting {
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if err := writeSSEComment(w, flusher, "keep-alive"); err != nil {
				return
			}
		case <-ticker.C:
		}
	}
}

func (s *Server) sseHarnessRunBatch(w http.ResponseWriter, flusher http.Flusher, ctx context.Context, r *http.Request, runID string, after *int64) error {
	store := pocketHarnessStore{s: s}
	events, err := store.Events(ctx, runID, *after, 200)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.Sequence <= *after {
			continue
		}
		if err := writeSSE(w, flusher, event.Type, strconv.FormatInt(event.Sequence, 10), event.Data); err != nil {
			return err
		}
		*after = event.Sequence
	}
	current, ok := s.ownedHarness(ctx, r)
	if !ok {
		return context.Canceled
	}
	if len(events) > 0 || current.Sequence == 0 || isTerminalHarnessState(current.State) || current.State == harness.StateWaiting {
		if err := writeSSE(w, flusher, "state", "", map[string]any{"run": current}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) streamPublishedRuntime(w http.ResponseWriter, r *http.Request) {
	app, _, err := s.appForRequest(r.Context(), r)
	if err != nil {
		writeError(w, http.StatusNotFound, "应用不存在或你没有访问权限")
		return
	}
	flusher, ok := prepareSSE(w)
	if !ok {
		return
	}
	if _, err := w.Write([]byte("retry: 1500\n\n")); err != nil {
		return
	}
	flusher.Flush()
	lastVersion := r.URL.Query().Get("after")
	sentSnapshot := r.URL.Query().Has("after")
	query := map[string]string{}
	for _, key := range []string{"ui_page", "page", "perPage", "search", "record_id"} {
		query[key] = r.URL.Query().Get(key)
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	heartbeat := time.NewTicker(sseHeartbeat)
	defer heartbeat.Stop()
	for {
		current, _, loadErr := s.appForRequest(r.Context(), r)
		if loadErr != nil || current["id"] != app["id"] {
			return
		}
		versionID := stringValue(current["published_version_id"])
		if !sentSnapshot || versionID != lastVersion {
			runtime := map[string]any{"status": "not_published", "stream_revision": nilIfEmpty(versionID)}
			if versionID != "" {
				version, getErr := s.PB.Get(r.Context(), "app_versions", versionID)
				if getErr != nil || version["tenant_id"] != who(r).Tenant["id"] || version["app_id"] != current["id"] {
					return
				}
				runtime, getErr = s.runtimeForVersion(r.Context(), current, stringValue(who(r).Tenant["id"]), version, query, 0)
				if getErr != nil {
					return
				}
				runtime["stream_revision"] = versionID
			}
			if err := writeSSE(w, flusher, "runtime", versionIDOrNone(versionID), runtime); err != nil {
				return
			}
			lastVersion, sentSnapshot = versionID, true
		}
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if err := writeSSEComment(w, flusher, "keep-alive"); err != nil {
				return
			}
		case <-ticker.C:
		}
	}
}

func versionIDOrNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}
