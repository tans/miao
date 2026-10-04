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
}

type Chooser interface {
	Choose(context.Context, *Run, []CandidateOption) (CandidateOption, error)
}

// StaticChooser is only for a server-produced selection. It never chooses the
// first option and rejects IDs that were not in the enumerated set.
type StaticChooser struct { SelectedID string }
func (c StaticChooser) Choose(_ context.Context, _ *Run, options []CandidateOption) (CandidateOption, error) {
	if c.SelectedID == "" { return CandidateOption{}, ErrChooserUnavailable }
	for _, option := range options { if option.ID == c.SelectedID { return option, nil } }
	return CandidateOption{}, ErrChooserUnavailable
}
