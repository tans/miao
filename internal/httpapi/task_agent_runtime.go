package httpapi

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/tans/miao/internal/harness"
)

// taskAgentRuntime keeps background work on the shared durable loop. The LLM
// proposes one bounded tool call at a time; all effects still pass through the
// existing authorization and receipt-producing handlers.
type taskAgentRuntime struct {
	s       *Server
	leaseID string
}

func (r taskAgentRuntime) active(ctx context.Context, run *harness.Run) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.s.assertWorkerLease(ctx, r.leaseID); err != nil {
		return nil, err
	}
	row, err := r.s.PB.Get(ctx, "miao_runs", run.ID)
	if err != nil {
		return nil, err
	}
	if boolValue(row["cancel_requested"]) {
		return nil, harness.ErrCancelled
	}
	if row["tenant_id"] != run.TenantID || row["app_id"] != run.AppID || row["created_by"] != run.UserID || !equalJSON(row["snapshot"], run.Context) {
		return nil, businessError(409, "任务授权快照已变化")
	}
	if _, err := taskAuthority(ctx, r.s, row); err != nil {
		return nil, err
	}
	return row, nil
}

func (r taskAgentRuntime) Observe(ctx context.Context, run *harness.Run) (harness.Observation, error) {
	row, err := r.active(ctx, run)
	if err != nil {
		return harness.Observation{}, err
	}
	return harness.Observation{Values: map[string]any{
		"goal":   asMap(run.Context)["goal"],
		"scope":  asMap(run.Context)["scope"],
		"input":  asMap(run.Context)["input"],
		"status": row["status"],
	}}, nil
}

func (r taskAgentRuntime) Enumerate(_ context.Context, run *harness.Run, _ harness.Observation) ([]harness.CandidateOption, error) {
	return []harness.CandidateOption{{ID: "agent:" + run.ID, Capability: "task.agent.step", Description: "根据任务目标执行一个受控后台 Agent 步骤"}}, nil
}

func (r taskAgentRuntime) Decide(ctx context.Context, run *harness.Run, observation harness.Observation, options []harness.CandidateOption) (harness.Decision, error) {
	if len(options) != 1 {
		return harness.Decision{}, harness.ErrCapability
	}
	return harness.Decision{CandidateID: options[0].ID}, nil
}

func (r taskAgentRuntime) Validate(ctx context.Context, run *harness.Run, _ harness.Observation, candidate *harness.Candidate) error {
	if candidate.Capability != "task.agent.step" || candidate.Write {
		return harness.ErrCapability
	}
	_, err := r.active(ctx, run)
	return err
}

func (r taskAgentRuntime) Execute(ctx context.Context, run *harness.Run, _ *harness.Candidate) (harness.StepResult, error) {
	row, err := r.active(ctx, run)
	if err != nil {
		return harness.StepResult{Outcome: harness.OutcomeFailed}, err
	}
	output, receipt, wait, err := r.step(ctx, run, row)
	if err != nil {
		if errors.Is(err, errTaskWaiting) || wait {
			return harness.StepResult{Outcome: harness.OutcomeWaiting, Value: output, Receipt: receipt}, harness.ErrWaiting
		}
		return harness.StepResult{Outcome: harness.OutcomeFailed, Value: output, Receipt: receipt}, err
	}
	return harness.StepResult{Outcome: harness.OutcomeContinue, Value: output, Receipt: receipt}, nil
}

func (r taskAgentRuntime) step(ctx context.Context, run *harness.Run, row map[string]any) (string, map[string]any, bool, error) {
	snapshot := asMap(run.Context)
	checkpoint := asMap(row["checkpoint"])
	messages := anySlice(checkpoint["messages"])
	if len(messages) == 0 {
		instructions := "你是 MIAO 后台业务 Agent。只能完成当前任务目标；先查询再修改，更新必须使用刚查询的 updated_at。只能调用提供的受控工具，不得扩大权限、修改结构、发布、执行 Shell 或猜测事实。工具回执才代表完成。"
		if snapshot["mode"] == "preview" {
			instructions = "本次为只读试运行，不得写入、通知或申请批准。\n" + instructions
		}
		messages = append(messages, map[string]any{"role": "system", "content": instructions})
		prompt, _ := json.Marshal(map[string]any{"goal": snapshot["goal"], "trigger_input": snapshot["input"], "scope": snapshot["scope"], "resume": row["pending"]})
		messages = append(messages, map[string]any{"role": "user", "content": string(prompt)})
	} else if answer := stringValue(asMap(row["pending"])["answer"]); answer != "" {
		messages = append(messages, map[string]any{"role": "user", "content": "负责人补充信息：" + answer})
		row["pending"] = nil
	}
	body := map[string]any{"messages": messages, "tools": taskToolSchemas(snapshot), "tool_choice": "auto"}
	result, _, err := r.s.callAI(ctx, run.TenantID, run.UserID, run.AppID, body)
	if err != nil {
		return "", nil, false, err
	}
	choices := anySlice(result["choices"])
	if len(choices) == 0 {
		return "", nil, false, errors.New("模型没有返回任务结果")
	}
	choice, message := asMap(choices[0]), asMap(asMap(choices[0])["message"])
	if len(message) == 0 {
		return "", nil, false, errors.New("模型返回格式无效")
	}
	messages = append(messages, message)
	output := clip(stringValue(row["output"])+stringValue(message["content"]), 30000)
	calls := anySlice(message["tool_calls"])
	if len(calls) == 0 {
		if stringValue(choice["finish_reason"]) == "length" {
			return output, nil, false, errors.New("模型输出达到长度上限，任务尚未完成")
		}
		receipt := map[string]any{"kind": "agent_response", "messages": messages, "output": output, "tool_receipts": checkpoint["tool_receipts"]}
		if err := r.persistCheckpoint(ctx, run.ID, messages, anySlice(checkpoint["tool_receipts"]), output); err != nil {
			return output, receipt, false, err
		}
		return output, receipt, false, nil
	}
	toolReceipts := anySlice(checkpoint["tool_receipts"])
	for _, raw := range calls {
		call := asMap(raw)
		fn := asMap(call["function"])
		name := stringValue(fn["name"])
		args := map[string]any{}
		if json.Unmarshal([]byte(stringValue(fn["arguments"])), &args) != nil {
			return output, nil, false, errors.New("模型工具参数不是有效 JSON")
		}
		toolOutput, toolErr := r.s.executeTaskTool(ctx, row, args, name, func() error { _, e := r.active(ctx, run); return e })
		content := toolOutput
		if toolErr != nil {
			content = "操作失败：" + toolErr.Error()
		}
		toolReceipts = append(toolReceipts, map[string]any{"tool_call_id": call["id"], "name": name, "arguments": args, "output": content, "succeeded": toolErr == nil})
		messages = append(messages, map[string]any{"role": "tool", "tool_call_id": call["id"], "name": name, "content": content})
		if toolErr == errTaskWaiting {
			_ = r.persistCheckpoint(ctx, run.ID, messages, toolReceipts, output)
			return output, map[string]any{"kind": "waiting", "tool": name}, true, errTaskWaiting
		}
		if toolErr != nil {
			_ = r.persistCheckpoint(ctx, run.ID, messages, toolReceipts, output)
			return output, map[string]any{"kind": "tool_error", "tool": name}, false, toolErr
		}
	}
	if err := r.persistCheckpoint(ctx, run.ID, messages, toolReceipts, output); err != nil {
		return output, nil, false, err
	}
	return output, map[string]any{"kind": "tool_calls", "count": len(calls), "output": output}, false, nil
}

func (r taskAgentRuntime) persistCheckpoint(ctx context.Context, id string, messages, toolReceipts []any, output string) error {
	data, _ := json.Marshal(map[string]any{"messages": messages, "tool_receipts": toolReceipts})
	if len(data) >= 6<<20 {
		return errors.New("任务会话超过保存限制")
	}
	_, err := r.s.PB.Update(ctx, "miao_runs", id, map[string]any{"checkpoint": map[string]any{"messages": messages, "tool_receipts": toolReceipts}, "output": output})
	return err
}

func (r taskAgentRuntime) CheckComplete(_ context.Context, run *harness.Run, _ harness.Observation) (harness.Completion, error) {
	if run.Loop == nil || len(run.Loop.Steps) == 0 {
		return harness.Completion{}, nil
	}
	step := run.Loop.Steps[len(run.Loop.Steps)-1]
	receipt := asMap(step.Result.Receipt)
	if step.CompletedAt.IsZero() || step.Result.Outcome != harness.OutcomeContinue || receipt["kind"] != "agent_response" {
		return harness.Completion{}, nil
	}
	return harness.Completion{Satisfied: true, Evidence: []any{receipt}}, nil
}

func (r taskAgentRuntime) Reconcile(ctx context.Context, run *harness.Run, step harness.Step) (harness.StepResult, error) {
	if step.Candidate.Capability != "task.agent.step" {
		return harness.StepResult{Outcome: harness.OutcomeUnknown}, harness.ErrUnknown
	}
	return r.Execute(ctx, run, &step.Candidate)
}
