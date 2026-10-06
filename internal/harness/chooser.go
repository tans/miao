package harness

import (
	"context"
	"errors"
)

var ErrChooserUnavailable = errors.New("candidate chooser is unavailable")

type CandidateOption struct {
	ID          string         `json:"id"`
	Capability  string         `json:"capability"`
	Description string         `json:"description"`
	Input       map[string]any `json:"input,omitempty"`
	Write       bool           `json:"write"`
	Direct      bool           `json:"direct,omitempty"`
	Evidence    map[string]any `json:"evidence,omitempty"`
}

// Chooser evaluates the request against freshly enumerated server candidates.
type Chooser interface {
	Choose(context.Context, *Run, []CandidateOption) (CandidateOption, error)
}

// StaticChooser accepts only an explicit server-produced selection.
type StaticChooser struct{ SelectedID string }

func (c StaticChooser) Choose(_ context.Context, _ *Run, options []CandidateOption) (CandidateOption, error) {
	if c.SelectedID == "" {
		return CandidateOption{}, ErrChooserUnavailable
	}
	for _, option := range options {
		if option.ID == c.SelectedID {
			return option, nil
		}
	}
	return CandidateOption{}, ErrChooserUnavailable
}
