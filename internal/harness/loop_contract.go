package harness

import (
	"context"
	"time"
)

type Outcome string

const (
	OutcomeContinue Outcome = "continue"
	OutcomeWaiting  Outcome = "waiting"
	OutcomeUnknown  Outcome = "unknown"
	OutcomeFailed   Outcome = "failed"
)

type Observation struct {
	Revision string         `json:"revision"`
	Values   map[string]any `json:"values"`
}

// Completion is produced by code checking the goal, never by the chooser.
type Completion struct {
	Satisfied bool     `json:"satisfied"`
	Evidence  []any    `json:"evidence,omitempty"`
	Missing   []string `json:"missing,omitempty"`
}

type Decision struct {
	CandidateID string `json:"candidate_id"`
}

type StepResult struct {
	Outcome Outcome `json:"outcome"`
	Value   any     `json:"value,omitempty"`
	Receipt any     `json:"receipt,omitempty"`
}

type Step struct {
	ID          string     `json:"id"`
	Candidate   Candidate  `json:"candidate"`
	Authority   *Authority `json:"authority,omitempty"`
	Observation string     `json:"observation"`
	Result      StepResult `json:"result"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt time.Time  `json:"completed_at,omitempty"`
}

type Limits struct {
	MaxSteps            int           `json:"max_steps"`
	MaxDecisions        int           `json:"max_decisions"`
	MaxModelRequests    int           `json:"max_model_requests"`
	MaxNoProgress       int           `json:"max_no_progress"`
	MaxObservationBytes int           `json:"max_observation_bytes"`
	Timeout             time.Duration `json:"timeout"`
}

type LoopState struct {
	Steps          []Step        `json:"steps"`
	Decisions      int           `json:"decisions"`
	ModelRequests  int           `json:"model_requests"`
	NoProgress     int           `json:"no_progress"`
	Observation    Observation   `json:"observation"`
	Completion     Completion    `json:"completion"`
	ObservedSteps  int           `json:"observed_steps"`
	ActiveDuration time.Duration `json:"active_duration"`
}

// Runtime provides application-specific observations and capabilities. The
// engine owns the loop and only executes an option returned by Enumerate.
type Runtime interface {
	Observe(context.Context, *Run) (Observation, error)
	Enumerate(context.Context, *Run, Observation) ([]CandidateOption, error)
	Decide(context.Context, *Run, Observation, []CandidateOption) (Decision, error)
	Validate(context.Context, *Run, Observation, *Candidate) error
	Execute(context.Context, *Run, *Candidate) (StepResult, error)
	CheckComplete(context.Context, *Run, Observation) (Completion, error)
}
