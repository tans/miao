package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser creates a new user
func (r *UserRepository) CreateUser(user *model.User) error {
	query := `
		INSERT INTO users (username, password_hash, is_admin, phone, nickname, avatar,
			balance, frozen_amount,
			level, adopted_count, margin_frozen,
			daily_claim_count, daily_claim_reset,
			business_verified, publish_count,
			status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		user.Username,
		user.PasswordHash,
		user.IsAdmin,
		user.Phone,
		user.Nickname,
		user.Avatar,
		user.Balance,
		user.FrozenAmount,
		user.Level,
		user.AdoptedCount,
		user.MarginFrozen,
		user.DailyClaimCount,
		user.DailyClaimReset,
		user.BusinessVerified,
		user.PublishCount,
		user.Status,
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
	user.ID = id
	user.CreatedAt = now
	user.UpdatedAt = now
	return nil
}

// GetUserByUsername retrieves a user by username
func (r *UserRepository) GetUserByUsername(username string) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, is_admin, phone, nickname, avatar,
			balance, frozen_amount,
			level, adopted_count, margin_frozen,
			daily_claim_count, daily_claim_reset,
			business_verified, publish_count,
			status, created_at, updated_at
		FROM users
		WHERE username = ?
	`
	user := &model.User{}
	var nickname, avatar sql.NullString

	err := r.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.Phone,
		&nickname,
		&avatar,
		&user.Balance,
		&user.FrozenAmount,
		&user.Level,
		&user.AdoptedCount,
		&user.MarginFrozen,
		&user.DailyClaimCount,
		&user.DailyClaimReset,
		&user.BusinessVerified,
		&user.PublishCount,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	user.Nickname = nickname.String
	user.Avatar = avatar.String

	return user, nil
}

// GetUserByID retrieves a user by ID
func (r *UserRepository) GetUserByID(id int64) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, is_admin, phone, nickname, avatar,
			balance, frozen_amount,
			level, adopted_count, margin_frozen,
			daily_claim_count, daily_claim_reset,
			business_verified, publish_count,
			status, created_at, updated_at
		FROM users
		WHERE id = ?
	`
	user := &model.User{}
	var nickname, avatar sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.Phone,
		&nickname,
		&avatar,
		&user.Balance,
		&user.FrozenAmount,
		&user.Level,
		&user.AdoptedCount,
		&user.MarginFrozen,
		&user.DailyClaimCount,
		&user.DailyClaimReset,
		&user.BusinessVerified,
		&user.PublishCount,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	user.Nickname = nickname.String
	user.Avatar = avatar.String

	return user, nil
}

// GetUserByIDForUpdate retrieves a user by ID with row lock (FOR UPDATE)
// Must be called within a transaction
func (r *UserRepository) GetUserByIDForUpdate(tx *sql.Tx, id int64) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, is_admin, phone, nickname, avatar,
			balance, frozen_amount,
			level, adopted_count, margin_frozen,
			daily_claim_count, daily_claim_reset,
			business_verified, publish_count,
			status, created_at, updated_at
		FROM users
		WHERE id = ?
	`
	user := &model.User{}
	var nickname, avatar sql.NullString

	err := tx.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.Phone,
		&nickname,
		&avatar,
		&user.Balance,
		&user.FrozenAmount,
		&user.Level,
		&user.AdoptedCount,
		&user.MarginFrozen,
		&user.DailyClaimCount,
		&user.DailyClaimReset,
		&user.BusinessVerified,
		&user.PublishCount,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	user.Nickname = nickname.String
	user.Avatar = avatar.String

	return user, nil
}

// UpdateUser updates a user
func (r *UserRepository) UpdateUser(user *model.User) error {
	query := `
		UPDATE users
		SET nickname = ?, avatar = ?, updated_at = ?
		WHERE id = ?
	`
	user.UpdatedAt = time.Now()
	_, err := r.db.Exec(query,
		user.Nickname,
		user.Avatar,
		user.UpdatedAt,
		user.ID,
	)
	return err
}

// UpdateUserForClaim 更新用户认领相关信息
func (r *UserRepository) UpdateUserForClaim(userID int64, marginFrozen float64, dailyClaimCount int) error {
	query := `
		UPDATE users
		SET margin_frozen = ?, daily_claim_count = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, marginFrozen, dailyClaimCount, time.Now(), userID)
	return err
}

// ResetDailyClaimCount 重置每日认领数
func (r *UserRepository) ResetDailyClaimCount(userID int64) error {
	query := `
		UPDATE users
		SET daily_claim_count = 0, daily_claim_reset = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, time.Now().AddDate(0, 0, 1), time.Now(), userID)
	return err
}

// UpdateUserBalance 更新用户余额
func (r *UserRepository) UpdateUserBalance(userID int64, balance float64) error {
	query := `
		UPDATE users
		SET balance = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, balance, time.Now(), userID)
	return err
}

// UpdateUserBalanceWithTx 更新用户余额（事务版本）
func (r *UserRepository) UpdateUserBalanceWithTx(tx *sql.Tx, userID int64, balance float64) error {
	query := `
		UPDATE users
		SET balance = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, balance, time.Now(), userID)
	return err
}

// UpdateUserMarginFrozen 更新用户冻结保证金
func (r *UserRepository) UpdateUserMarginFrozen(userID int64, marginFrozen float64) error {
	query := `
		UPDATE users
		SET margin_frozen = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, marginFrozen, time.Now(), userID)
	return err
}

// UpdateUserFrozenAmount 更新用户冻结金额（商家）
func (r *UserRepository) UpdateUserFrozenAmount(userID int64, frozenAmount float64) error {
	query := `
		UPDATE users
		SET frozen_amount = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, frozenAmount, time.Now(), userID)
	return err
}

// UpdateUserLevel 更新用户等级
func (r *UserRepository) UpdateUserLevel(userID int64, level model.UserLevel) error {
	query := `
		UPDATE users
		SET level = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, level, time.Now(), userID)
	return err
}

// GetUserByPhone retrieves a user by phone (for future use)
func (r *UserRepository) GetUserByPhone(phone string) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, is_admin, phone, nickname, avatar,
			balance, frozen_amount,
			level, behavior_score, trade_score, total_score, margin_frozen,
			daily_claim_count, daily_claim_reset,
			business_verified, publish_count,
			status, created_at, updated_at
		FROM users
		WHERE phone = ?
	`
	user := &model.User{}
	var nickname, avatar sql.NullString

	err := r.db.QueryRow(query, phone).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.Phone,
		&nickname,
		&avatar,
		&user.Balance,
		&user.FrozenAmount,
		&user.Level,
		&user.AdoptedCount,
		&user.MarginFrozen,
		&user.DailyClaimCount,
		&user.DailyClaimReset,
		&user.BusinessVerified,
		&user.PublishCount,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	user.Nickname = nickname.String
	user.Avatar = avatar.String

	return user, nil
}

// ExistsByUsername checks if a user exists by username
func (r *UserRepository) ExistsByUsername(username string) (bool, error) {
	query := `SELECT COUNT(*) FROM users WHERE username = ?`
	var count int
	err := r.db.QueryRow(query, username).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ExistsByPhone checks if a user exists by phone
func (r *UserRepository) ExistsByPhone(phone string) (bool, error) {
	query := `SELECT COUNT(*) FROM users WHERE phone = ?`
	var count int
	err := r.db.QueryRow(query, phone).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetUserByWechatOpenID retrieves a user by Wechat openid
func (r *UserRepository) GetUserByWechatOpenID(openid string) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, is_admin, phone, nickname, avatar, wechat_openid,
			balance, frozen_amount,
			level, adopted_count, margin_frozen,
			daily_claim_count, daily_claim_reset,
			business_verified, publish_count,
			status, created_at, updated_at
		FROM users
		WHERE wechat_openid = ?
	`
	user := &model.User{}
	var nickname, avatar, wechatOpenID sql.NullString

	err := r.db.QueryRow(query, openid).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.IsAdmin,
		&user.Phone,
		&nickname,
		&avatar,
		&wechatOpenID,
		&user.Balance,
		&user.FrozenAmount,
		&user.Level,
		&user.AdoptedCount,
		&user.MarginFrozen,
		&user.DailyClaimCount,
		&user.DailyClaimReset,
		&user.BusinessVerified,
		&user.PublishCount,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	user.Nickname = nickname.String
	user.Avatar = avatar.String
	user.WechatOpenID = wechatOpenID.String

	return user, nil
}

// ExistsByWechatOpenID checks if a user exists by wechat openid
func (r *UserRepository) ExistsByWechatOpenID(openid string) (bool, error) {
	query := `SELECT COUNT(*) FROM users WHERE wechat_openid = ?`
	var count int
	err := r.db.QueryRow(query, openid).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// UpdateWechatOpenID updates user's Wechat openid
func (r *UserRepository) UpdateWechatOpenID(userID int64, openid string) error {
	query := `UPDATE users SET wechat_openid = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, openid, time.Now(), userID)
	return err
}

// ListUsers lists users with pagination and optional filters
func (r *UserRepository) ListUsers(isAdmin *bool, status *int, keyword string, limit, offset int) ([]*model.User, error) {
	query := `
		SELECT id, username, password_hash, is_admin, phone, nickname, avatar,
			balance, frozen_amount,
			level, behavior_score, trade_score, total_score, margin_frozen,
			daily_claim_count, daily_claim_reset,
			business_verified, publish_count,
			status, created_at, updated_at
		FROM users
		WHERE 1=1
	`
	var args []interface{}

	if isAdmin != nil {
		query += " AND is_admin = ?"
		args = append(args, *isAdmin)
	}
	if status != nil {
		query += " AND status = ?"
		args = append(args, *status)
	}
	if keyword != "" {
		query += " AND (username LIKE ? OR phone LIKE ?)"
		args = append(args, "%"+keyword+"%", "%"+keyword+"%")
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	return r.queryUsers(query, args...)
}

// queryUsers is a helper to scan user results
func (r *UserRepository) queryUsers(query string, args ...interface{}) ([]*model.User, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		user := &model.User{}
		var nickname, avatar sql.NullString

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.IsAdmin,
			&user.Phone,
			&nickname,
			&avatar,
			&user.Balance,
			&user.FrozenAmount,
			&user.Level,
			&user.AdoptedCount,
			&user.MarginFrozen,
			&user.DailyClaimCount,
			&user.DailyClaimReset,
			&user.BusinessVerified,
			&user.PublishCount,
			&user.Status,
			&user.CreatedAt,
			&user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		user.Nickname = nickname.String
		user.Avatar = avatar.String

		users = append(users, user)
	}

	return users, rows.Err()
}

// UpdateProfile 更新用户资料
func (r *UserRepository) UpdateProfile(userID int64, nickname, phone, avatar string) error {
	query := `UPDATE users SET nickname = ?, phone = ?, avatar = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, nickname, phone, avatar, time.Now(), userID)
	return err
}

// UpdatePassword 更新密码
func (r *UserRepository) UpdatePassword(userID int64, passwordHash string) error {
	query := `UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, passwordHash, time.Now(), userID)
	return err
}
