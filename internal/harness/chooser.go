package harness

import "errors"

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
