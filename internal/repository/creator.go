package repository

import (
	"database/sql"
	"github.com/tans/miao/internal/database"
	"time"

	"github.com/tans/miao/internal/model"
)

type CreatorRepository struct {
	db database.DB
}

func NewCreatorRepository(db database.DB) *CreatorRepository {
	return &CreatorRepository{db: db}
}

// GetUserByID retrieves a user by ID
func (r *CreatorRepository) GetUserByID(id int64) (*model.User, error) {
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
	now := time.Now()

	// 使用单个原子UPDATE语句，通过WHERE子句检查上限，避免TOCTOU竞态条件
	result, err := r.db.Exec(`
		UPDATE users
		SET daily_claim_count = CASE
				WHEN daily_claim_reset <= ? THEN 1
				ELSE daily_claim_count + 1
			END,
			daily_claim_reset = CASE
				WHEN daily_claim_reset <= ? THEN ? + INTERVAL '1 day'
				ELSE daily_claim_reset
			END,
			updated_at = ?
		WHERE id = ?
			AND (CASE WHEN daily_claim_reset <= ? THEN 0 ELSE daily_claim_count END) < ?
	`, now, now, now, now, userID, now, maxLimit)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	// 如果没有行被更新，说明已达上限
	return rowsAffected > 0, nil
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

// UpdateUserLevel 根据累计采纳数更新用户等级
func (r *CreatorRepository) UpdateUserLevel(userID int64) error {
	// 查询累计采纳数
	var adoptedCount int
	err := r.db.QueryRow("SELECT adopted_count FROM users WHERE id = ?", userID).Scan(&adoptedCount)
	if err != nil {
		return err
	}

	// 计算等级
	var newLevel model.UserLevel
	if adoptedCount >= 200 {
		newLevel = model.LevelExclusive
	} else if adoptedCount >= 80 {
		newLevel = model.LevelGold
	} else if adoptedCount >= 30 {
		newLevel = model.LevelQuality
	} else if adoptedCount >= 10 {
		newLevel = model.LevelActive
	} else if adoptedCount >= 3 {
		newLevel = model.LevelNewbie
	} else {
		newLevel = model.LevelTrial
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

// IncrementAdoptedCount 增加用户采纳数
func (r *CreatorRepository) IncrementAdoptedCount(userID int64) error {
	query := `UPDATE users SET adopted_count = adopted_count + 1, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), userID)
	return err
}

// CreateClaim 创建认领记录
func (r *CreatorRepository) CreateClaim(claim *model.Claim) error {
	query := `
		INSERT INTO claims (task_id, creator_id, status, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	id, err := database.InsertReturningID(r.db, query,
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
			likes, created_at, updated_at
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
		&claim.Likes,
		&claim.CreatedAt,
		&claim.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
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

// DeleteClaim 物理删除认领记录（取消时使用）
func (r *CreatorRepository) DeleteClaim(claimID int64) error {
	query := `DELETE FROM claims WHERE id = ?`
	_, err := r.db.Exec(query, claimID)
	return err
}

// GetClaimByTaskIDAndCreatorID 获取某用户对某任务的认领记录
func (r *CreatorRepository) GetClaimByTaskIDAndCreatorID(taskID, creatorID int64) (*model.Claim, error) {
	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned, likes,
			created_at, updated_at
		FROM claims
		WHERE task_id = ? AND creator_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`
	claim := &model.Claim{}
	var content, reviewComment sql.NullString
	var submitAt, reviewAt sql.NullTime
	var reviewResult sql.NullInt64

	err := r.db.QueryRow(query, taskID, creatorID).Scan(
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
		&claim.Likes,
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
	claim.SubmitAt = &submitAt.Time
	claim.ReviewAt = &reviewAt.Time
	if reviewResult.Valid {
		reviewResultInt := int(reviewResult.Int64)
		claim.ReviewResult = &reviewResultInt
	}
	return claim, nil
}

// CountPendingClaimsByCreatorID 统计某用户待提交的认领数量
func (r *CreatorRepository) CountPendingClaimsByCreatorID(creatorID int64) (int, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM claims WHERE creator_id = ? AND status = ?`,
		creatorID, model.ClaimStatusPending,
	).Scan(&count)
	return count, err
}

// ListClaimsByCreatorID 获取创作者的认领列表（排除已取消的）
func (r *CreatorRepository) ListClaimsByCreatorID(creatorID int64) ([]*model.Claim, error) {
	query := `
		SELECT c.id, c.task_id, c.creator_id, c.status, c.content, c.submit_at, c.expires_at,
			c.review_at, c.review_result, c.review_comment,
			c.creator_reward, c.platform_fee, c.margin_returned,
			c.likes, c.created_at, c.updated_at,
			t.title as task_title, t.status as task_status, t.end_at as task_end_at
		FROM claims c
		LEFT JOIN tasks t ON c.task_id = t.id
		WHERE c.creator_id = ? AND c.status != ?
		ORDER BY c.created_at DESC
	`
	return r.queryClaimsWithTaskTitle(query, creatorID, model.ClaimStatusCancelled)
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
			likes, created_at, updated_at
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
			&claim.Likes,
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

// queryClaimsWithTaskTitle is a helper to scan claim results with task_title joined from tasks table
func (r *CreatorRepository) queryClaimsWithTaskTitle(query string, args ...interface{}) ([]*model.Claim, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var claims []*model.Claim
	for rows.Next() {
		claim := &model.Claim{}
		var content, reviewComment, taskTitle sql.NullString
		var submitAt, reviewAt, taskEndAt sql.NullTime
		var reviewResult, taskStatus sql.NullInt64

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
			&claim.Likes,
			&claim.CreatedAt,
			&claim.UpdatedAt,
			&taskTitle,
			&taskStatus,
			&taskEndAt,
		)
		if err != nil {
			return nil, err
		}

		claim.Content = content.String
		claim.ReviewComment = reviewComment.String
		claim.TaskTitle = taskTitle.String
		if taskStatus.Valid {
			claim.TaskStatus = model.TaskStatus(taskStatus.Int64)
		}
		if taskEndAt.Valid {
			claim.TaskEndAt = &taskEndAt.Time
		}
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
		INSERT INTO claim_materials (
			claim_id, file_name, file_path, source_file_path, processed_file_path,
			file_size, file_type, thumbnail_path, process_status, process_error,
			watermark_applied, compressed, duration, width, height, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	id, err := database.InsertReturningID(r.db, query,
		material.ClaimID,
		material.FileName,
		material.FilePath,
		material.SourceFilePath,
		material.ProcessedFilePath,
		material.FileSize,
		material.FileType,
		material.ThumbnailPath,
		material.ProcessStatus,
		material.ProcessError,
		material.WatermarkApplied,
		material.Compressed,
		material.Duration,
		material.Width,
		material.Height,
		now,
	)
	if err != nil {
		return err
	}
	material.ID = id
	material.CreatedAt = now
	return nil
}

// DeleteClaimMaterials 删除某认领的所有媒体文件记录
func (r *CreatorRepository) DeleteClaimMaterials(claimID int64) error {
	query := `DELETE FROM claim_materials WHERE claim_id = ?`
	_, err := r.db.Exec(query, claimID)
	return err
}

// GetClaimMaterials 获取某认领的所有媒体文件
func (r *CreatorRepository) GetClaimMaterials(claimID int64) ([]*model.ClaimMaterial, error) {
	query := `
		SELECT id, claim_id, file_name, file_path, source_file_path, processed_file_path,
		       file_size, file_type, thumbnail_path, process_status, process_error,
		       watermark_applied, compressed, duration, width, height, created_at
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
			&m.ID, &m.ClaimID, &m.FileName, &m.FilePath, &m.SourceFilePath, &m.ProcessedFilePath,
			&m.FileSize, &m.FileType, &m.ThumbnailPath, &m.ProcessStatus, &m.ProcessError,
			&m.WatermarkApplied, &m.Compressed, &m.Duration, &m.Width, &m.Height, &m.CreatedAt,
		); err != nil {
			return nil, err
		}
		materials = append(materials, m)
	}
	return materials, rows.Err()
}

// HasWorkLiked 检查用户是否已点赞作品
func (r *CreatorRepository) HasWorkLiked(workID, userID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM work_likes WHERE work_id = ? AND user_id = ?`, workID, userID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// AddWorkLike 添加作品点赞
func (r *CreatorRepository) AddWorkLike(workID, userID int64) (bool, error) {
	liked, err := r.HasWorkLiked(workID, userID)
	if err != nil {
		return false, err
	}
	if liked {
		return false, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`INSERT INTO work_likes (work_id, user_id, created_at) VALUES (?, ?, ?)`, workID, userID, time.Now()); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`UPDATE claims SET likes = likes + 1, updated_at = ? WHERE id = ?`, time.Now(), workID); err != nil {
		return false, err
	}

	return true, tx.Commit()
}

// RemoveWorkLike 取消作品点赞
func (r *CreatorRepository) RemoveWorkLike(workID, userID int64) (bool, error) {
	liked, err := r.HasWorkLiked(workID, userID)
	if err != nil {
		return false, err
	}
	if !liked {
		return false, nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM work_likes WHERE work_id = ? AND user_id = ?`, workID, userID); err != nil {
		return false, err
	}
	if _, err := tx.Exec(`UPDATE claims SET likes = CASE WHEN likes > 0 THEN likes - 1 ELSE 0 END, updated_at = ? WHERE id = ?`, time.Now(), workID); err != nil {
		return false, err
	}

	return true, tx.Commit()
}

// GetWorkLikeCount 获取作品点赞数
func (r *CreatorRepository) GetWorkLikeCount(workID int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(`SELECT COALESCE(likes, 0) FROM claims WHERE id = ?`, workID).Scan(&count)
	return count, err
}
