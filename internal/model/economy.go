package model

func ClaimRewardBudget(unitPrice, awardPrice float64) float64 {
	total := unitPrice + awardPrice
	if total < 0 {
		return 0
	}
	return total
}

func RefundableTaskFrozenAmount(frozenAmount float64) float64 {
	if frozenAmount < 0 {
		return 0
	}
	return frozenAmount
}
