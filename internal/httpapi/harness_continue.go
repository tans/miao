package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/tans/miao/internal/harness"
)

func (s *Server) continueHarnessRun(w http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 30*time.Second)
	defer cancel()
	run, ok := s.ownedHarness(ctx, request)
	if !ok {
		writeError(w, 404, "运行不存在")
		return
	}
	input := mapBody(request)
	answer := strings.TrimSpace(stringValue(input["answer"]))
	if answer == "" || len([]rune(answer)) > 12000 {
		writeError(w, 400, "补充内容需要 1–12000 个字符")
		return
	}
	if run.State != harness.StateWaiting || run.Phase != "execution_waiting" || run.Candidate == nil || run.Candidate.Capability != "requirements.collect" {
		writeError(w, 409, "当前运行不等待需求补充")
		return
	}
	owner, err := randomToken()
	if err != nil {
		writeError(w, 503, "运行占用创建失败")
		return
	}
	store := pocketHarnessStore{s: s}
	owned, err := store.Acquire(ctx, run.ID, owner, time.Now(), time.Now().Add(time.Minute))
	if err != nil {
		writeError(w, 409, "运行正在推进，请稍后重试")
		return
	}
	updateErr := func() error {
		defer store.Release(context.WithoutCancel(ctx), run.ID, owner)
		if owned.CancelRequested || owned.State != harness.StateWaiting || owned.Phase != "execution_waiting" || owned.Version != int64(intValue(input["expected_version"])) {
			return harness.ErrStaleVersion
		}
		if _, err := s.workspaceActor(ctx, s.PB, runActor(owned)); err != nil {
			return err
		}
		value := cloneAnyMap(asMap(owned.Context))
		answers := anySlice(value["answers"])
		if len(answers) >= 20 {
			return businessError(400, "需求补充次数已达到本轮上限")
		}
		value["answers"] = append(answers, answer)
		owned.Context = value
		owned.State, owned.Phase = harness.StateQueued, "input_received"
		owned.Version++
		owned.Sequence++
		return store.Commit(ctx, owned, &harness.Event{RunID: run.ID, Sequence: owned.Sequence, Type: "input_received", Data: map[string]any{"message": "已收到补充需求"}})
	}()
	if updateErr != nil {
		if updateErr == harness.ErrStaleVersion || updateErr == harness.ErrConflict {
			writeError(w, 409, "运行版本已变化，请重新读取")
			return
		}
		s.writeBusinessError(w, updateErr)
		return
	}
	_ = s.harnessEngine().Resume(ctx, run.ID)
	saved, err := store.Load(request.Context(), run.ID)
	if err != nil {
		writeError(w, 503, "运行读取失败")
		return
	}
	writeJSON(w, 202, map[string]any{"run": saved})
}

func (s *Server) resumeHarnessRun(w http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 30*time.Second)
	defer cancel()
	run, ok := s.ownedHarness(ctx, request)
	if !ok {
		writeError(w, 404, "运行不存在")
		return
	}
	if run.State != harness.StateUnknown && run.State != harness.StateQueued {
		writeError(w, 409, "当前状态不需要核实或恢复")
		return
	}
	engine := s.harnessEngine()
	err := engine.Resume(ctx, run.ID)
	if errors.Is(err, harness.ErrBusy) || errors.Is(err, harness.ErrConflict) {
		writeError(w, 409, "运行正在推进，请稍后重试")
		return
	}
	saved, loadErr := engine.Store.Load(request.Context(), run.ID)
	if loadErr != nil {
		writeError(w, 503, "运行读取失败")
		return
	}
	writeJSON(w, 200, map[string]any{"run": saved})
}
