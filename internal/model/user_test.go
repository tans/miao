package model

import "testing"

func TestUserGetCommissionFallsBackForInvalidLevel(t *testing.T) {
	tests := []struct {
		name  string
		level UserLevel
	}{
		{name: "negative", level: UserLevel(-1)},
		{name: "too high", level: UserLevel(99)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := &User{Level: tt.level}

			if got := user.GetCommission(); got != 0.10 {
				t.Fatalf("GetCommission() = %v, want 0.10", got)
			}
		})
	}
}
