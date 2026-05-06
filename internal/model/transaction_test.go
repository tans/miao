package model

import "testing"

func TestTransactionDisplayAmountUsesBalanceDelta(t *testing.T) {
	t.Run("freeze with positive stored amount displays as expense", func(t *testing.T) {
		tx := &Transaction{
			Type:          TransactionTypeFreeze,
			Amount:        100,
			BalanceBefore: 1000,
			BalanceAfter:  900,
		}

		if got := tx.DisplayAmount(); got != -100 {
			t.Fatalf("DisplayAmount() = %v, want -100", got)
		}
	})

	t.Run("income with positive balance delta displays as income", func(t *testing.T) {
		tx := &Transaction{
			Type:          TransactionTypePayment,
			Amount:        42.5,
			BalanceBefore: 0,
			BalanceAfter:  42.5,
		}

		if got := tx.DisplayAmount(); got != 42.5 {
			t.Fatalf("DisplayAmount() = %v, want 42.5", got)
		}
	})
}

func TestTransactionDisplayAmountFallsBackToType(t *testing.T) {
	tx := &Transaction{
		Type:          TransactionTypeConsume,
		Amount:        80,
		BalanceBefore: 1000,
		BalanceAfter:  1000,
	}

	if got := tx.DisplayAmount(); got != -80 {
		t.Fatalf("DisplayAmount() = %v, want -80", got)
	}
}

func TestTransactionTypeNameCoversPaymentTypes(t *testing.T) {
	tests := []struct {
		name string
		typ  TransactionType
		want string
	}{
		{name: "consume", typ: TransactionTypeConsume, want: "奖励支出"},
		{name: "payment", typ: TransactionTypePayment, want: "参与奖励"},
		{name: "award payment", typ: TransactionTypeAwardPayment, want: "采纳奖励"},
		{name: "platform income", typ: TransactionTypePlatformIncome, want: "平台收入"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.typ.Name(); got != tt.want {
				t.Fatalf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}
