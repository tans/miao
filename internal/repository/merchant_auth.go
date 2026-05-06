package repository

import (
	"database/sql"
	"strings"
	"time"

	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/model"
)

type MerchantAuthRepository struct {
	db database.DB
}

func NewMerchantAuthRepository(db database.DB) *MerchantAuthRepository {
	return &MerchantAuthRepository{db: db}
}

func (r *MerchantAuthRepository) GetByUserID(userID int64) (*model.MerchantAuthApplication, error) {
	query := `
		SELECT id, user_id, company_name, credit_code, contact_name, contact_phone,
			license_url, status, review_comment, reviewed_at, created_at, updated_at
		FROM merchant_auth_applications
		WHERE user_id = ?
		LIMIT 1
	`
	if r.db.Dialect() == database.DriverSQLite {
		query = `
			SELECT COALESCE(id, rowid) AS id, user_id, company_name, credit_code, contact_name, contact_phone,
				license_url, status, review_comment, reviewed_at, created_at, updated_at
			FROM merchant_auth_applications
			WHERE user_id = ?
			LIMIT 1
		`
	}
	app := &model.MerchantAuthApplication{}
	var reviewedAt sql.NullTime
	var reviewComment sql.NullString

	err := r.db.QueryRow(query, userID).Scan(
		&app.ID,
		&app.UserID,
		&app.CompanyName,
		&app.CreditCode,
		&app.ContactName,
		&app.ContactPhone,
		&app.LicenseURL,
		&app.Status,
		&reviewComment,
		&reviewedAt,
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	app.ReviewComment = reviewComment.String
	if reviewedAt.Valid {
		app.ReviewedAt = reviewedAt.Time
	}

	return app, nil
}

func (r *MerchantAuthRepository) ListApplications(keyword string, status *int, limit, offset int) ([]*model.MerchantAuthListItem, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	whereClause := "WHERE 1=1"
	args := []interface{}{}
	if status != nil {
		whereClause += " AND maa.status = ?"
		args = append(args, *status)
	}
	if keyword != "" {
		whereClause += ` AND (
			u.username LIKE ? OR u.phone LIKE ? OR u.nickname LIKE ? OR
			maa.company_name LIKE ? OR maa.credit_code LIKE ? OR maa.contact_name LIKE ? OR maa.contact_phone LIKE ?
		)`
		likeKeyword := "%" + escapeLikeKeyword(keyword) + "%"
		args = append(args, likeKeyword, likeKeyword, likeKeyword, likeKeyword, likeKeyword, likeKeyword, likeKeyword)
	}

	countQuery := `
		SELECT COUNT(*)
		FROM merchant_auth_applications maa
		JOIN users u ON u.id = maa.user_id
	` + whereClause
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT
			maa.id,
			maa.user_id,
			u.username,
			u.phone,
			maa.company_name,
			maa.credit_code,
			maa.contact_name,
			maa.contact_phone,
			maa.license_url,
			maa.status,
			maa.review_comment,
			maa.reviewed_at,
			maa.created_at,
			maa.updated_at,
			COALESCE(u.business_verified, 0)
		FROM merchant_auth_applications maa
		JOIN users u ON u.id = maa.user_id
	` + whereClause + `
		ORDER BY maa.updated_at DESC, maa.created_at DESC
		LIMIT ? OFFSET ?
	`
	if r.db.Dialect() == database.DriverSQLite {
		query = `
			SELECT
				COALESCE(maa.id, maa.rowid) AS id,
				maa.user_id,
				u.username,
				u.phone,
				maa.company_name,
				maa.credit_code,
				maa.contact_name,
				maa.contact_phone,
				maa.license_url,
				maa.status,
				maa.review_comment,
				maa.reviewed_at,
				maa.created_at,
				maa.updated_at,
				COALESCE(u.business_verified, 0)
			FROM merchant_auth_applications maa
			JOIN users u ON u.id = maa.user_id
		` + whereClause + `
			ORDER BY maa.updated_at DESC, maa.created_at DESC
			LIMIT ? OFFSET ?
		`
	}
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*model.MerchantAuthListItem
	for rows.Next() {
		item := &model.MerchantAuthListItem{}
		var reviewedAt sql.NullTime
		var reviewComment sql.NullString
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Username,
			&item.Phone,
			&item.CompanyName,
			&item.CreditCode,
			&item.ContactName,
			&item.ContactPhone,
			&item.LicenseURL,
			&item.Status,
			&reviewComment,
			&reviewedAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.BusinessVerified,
		); err != nil {
			return nil, 0, err
		}
		item.ReviewComment = reviewComment.String
		if reviewedAt.Valid {
			item.ReviewedAt = reviewedAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *MerchantAuthRepository) SaveSubmission(userID int64, app *model.MerchantAuthApplication) (*model.MerchantAuthApplication, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	app.Status = int(model.MerchantAuthStatusPending)
	app.UserID = userID
	app.ReviewComment = ""
	app.ReviewedAt = time.Time{}

	updateQuery := `
		UPDATE merchant_auth_applications
		SET company_name = ?, credit_code = ?, contact_name = ?, contact_phone = ?,
			license_url = ?, status = ?, review_comment = ?, reviewed_at = NULL, updated_at = ?
		WHERE user_id = ?
	`
	result, err := tx.Exec(updateQuery,
		app.CompanyName,
		app.CreditCode,
		app.ContactName,
		app.ContactPhone,
		app.LicenseURL,
		app.Status,
		app.ReviewComment,
		now,
		userID,
	)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if rowsAffected == 0 {
		insertQuery := `
			INSERT INTO merchant_auth_applications (
				user_id, company_name, credit_code, contact_name, contact_phone,
				license_url, status, review_comment, reviewed_at, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)
		`
		id, err := database.InsertReturningID(tx, insertQuery,
			userID,
			app.CompanyName,
			app.CreditCode,
			app.ContactName,
			app.ContactPhone,
			app.LicenseURL,
			app.Status,
			app.ReviewComment,
			now,
			now,
		)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
		if r.db.Dialect() == database.DriverSQLite {
			if _, err := tx.Exec(`UPDATE merchant_auth_applications SET id = ? WHERE rowid = ?`, id, id); err != nil {
				_ = tx.Rollback()
				return nil, err
			}
		}
		app.ID = id
		app.CreatedAt = now
	} else {
		var existingID int64
		if err := tx.QueryRow(`SELECT id FROM merchant_auth_applications WHERE user_id = ? LIMIT 1`, userID).Scan(&existingID); err == nil {
			app.ID = existingID
		}
	}

	if _, err := tx.Exec(`UPDATE users SET business_verified = 0, updated_at = ? WHERE id = ?`, now, userID); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetByUserID(userID)
}

func (r *MerchantAuthRepository) Review(userID int64, approved bool, comment string) (*model.MerchantAuthApplication, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	status := int(model.MerchantAuthStatusRejected)
	if approved {
		status = int(model.MerchantAuthStatusApproved)
	}

	comment = strings.TrimSpace(comment)
	result, err := tx.Exec(`
		UPDATE merchant_auth_applications
		SET status = ?, review_comment = ?, reviewed_at = ?, updated_at = ?
		WHERE user_id = ?
	`, status, comment, now, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if rowsAffected == 0 {
		_ = tx.Rollback()
		return nil, ErrNotFound
	}

	businessVerified := 0
	if approved {
		businessVerified = 1
	}
	if _, err := tx.Exec(`UPDATE users SET business_verified = ?, updated_at = ? WHERE id = ?`, businessVerified, now, userID); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetByUserID(userID)
}

func (r *MerchantAuthRepository) UpdateStatus(userID int64, status int, comment string) (*model.MerchantAuthApplication, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	comment = strings.TrimSpace(comment)

	var reviewedAt interface{}
	switch status {
	case int(model.MerchantAuthStatusPending):
		reviewedAt = nil
		comment = ""
	default:
		reviewedAt = now
	}

	result, err := tx.Exec(`
		UPDATE merchant_auth_applications
		SET status = ?, review_comment = ?, reviewed_at = ?, updated_at = ?
		WHERE user_id = ?
	`, status, comment, reviewedAt, now, userID)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if rowsAffected == 0 {
		_ = tx.Rollback()
		return nil, ErrNotFound
	}

	businessVerified := 0
	if status == int(model.MerchantAuthStatusApproved) {
		businessVerified = 1
	}
	if _, err := tx.Exec(`UPDATE users SET business_verified = ?, updated_at = ? WHERE id = ?`, businessVerified, now, userID); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetByUserID(userID)
}

func (r *MerchantAuthRepository) DeleteByUserID(userID int64) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	result, err := tx.Exec(`DELETE FROM merchant_auth_applications WHERE user_id = ?`, userID)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if rowsAffected == 0 {
		_ = tx.Rollback()
		return ErrNotFound
	}

	if _, err := tx.Exec(`UPDATE users SET business_verified = 0, updated_at = ? WHERE id = ?`, time.Now(), userID); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
