package repository

import (
	"database/sql"
	"github.com/tans/miao/internal/database"
	"strings"
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
	now := time.Now()
	targetID := appeal.TargetID
	if targetID <= 0 {
		targetID = appeal.ClaimID
	}
	query := `
		INSERT INTO appeals (user_id, type, claim_id, target_id, reason, evidence, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	id, err := database.InsertReturningID(r.db, query,
		appeal.UserID,
		appeal.Type,
		appeal.ClaimID,
		targetID,
		appeal.Reason,
		appeal.Evidence,
		appeal.Status,
		now,
	)
	if err != nil {
		if appeal.ClaimID > 0 {
			legacyQuery := `
				INSERT INTO appeals (user_id, type, target_id, reason, evidence, status, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)
			`
			id, err = database.InsertReturningID(r.db, legacyQuery,
				appeal.UserID,
				appeal.Type,
				targetID,
				appeal.Reason,
				appeal.Evidence,
				appeal.Status,
				now,
			)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	appeal.ID = id
	appeal.CreatedAt = now
	return nil
}

// GetAppealByID retrieves an appeal by ID
func (r *AppealRepository) GetAppealByID(id int64) (*model.Appeal, error) {
	query := `
		SELECT id, user_id, type, claim_id, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
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
		&appeal.ClaimID,
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
		if !hasColumnErr(err, "claim_id") {
			if err == sql.ErrNoRows {
				return nil, ErrNotFound
			}
			return nil, err
		}
		legacyQuery := `
			SELECT id, user_id, type, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
			FROM appeals
			WHERE id = ?
		`
		var legacyTargetID int64
		err = r.db.QueryRow(legacyQuery, id).Scan(
			&appeal.ID,
			&appeal.UserID,
			&appeal.Type,
			&legacyTargetID,
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
		appeal.ClaimID = legacyTargetID
		appeal.TargetID = legacyTargetID
	} else {
		if appeal.ClaimID == 0 {
			appeal.ClaimID = appeal.TargetID
		}
		if appeal.TargetID == 0 {
			appeal.TargetID = appeal.ClaimID
		}
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

func hasColumnErr(err error, column string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), strings.ToLower(column))
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
		SELECT id, user_id, type, claim_id, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
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
			&appeal.ClaimID,
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
		SELECT id, user_id, type, claim_id, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
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
			&appeal.ClaimID,
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
		if appeal.ClaimID == 0 {
			appeal.ClaimID = appeal.TargetID
		}
		if appeal.TargetID == 0 {
			appeal.TargetID = appeal.ClaimID
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
	query := `UPDATE appeals SET status = ?, result = ?, handle_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, result, time.Now(), id)
	return err
}

// UpdateAppealWithAdmin updates an appeal's status, result, admin_id and handle_at
func (r *AppealRepository) UpdateAppealWithAdmin(id int64, status int, result string, adminID int64) error {
	query := `UPDATE appeals SET status = ?, result = ?, admin_id = ?, handle_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, result, adminID, time.Now(), id)
	return err
}

// GetAppealsByClaimID retrieves appeals for a specific claim.
func (r *AppealRepository) GetAppealsByClaimID(claimID int64, appealType int) ([]*model.Appeal, error) {
	query := `
		SELECT id, user_id, type, claim_id, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
		FROM appeals
		WHERE claim_id = ? AND type = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(query, claimID, appealType)
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
			&appeal.ClaimID,
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
		if appeal.ClaimID == 0 {
			appeal.ClaimID = appeal.TargetID
		}
		if appeal.TargetID == 0 {
			appeal.TargetID = appeal.ClaimID
		}
		appeals = append(appeals, appeal)
	}

	return appeals, rows.Err()
}

// GetAppealsByTargetID keeps legacy tests and callers working while the
// business logic migrates to claim_id.
func (r *AppealRepository) GetAppealsByTargetID(targetID int64, appealType int) ([]*model.Appeal, error) {
	return r.GetAppealsByClaimID(targetID, appealType)
}
