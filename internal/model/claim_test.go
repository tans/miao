package model

import (
	"encoding/json"
	"testing"
)

func TestClaimCreateUnmarshalJSONAcceptsStringAndNumber(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int64
	}{
		{name: "number", body: `{"task_id":20}`, want: 20},
		{name: "string", body: `{"task_id":"20"}`, want: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req ClaimCreate
			if err := json.Unmarshal([]byte(tt.body), &req); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			if req.TaskID != tt.want {
				t.Fatalf("TaskID = %d, want %d", req.TaskID, tt.want)
			}
		})
	}
}

func TestClaimCreateUnmarshalJSONRejectsInvalidTaskID(t *testing.T) {
	var req ClaimCreate
	err := json.Unmarshal([]byte(`{"task_id":"abc"}`), &req)
	if err == nil {
		t.Fatal("json.Unmarshal() error = nil, want non-nil")
	}
}
