package repository

import (
	"database/sql"
	"github.com/tans/miao/internal/database"
	"time"

	"github.com/tans/miao/internal/model"
)

type AppealRepository struct {
	db database.DB
}

func NewAppealRepository(db database.DB) *AppealRepository {
	return &AppealRepository{db: db}
}

// CreateAppeal creates a new appeal
func (r *AppealRepository) CreateAppeal(appeal *model.Appeal) error {
	query := `
		INSERT INTO appeals (user_id, type, target_id, reason, evidence, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	id, err := database.InsertReturningID(r.db, query,
		appeal.UserID,
		appeal.Type,
		appeal.TargetID,
		appeal.Reason,
		appeal.Evidence,
		appeal.Status,
		now,
	)
	if err != nil {
		return err
	}
	appeal.ID = id
	appeal.CreatedAt = now
	return nil
}

// GetAppealByID retrieves an appeal by ID
func (r *AppealRepository) GetAppealByID(id int64) (*model.Appeal, error) {
	query := `
		SELECT id, user_id, type, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
		FROM appeals
		WHERE id = ?
	`
	appeal := &model.Appeal{}
	var result, evidence sql.NullString
	var adminID sql.NullInt64
	var handleAt sql.NullTime
	err := r.db.QueryRow(query, id).Scan(
		&appeal.ID,
		&appeal.UserID,
		&appeal.Type,
		&appeal.TargetID,
		&appeal.Reason,
		&evidence,
		&appeal.Status,
		&result,
		&adminID,
		&handleAt,
		&appeal.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	appeal.Result = result.String
	appeal.Evidence = evidence.String
	if adminID.Valid {
		appeal.AdminID = adminID.Int64
	}
	if handleAt.Valid {
		appeal.HandleAt = &handleAt.Time
	}
	return appeal, nil
}

// ListAppeals retrieves appeals with optional filters and pagination
func (r *AppealRepository) ListAppeals(status, appealType int, limit, offset int) ([]*model.Appeal, int, error) {
	// Build count query
	countQuery := `SELECT COUNT(*) FROM appeals WHERE 1=1`
	args := []interface{}{}
	if status > 0 {
		countQuery += ` AND status = ?`
		args = append(args, status)
	}
	if appealType > 0 {
		countQuery += ` AND type = ?`
		args = append(args, appealType)
	}

	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Build select query
	query := `
		SELECT id, user_id, type, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
		FROM appeals
		WHERE 1=1`
	if status > 0 {
		query += ` AND status = ?`
	}
	if appealType > 0 {
		query += ` AND type = ?`
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`

	args = append(args, limit, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var appeals []*model.Appeal
	for rows.Next() {
		appeal := &model.Appeal{}
		var result, evidence sql.NullString
		var adminID sql.NullInt64
		var handleAt sql.NullTime
		if err := rows.Scan(
			&appeal.ID,
			&appeal.UserID,
			&appeal.Type,
			&appeal.TargetID,
			&appeal.Reason,
			&evidence,
			&appeal.Status,
			&result,
			&adminID,
			&handleAt,
			&appeal.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		appeal.Result = result.String
		appeal.Evidence = evidence.String
		if adminID.Valid {
			appeal.AdminID = adminID.Int64
		}
		if handleAt.Valid {
			appeal.HandleAt = &handleAt.Time
		}
		appeals = append(appeals, appeal)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return appeals, total, nil
}

// ListAppealsByUserID retrieves appeals for a specific user with pagination
func (r *AppealRepository) ListAppealsByUserID(userID int64, limit, offset int) ([]*model.Appeal, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM appeals WHERE user_id = ?`
	var total int
	if err := r.db.QueryRow(countQuery, userID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get appeals
	query := `
		SELECT id, user_id, type, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
		FROM appeals
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var appeals []*model.Appeal
	for rows.Next() {
		appeal := &model.Appeal{}
		var result, evidence sql.NullString
		var adminID sql.NullInt64
		var handleAt sql.NullTime
		if err := rows.Scan(
			&appeal.ID,
			&appeal.UserID,
			&appeal.Type,
			&appeal.TargetID,
			&appeal.Reason,
			&evidence,
			&appeal.Status,
			&result,
			&adminID,
			&handleAt,
			&appeal.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		appeal.Result = result.String
		appeal.Evidence = evidence.String
		if adminID.Valid {
			appeal.AdminID = adminID.Int64
		}
		if handleAt.Valid {
			appeal.HandleAt = &handleAt.Time
		}
		appeals = append(appeals, appeal)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return appeals, total, nil
}

// UpdateAppealStatus updates an appeal's status and result
func (r *AppealRepository) UpdateAppealStatus(id int64, status int, result string) error {
	query := `UPDATE appeals SET status = ?, result = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, result, id)
	return err
}

// UpdateAppealWithAdmin updates an appeal's status, result, admin_id and handle_at
func (r *AppealRepository) UpdateAppealWithAdmin(id int64, status int, result string, adminID int64) error {
	query := `UPDATE appeals SET status = ?, result = ?, admin_id = ?, handle_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, result, adminID, time.Now(), id)
	return err
}

// GetAppealsByTargetID retrieves appeals for a specific target (task or submission)
func (r *AppealRepository) GetAppealsByTargetID(targetID int64, appealType int) ([]*model.Appeal, error) {
	query := `
		SELECT id, user_id, type, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
		FROM appeals
		WHERE target_id = ? AND type = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query, targetID, appealType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var appeals []*model.Appeal
	for rows.Next() {
		appeal := &model.Appeal{}
		var result, evidence sql.NullString
		var adminID sql.NullInt64
		var handleAt sql.NullTime
		if err := rows.Scan(
			&appeal.ID,
			&appeal.UserID,
			&appeal.Type,
			&appeal.TargetID,
			&appeal.Reason,
			&evidence,
			&appeal.Status,
			&result,
			&adminID,
			&handleAt,
			&appeal.CreatedAt,
		); err != nil {
			return nil, err
		}
		appeal.Result = result.String
		appeal.Evidence = evidence.String
		if adminID.Valid {
			appeal.AdminID = adminID.Int64
		}
		if handleAt.Valid {
			appeal.HandleAt = &handleAt.Time
		}
		appeals = append(appeals, appeal)
	}

	return appeals, rows.Err()
}
