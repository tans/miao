package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type CreditRepository struct {
	db *sql.DB
}

func NewCreditRepository(db *sql.DB) *CreditRepository {
	return &CreditRepository{db: db}
}

// CreateCreditLog creates a new credit log
func (r *CreditRepository) CreateCreditLog(log *model.CreditLog) error {
	query := `
		INSERT INTO credit_logs (user_id, type, change, reason, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		log.UserID,
		log.Type,
		log.Change,
		log.Reason,
		log.RelatedID,
		now,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id
	log.CreatedAt = now
	return nil
}

// GetCreditLogsByUserID retrieves credit logs for a user with pagination
func (r *CreditRepository) GetCreditLogsByUserID(userID int64, limit, offset int) ([]*model.CreditLog, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM credit_logs WHERE user_id = ?`
	var total int
	if err := r.db.QueryRow(countQuery, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get logs
	query := `
		SELECT id, user_id, type, change, reason, related_id, created_at
		FROM credit_logs
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*model.CreditLog
	for rows.Next() {
		log := &model.CreditLog{}
		if err := rows.Scan(
			&log.ID,
			&log.UserID,
			&log.Type,
			&log.Change,
			&log.Reason,
			&log.RelatedID,
			&log.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}
