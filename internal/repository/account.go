package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type AccountRepository struct {
	db *sql.DB
}

func NewAccountRepository(db *sql.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// CreateTransaction creates a new transaction record
func (r *AccountRepository) CreateTransaction(transaction *model.Transaction) error {
	query := `
		INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		transaction.UserID,
		transaction.Type,
		transaction.Amount,
		transaction.BalanceBefore,
		transaction.BalanceAfter,
		transaction.Remark,
		transaction.RelatedID,
		now,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	transaction.ID = id
	transaction.CreatedAt = now
	return nil
}

// ListTransactions retrieves transactions for an account with pagination
func (r *AccountRepository) ListTransactions(accountID int64, limit, offset int) ([]*model.Transaction, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM transactions WHERE user_id = ?`
	var total int
	if err := r.db.QueryRow(countQuery, accountID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get transactions
	query := `
		SELECT id, user_id, type, amount, balance_before, balance_after, remark, related_id, created_at
		FROM transactions
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, accountID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transactions []*model.Transaction
	for rows.Next() {
		t := &model.Transaction{}
		if err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.Type,
			&t.Amount,
			&t.BalanceBefore,
			&t.BalanceAfter,
			&t.Remark,
			&t.RelatedID,
			&t.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return transactions, total, nil
}

// ListTransactionsByUserID retrieves transactions for a user with pagination
func (r *AccountRepository) ListTransactionsByUserID(userID int64, limit, offset int) ([]*model.Transaction, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM transactions WHERE user_id = ?`
	var total int
	if err := r.db.QueryRow(countQuery, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get transactions
	query := `
		SELECT id, user_id, type, amount, balance_before, balance_after, remark, related_id, created_at
		FROM transactions
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transactions []*model.Transaction
	for rows.Next() {
		t := &model.Transaction{}
		if err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.Type,
			&t.Amount,
			&t.BalanceBefore,
			&t.BalanceAfter,
			&t.Remark,
			&t.RelatedID,
			&t.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, t)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return transactions, total, nil
}
