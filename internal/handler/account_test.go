package handler

import "testing"

func TestCalculateWithdrawActualAmount(t *testing.T) {
	tests := []struct {
		name     string
		amount   float64
		rate     float64
		expected float64
	}{
		{name: "10_percent", amount: 100, rate: 0.10, expected: 90},
		{name: "5_percent", amount: 100, rate: 0.05, expected: 95},
		{name: "3_percent", amount: 100, rate: 0.03, expected: 97},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateWithdrawActualAmount(tt.amount, tt.rate)
			if got != tt.expected {
				t.Fatalf("calculateWithdrawActualAmount(%v, %v) = %v, want %v", tt.amount, tt.rate, got, tt.expected)
			}
		})
	}
}
