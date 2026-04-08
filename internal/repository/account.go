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

// CreateAccount creates a new account
func (r *AccountRepository) CreateAccount(account *model.Account) error {
	query := `
		INSERT INTO accounts (user_id, balance, frozen_amount, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		account.UserID,
		account.Balance,
		account.FrozenAmount,
		now,
		now,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	account.ID = id
	account.CreatedAt = now
	account.UpdatedAt = now
	return nil
}

// GetAccountByUserID retrieves an account by user ID
func (r *AccountRepository) GetAccountByUserID(userID int64) (*model.Account, error) {
	query := `
		SELECT id, user_id, balance, frozen_amount, created_at, updated_at
		FROM accounts
		WHERE user_id = ?
	`
	account := &model.Account{}
	err := r.db.QueryRow(query, userID).Scan(
		&account.ID,
		&account.UserID,
		&account.Balance,
		&account.FrozenAmount,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return account, nil
}

// GetAccountByID retrieves an account by ID
func (r *AccountRepository) GetAccountByID(id int64) (*model.Account, error) {
	query := `
		SELECT id, user_id, balance, frozen_amount, created_at, updated_at
		FROM accounts
		WHERE id = ?
	`
	account := &model.Account{}
	err := r.db.QueryRow(query, id).Scan(
		&account.ID,
		&account.UserID,
		&account.Balance,
		&account.FrozenAmount,
		&account.CreatedAt,
		&account.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return account, nil
}

// UpdateAccount updates an account
func (r *AccountRepository) UpdateAccount(account *model.Account) error {
	query := `
		UPDATE accounts
		SET balance = ?, frozen_amount = ?, updated_at = ?
		WHERE id = ?
	`
	account.UpdatedAt = time.Now()
	_, err := r.db.Exec(query,
		account.Balance,
		account.FrozenAmount,
		account.UpdatedAt,
		account.ID,
	)
	return err
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
