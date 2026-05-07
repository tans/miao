package model

import "testing"

func TestClaimRewardBudgetIncludesParticipationAndAward(t *testing.T) {
	if got := ClaimRewardBudget(10, 20); got != 30 {
		t.Fatalf("ClaimRewardBudget(10, 20) = %v, want 30", got)
	}
}

func TestCreatorNetReward(t *testing.T) {
	if got := CreatorNetReward(20, 0.10); got != 18 {
		t.Fatalf("CreatorNetReward(20, 0.10) = %v, want 18", got)
	}
	if got := CreatorNetReward(0, 0.10); got != 0 {
		t.Fatalf("CreatorNetReward(0, 0.10) = %v, want 0", got)
	}
}

func TestPlatformCommissionAmount(t *testing.T) {
	if got := PlatformCommissionAmount(30, 0.10); got != 3 {
		t.Fatalf("PlatformCommissionAmount(30, 0.10) = %v, want 3", got)
	}
	if got := PlatformCommissionAmount(-1, 0.10); got != 0 {
		t.Fatalf("PlatformCommissionAmount(-1, 0.10) = %v, want 0", got)
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
