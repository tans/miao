package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type CreatorRepository struct {
	db *sql.DB
}

func NewCreatorRepository(db *sql.DB) *CreatorRepository {
	return &CreatorRepository{db: db}
}

// GetUserByID retrieves a user by ID
func (r *CreatorRepository) GetUserByID(id int64) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, is_admin, phone, nickname, avatar,
			balance, frozen_amount,
			level, behavior_score, trade_score, total_score, margin_frozen,
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
		&user.BehaviorScore,
		&user.TradeScore,
		&user.TotalScore,
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
			return nil, nil
		}
		return nil, err
	}

	user.Nickname = nickname.String
	user.Avatar = avatar.String

	return user, nil
}

// UpdateUserDailyClaim 更新用户每日认领数
func (r *CreatorRepository) UpdateUserDailyClaim(userID int64, count int, resetTime time.Time) error {
	query := `
		UPDATE users
		SET daily_claim_count = ?, daily_claim_reset = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, count, resetTime, time.Now(), userID)
	return err
}

// IncrementDailyClaimCount 原子性增加每日认领数（带上限检查）
func (r *CreatorRepository) IncrementDailyClaimCount(userID int64, maxLimit int) (bool, error) {
	// 使用事务保证原子性
	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// 查询当前值
	var currentCount int
	var resetTime time.Time
	err = tx.QueryRow("SELECT daily_claim_count, daily_claim_reset FROM users WHERE id = ?", userID).Scan(&currentCount, &resetTime)
	if err != nil {
		return false, err
	}

	// 检查是否需要重置
	now := time.Now()
	if now.After(resetTime) {
		currentCount = 0
		resetTime = now.AddDate(0, 0, 1)
	}

	// 检查是否已达上限
	if currentCount >= maxLimit {
		return false, nil
	}

	// 原子性增加
	newCount := currentCount + 1
	_, err = tx.Exec("UPDATE users SET daily_claim_count = ?, daily_claim_reset = ?, updated_at = ? WHERE id = ?",
		newCount, resetTime, now, userID)
	if err != nil {
		return false, err
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	return true, nil
}

// UpdateUserMarginFrozen 更新用户冻结保证金
func (r *CreatorRepository) UpdateUserMarginFrozen(userID int64, marginFrozen float64) error {
	query := `
		UPDATE users
		SET margin_frozen = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, marginFrozen, time.Now(), userID)
	return err
}

// UpdateUserBalance 更新用户余额
func (r *CreatorRepository) UpdateUserBalance(userID int64, balance float64) error {
	query := `
		UPDATE users
		SET balance = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, balance, time.Now(), userID)
	return err
}

// UpdateUserScore 更新用户积分
func (r *CreatorRepository) UpdateUserScore(userID int64, behaviorScore int, tradeScore float64, totalScore int) error {
	query := `
		UPDATE users
		SET behavior_score = ?, trade_score = ?, total_score = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, behaviorScore, tradeScore, totalScore, time.Now(), userID)
	return err
}

// UpdateUserLevel 根据完成任务数更新用户等级
func (r *CreatorRepository) UpdateUserLevel(userID int64) error {
	// 查询完成任务数
	var completedTasks int
	err := r.db.QueryRow("SELECT COUNT(*) FROM claims WHERE creator_id = ? AND status = ?", userID, model.ClaimStatusApproved).Scan(&completedTasks)
	if err != nil {
		return err
	}

	// 计算等级
	var newLevel model.UserLevel
	if completedTasks >= 200 {
		newLevel = model.LevelDiamond
	} else if completedTasks >= 50 {
		newLevel = model.LevelGold
	} else if completedTasks >= 10 {
		newLevel = model.LevelSilver
	} else {
		newLevel = model.LevelBronze
	}

	// 更新等级
	query := `
		UPDATE users
		SET level = ?, updated_at = ?
		WHERE id = ?
	`
	_, err = r.db.Exec(query, newLevel, time.Now(), userID)
	return err
}

// CreateClaim 创建认领记录
func (r *CreatorRepository) CreateClaim(claim *model.Claim) error {
	query := `
		INSERT INTO claims (task_id, creator_id, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		claim.TaskID,
		claim.CreatorID,
		claim.Status,
		claim.ExpiresAt,
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
	claim.ID = id
	claim.CreatedAt = now
	claim.UpdatedAt = now
	return nil
}

// GetClaimByID 获取认领记录
func (r *CreatorRepository) GetClaimByID(id int64) (*model.Claim, error) {
	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned,
			created_at, updated_at
		FROM claims
		WHERE id = ?
	`
	claim := &model.Claim{}
	var content, reviewComment sql.NullString
	var submitAt, reviewAt sql.NullTime
	var reviewResult sql.NullInt64

	err := r.db.QueryRow(query, id).Scan(
		&claim.ID,
		&claim.TaskID,
		&claim.CreatorID,
		&claim.Status,
		&content,
		&submitAt,
		&claim.ExpiresAt,
		&reviewAt,
		&reviewResult,
		&reviewComment,
		&claim.CreatorReward,
		&claim.PlatformFee,
		&claim.MarginReturned,
		&claim.CreatedAt,
		&claim.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	claim.Content = content.String
	claim.ReviewComment = reviewComment.String
	if submitAt.Valid {
		claim.SubmitAt = &submitAt.Time
	}
	if reviewAt.Valid {
		claim.ReviewAt = &reviewAt.Time
	}
	if reviewResult.Valid {
		r := int(reviewResult.Int64)
		claim.ReviewResult = &r
	}

	return claim, nil
}

// SubmitClaim 提交交付
func (r *CreatorRepository) SubmitClaim(claimID int64, content string, submitAt time.Time) error {
	query := `
		UPDATE claims
		SET status = ?, content = ?, submit_at = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, model.ClaimStatusSubmitted, content, submitAt, time.Now(), claimID)
	return err
}

// UpdateClaimStatus 更新认领状态
func (r *CreatorRepository) UpdateClaimStatus(claimID int64, status model.ClaimStatus) error {
	query := `
		UPDATE claims
		SET status = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, status, time.Now(), claimID)
	return err
}

// ListClaimsByCreatorID 获取创作者的认领列表
func (r *CreatorRepository) ListClaimsByCreatorID(creatorID int64) ([]*model.Claim, error) {
	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned,
			created_at, updated_at
		FROM claims
		WHERE creator_id = ?
		ORDER BY created_at DESC
	`
	return r.queryClaims(query, creatorID)
}

// ListClaimsByStatus returns paginated claims filtered by status (for works feed).
func (r *CreatorRepository) ListClaimsByStatus(status model.ClaimStatus, limit, offset int) ([]*model.Claim, int, error) {
	var total int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM claims WHERE status = ?`, status).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned,
			created_at, updated_at
		FROM claims
		WHERE status = ?
		ORDER BY review_at DESC
		LIMIT ? OFFSET ?
	`
	claims, err := r.queryClaims(query, status, limit, offset)
	return claims, total, err
}

// queryClaims is a helper to scan claim results
func (r *CreatorRepository) queryClaims(query string, args ...interface{}) ([]*model.Claim, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var claims []*model.Claim
	for rows.Next() {
		claim := &model.Claim{}
		var content, reviewComment sql.NullString
		var submitAt, reviewAt sql.NullTime
		var reviewResult sql.NullInt64

		err := rows.Scan(
			&claim.ID,
			&claim.TaskID,
			&claim.CreatorID,
			&claim.Status,
			&content,
			&submitAt,
			&claim.ExpiresAt,
			&reviewAt,
			&reviewResult,
			&reviewComment,
			&claim.CreatorReward,
			&claim.PlatformFee,
			&claim.MarginReturned,
			&claim.CreatedAt,
			&claim.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		claim.Content = content.String
		claim.ReviewComment = reviewComment.String
		if submitAt.Valid {
			claim.SubmitAt = &submitAt.Time
		}
		if reviewAt.Valid {
			claim.ReviewAt = &reviewAt.Time
		}
		if reviewResult.Valid {
			r := int(reviewResult.Int64)
			claim.ReviewResult = &r
		}

		claims = append(claims, claim)
	}

	return claims, rows.Err()
}

// CreateClaimMaterial 保存认领媒体文件记录
func (r *CreatorRepository) CreateClaimMaterial(material *model.ClaimMaterial) error {
	query := `
		INSERT INTO claim_materials (claim_id, file_name, file_path, file_size, file_type, thumbnail_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		material.ClaimID,
		material.FileName,
		material.FilePath,
		material.FileSize,
		material.FileType,
		material.ThumbnailPath,
		now,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	material.ID = id
	material.CreatedAt = now
	return nil
}

// GetClaimMaterials 获取某认领的所有媒体文件
func (r *CreatorRepository) GetClaimMaterials(claimID int64) ([]*model.ClaimMaterial, error) {
	query := `
		SELECT id, claim_id, file_name, file_path, file_size, file_type, thumbnail_path, created_at
		FROM claim_materials
		WHERE claim_id = ?
		ORDER BY id ASC
	`
	rows, err := r.db.Query(query, claimID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var materials []*model.ClaimMaterial
	for rows.Next() {
		m := &model.ClaimMaterial{}
		if err := rows.Scan(
			&m.ID, &m.ClaimID, &m.FileName, &m.FilePath,
			&m.FileSize, &m.FileType, &m.ThumbnailPath, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		materials = append(materials, m)
	}
	return materials, rows.Err()
}
