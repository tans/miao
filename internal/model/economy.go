package model

import "math"

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func ClaimRewardBudget(unitPrice, awardPrice float64) float64 {
	total := unitPrice + awardPrice
	if total < 0 {
		return 0
	}
	return roundMoney(total)
}

func CreatorNetReward(amount, commissionRate float64) float64 {
	if amount <= 0 {
		return 0
	}
	return roundMoney(amount * (1 - commissionRate))
}

func PlatformCommissionAmount(amount, commissionRate float64) float64 {
	if amount <= 0 {
		return 0
	}
	return roundMoney(amount * commissionRate)
}

func RefundableTaskFrozenAmount(frozenAmount float64) float64 {
	if frozenAmount < 0 {
		return 0
	}
	return roundMoney(frozenAmount)
}
