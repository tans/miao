package httpapi

import (
	"context"
	"errors"

	"github.com/tans/miao/internal/harness"
)

var errTaskAgentDisabled = errors.New("后台 LLM Agent 能力暂时下架，请改用固定数据报告任务")

// taskAgentRuntime remains as a durable compatibility boundary for runs that
// were queued before the capability was removed. It fails before any model
// request or tool execution so old runs cannot enter a half-executed state.
type taskAgentRuntime struct {
	s       *Server
	leaseID string
}

func (r taskAgentRuntime) Observe(context.Context, *harness.Run) (harness.Observation, error) {
	return harness.Observation{}, errTaskAgentDisabled
}

func (r taskAgentRuntime) Enumerate(context.Context, *harness.Run, harness.Observation) ([]harness.CandidateOption, error) {
	return nil, errTaskAgentDisabled
}

func (r taskAgentRuntime) Decide(context.Context, *harness.Run, harness.Observation, []harness.CandidateOption) (harness.Decision, error) {
	return harness.Decision{}, errTaskAgentDisabled
}

func (r taskAgentRuntime) Validate(context.Context, *harness.Run, harness.Observation, *harness.Candidate) error {
	return errTaskAgentDisabled
}

func (r taskAgentRuntime) Execute(context.Context, *harness.Run, *harness.Candidate) (harness.StepResult, error) {
	return harness.StepResult{Outcome: harness.OutcomeFailed, Value: map[string]any{"message": errTaskAgentDisabled.Error()}}, errTaskAgentDisabled
}

func (r taskAgentRuntime) CheckComplete(context.Context, *harness.Run, harness.Observation) (harness.Completion, error) {
	return harness.Completion{}, errTaskAgentDisabled
}

func (r taskAgentRuntime) Reconcile(context.Context, *harness.Run, harness.Step) (harness.StepResult, error) {
	return harness.StepResult{Outcome: harness.OutcomeUnknown, Value: map[string]any{"message": errTaskAgentDisabled.Error()}}, errTaskAgentDisabled
}
