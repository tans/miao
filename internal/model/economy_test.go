package model

import "testing"

func TestClaimRewardBudgetIncludesParticipationAndAward(t *testing.T) {
	if got := ClaimRewardBudget(10, 20); got != 30 {
		t.Fatalf("ClaimRewardBudget(10, 20) = %v, want 30", got)
	}
}

func TestRefundableTaskFrozenAmountUsesFrozenOnly(t *testing.T) {
	if got := RefundableTaskFrozenAmount(60); got != 60 {
		t.Fatalf("RefundableTaskFrozenAmount(60) = %v, want 60", got)
	}
	if got := RefundableTaskFrozenAmount(-1); got != 0 {
		t.Fatalf("RefundableTaskFrozenAmount(-1) = %v, want 0", got)
	}
}
