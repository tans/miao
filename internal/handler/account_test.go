package handler

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/model"
)

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

func TestTransactionFeeFromRemark(t *testing.T) {
	got := transactionFeeFromRemark("提现到账90.00元(扣除佣金10.00元)")
	if got != 10 {
		t.Fatalf("transactionFeeFromRemark() = %v, want 10", got)
	}
}

func TestDeriveTransactionFeeUsesWithdrawRelatedRecord(t *testing.T) {
	oldDB := db
	tempDB, err := database.InitDB(config.DatabaseConfig{
		Driver: string(database.DriverSQLite),
		Path:   filepath.Join(t.TempDir(), "wallet_fee.db"),
	})
	require.NoError(t, err)
	defer tempDB.Close()
	db = tempDB
	t.Cleanup(func() {
		db = oldDB
	})

	_, err = db.Exec(`
		CREATE TABLE withdraw_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			withdraw_no TEXT NOT NULL,
			idempotency_key TEXT,
			amount REAL NOT NULL,
			actual_amount REAL NOT NULL,
			commission_amount REAL NOT NULL,
			status INTEGER NOT NULL,
			created_at DATETIME,
			updated_at DATETIME
		);
	`)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO withdraw_orders (id, user_id, withdraw_no, amount, actual_amount, commission_amount, status, created_at, updated_at)
		VALUES (1, 7, 'W1', 100, 90, 10, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	require.NoError(t, err)

	tx := &model.Transaction{Type: model.TransactionTypeWithdraw, RelatedID: 1}
	if got := deriveTransactionFee(tx); got != 10 {
		t.Fatalf("deriveTransactionFee() = %v, want 10", got)
	}

	formatted := formatTransaction(tx)
	if got, ok := formatted["fee"].(float64); !ok || got != 10 {
		t.Fatalf("formatTransaction() fee = %#v, want 10", formatted["fee"])
	}
}
