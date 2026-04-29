package repository

import (
	"database/sql"
	"github.com/tans/miao/internal/database"
	"time"

	"github.com/tans/miao/internal/model"
)

type BusinessRepository struct {
	db database.DB
}

func NewBusinessRepository(db database.DB) *BusinessRepository {
	return &BusinessRepository{db: db}
}

// BeginTx starts a new transaction
func (r *BusinessRepository) BeginTx() (database.Tx, error) {
	return r.db.Begin()
}

// GetUserByID retrieves a user by ID
func (r *BusinessRepository) GetUserByID(id int64) (*model.User, error) {
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

// UpdateUserBalance 更新用户余额
func (r *BusinessRepository) UpdateUserBalance(userID int64, balance float64) error {
	query := `
		UPDATE users
		SET balance = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, balance, time.Now(), userID)
	return err
}

// UpdateUserFrozenAmount 更新用户冻结金额
func (r *BusinessRepository) UpdateUserFrozenAmount(userID int64, frozenAmount float64) error {
	query := `
		UPDATE users
		SET frozen_amount = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, frozenAmount, time.Now(), userID)
	return err
}

// UpdateUserPublishCount 更新用户发布任务数
func (r *BusinessRepository) UpdateUserPublishCount(userID int64, count int) error {
	query := `
		UPDATE users
		SET publish_count = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, count, time.Now(), userID)
	return err
}

// CreateTask creates a task with materials. If tx is provided, it uses that transaction
// (caller is responsible for commit). If tx is nil, it creates its own transaction.
func (r *BusinessRepository) CreateTask(task *model.Task, materials []model.TaskMaterialInput, tx database.Tx) error {
	needsCommit := false
	if tx == nil {
		var err error
		tx, err = r.db.Begin()
		if err != nil {
			return err
		}
		needsCommit = true
	}

	commitTx := func() error {
		if needsCommit {
			return tx.Commit()
		}
		return nil
	}

	rollbackTx := func() {
		if needsCommit {
			tx.Rollback()
		}
	}

	if task.Status == 0 {
		task.Status = model.TaskStatusPending
	}

	now := time.Now()
	id, err := database.InsertReturningID(tx, `
		INSERT INTO tasks (business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, total_budget, frozen_amount, paid_amount,
			end_at, created_at, updated_at,
			industries, video_duration, video_aspect, video_resolution,
			creative_style, award_price,
			public, service_fee_rate, service_fee_amount)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.BusinessID, task.Title, task.Description, task.Category,
		task.UnitPrice, task.TotalCount, task.RemainingCount,
		task.Status, task.TotalBudget, task.FrozenAmount, task.PaidAmount,
		task.EndAt, now, now,
		task.Industries, task.VideoDuration, task.VideoAspect, task.VideoResolution,
		task.Styles, task.AwardPrice,
		task.Public, task.ServiceFeeRate, task.ServiceFeeAmount,
	)
	if err != nil {
		rollbackTx()
		return err
	}
	task.ID = id
	task.CreatedAt = now
	task.UpdatedAt = now

	// Insert materials
	taskRepo := &TaskRepository{db: r.db}
	if err = taskRepo.CreateTaskMaterials(tx, id, materials); err != nil {
		rollbackTx()
		return err
	}

	return commitTx()
}

// GetTaskByID 获取任务
func (r *BusinessRepository) GetTaskByID(id int64) (*model.Task, error) {
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at,
			public, service_fee_rate, service_fee_amount
		FROM tasks
		WHERE id = ?
	`
	task := &model.Task{}
	var reviewAt, publishAt, endAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&task.ID,
		&task.BusinessID,
		&task.Title,
		&task.Description,
		&task.Category,
		&task.UnitPrice,
		&task.TotalCount,
		&task.RemainingCount,
		&task.Status,
		&reviewAt,
		&publishAt,
		&endAt,
		&task.TotalBudget,
		&task.FrozenAmount,
		&task.PaidAmount,
		&task.CreatedAt,
		&task.UpdatedAt,
		&task.Public,
		&task.ServiceFeeRate,
		&task.ServiceFeeAmount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if reviewAt.Valid {
		task.ReviewAt = &reviewAt.Time
	}
	if publishAt.Valid {
		task.PublishAt = &publishAt.Time
	}
	if endAt.Valid {
		task.EndAt = &endAt.Time
	}

	taskRepo := &TaskRepository{db: r.db}
	mats, err2 := taskRepo.GetTaskMaterials(task.ID)
	if err2 == nil {
		task.Materials = mats
	}

	return task, nil
}

// UpdateTask 更新任务
func (r *BusinessRepository) UpdateTask(task *model.Task) error {
	query := `
		UPDATE tasks
		SET remaining_count = ?, status = ?, updated_at = ?
		WHERE id = ?
	`
	task.UpdatedAt = time.Now()
	_, err := r.db.Exec(query, task.RemainingCount, task.Status, task.UpdatedAt, task.ID)
	return err
}

// UpdateTaskFrozenAmount 更新任务冻结金额
func (r *BusinessRepository) UpdateTaskFrozenAmount(taskID int64, frozenAmount float64) error {
	query := `
		UPDATE tasks
		SET frozen_amount = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, frozenAmount, time.Now(), taskID)
	return err
}

// UpdateTaskPaidAmount 更新任务已支付金额
func (r *BusinessRepository) UpdateTaskPaidAmount(taskID int64, paidAmount float64) error {
	query := `
		UPDATE tasks
		SET paid_amount = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, paidAmount, time.Now(), taskID)
	return err
}

// ListTasksByBusinessID 获取商家的任务列表
func (r *BusinessRepository) ListTasksByBusinessID(businessID int64) ([]*model.Task, error) {
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at,
			industries, video_duration, video_aspect, video_resolution,
			creative_style, award_price,
			public, service_fee_rate, service_fee_amount,
			COALESCE((SELECT COUNT(*) FROM claims WHERE task_id = tasks.id AND status = 2), 0) AS pending_review_count
		FROM tasks
		WHERE business_id = ?
		ORDER BY created_at DESC
	`
	return r.queryTasks(query, businessID)
}

// queryTasks is a helper to scan task results
func (r *BusinessRepository) queryTasks(query string, args ...interface{}) ([]*model.Task, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		task := &model.Task{}
		var reviewAt, publishAt, endAt sql.NullTime

		err := rows.Scan(
			&task.ID,
			&task.BusinessID,
			&task.Title,
			&task.Description,
			&task.Category,
			&task.UnitPrice,
			&task.TotalCount,
			&task.RemainingCount,
			&task.Status,
			&reviewAt,
			&publishAt,
			&endAt,
			&task.TotalBudget,
			&task.FrozenAmount,
			&task.PaidAmount,
			&task.CreatedAt,
			&task.UpdatedAt,
			// v1.md 规范新增字段
			&task.Industries,
			&task.VideoDuration,
			&task.VideoAspect,
			&task.VideoResolution,
			&task.Styles,
			&task.AwardPrice,
			&task.Public,
			&task.ServiceFeeRate,
			&task.ServiceFeeAmount,
			&task.PendingReviewCount,
		)
		if err != nil {
			return nil, err
		}

		if reviewAt.Valid {
			task.ReviewAt = &reviewAt.Time
		}
		if publishAt.Valid {
			task.PublishAt = &publishAt.Time
		}
		if endAt.Valid {
			task.EndAt = &endAt.Time
		}

		tasks = append(tasks, task)
	}

	return tasks, rows.Err()
}

// GetClaimByID 获取认领记录
func (r *BusinessRepository) GetClaimByID(id int64) (*model.Claim, error) {
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

// ApproveClaim 验收通过
func (r *BusinessRepository) ApproveClaim(claimID int64, reviewAt time.Time, comment string, creatorReward, platformFee float64) error {
	query := `
		UPDATE claims
		SET status = ?, review_at = ?, review_result = ?, review_comment = ?,
			creator_reward = ?, platform_fee = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		model.ClaimStatusApproved,
		reviewAt,
		model.ReviewResultPass,
		comment,
		creatorReward,
		platformFee,
		time.Now(),
		claimID,
	)
	return err
}

// ReturnClaim 退回认领
func (r *BusinessRepository) ReturnClaim(claimID int64, reviewAt time.Time, comment string) error {
	query := `
		UPDATE claims
		SET status = ?, review_at = ?, review_result = ?, review_comment = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		model.ClaimStatusPending,
		reviewAt,
		model.ReviewResultReturn,
		comment,
		time.Now(),
		claimID,
	)
	return err
}

// ReportClaim 举报认领（设置状态为退回，但标记为举报）
func (r *BusinessRepository) ReportClaim(claimID int64, reviewAt time.Time, comment string) error {
	query := `
		UPDATE claims
		SET status = ?, review_at = ?, review_result = ?, review_comment = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		model.ClaimStatusPending,
		reviewAt,
		model.ReviewResultReport,
		comment,
		time.Now(),
		claimID,
	)
	return err
}

// ListClaimsByTaskID 获取任务的认领列表
func (r *BusinessRepository) ListClaimsByTaskID(taskID int64) ([]*model.Claim, error) {
	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned,
			created_at, updated_at
		FROM claims
		WHERE task_id = ?
		ORDER BY created_at DESC
	`
	return r.queryClaims(query, taskID)
}

// ListClaimsByBusinessID 获取商家的所有认领列表
func (r *BusinessRepository) ListClaimsByBusinessID(businessID int64, status *int) ([]*model.Claim, error) {
	var args []interface{}
	query := `
		SELECT c.id, c.task_id, c.creator_id, c.status, c.content, c.submit_at, c.expires_at,
			c.review_at, c.review_result, c.review_comment,
			c.creator_reward, c.platform_fee, c.margin_returned,
			c.created_at, c.updated_at
		FROM claims c
		JOIN tasks t ON c.task_id = t.id
		WHERE t.business_id = ?
	`
	args = append(args, businessID)
	if status != nil {
		query += " AND c.status = ?"
		args = append(args, *status)
	}
	query += " ORDER BY c.created_at DESC"
	return r.queryClaims(query, args...)
}

// ClaimWithDetails holds claim with additional details
type ClaimWithDetails struct {
	*model.Claim
	TaskTitle   string  `json:"task_title"`
	UnitPrice   float64 `json:"unit_price"`
	CreatorName string  `json:"creator_name"`
}

// queryClaims is a helper to scan claim results
func (r *BusinessRepository) queryClaims(query string, args ...interface{}) ([]*model.Claim, error) {
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

// CreateTransaction 创建交易记录
func (r *BusinessRepository) CreateTransaction(tx *model.Transaction) error {
	query := `
		INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query,
		tx.UserID,
		tx.Type,
		tx.Amount,
		tx.BalanceBefore,
		tx.BalanceAfter,
		tx.Remark,
		tx.RelatedID,
		tx.CreatedAt,
	)
	return err
}

// UpdateUserMarginFrozen 更新用户冻结保证金
func (r *BusinessRepository) UpdateUserMarginFrozen(userID int64, marginFrozen float64) error {
	query := `
		UPDATE users
		SET margin_frozen = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, marginFrozen, time.Now(), userID)
	return err
}

// UpdateCreatorLevel 更新创作者等级（基于累计采纳数）
func (r *BusinessRepository) UpdateCreatorLevel(userID int64) error {
	user, err := r.GetUserByID(userID)
	if err != nil || user == nil {
		return err
	}

	// 新等级体系：基于累计采纳数
	// Lv0 (试用): 默认
	// Lv1 (新手): 累计采纳 >= 3
	// Lv2 (活跃): 累计采纳 >= 10
	// Lv3 (优质): 累计采纳 >= 30
	// Lv4 (金牌): 累计采纳 >= 80
	// Lv5 (特约): 累计采纳 >= 200

	adoptedCount := user.AdoptedCount

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

	if newLevel != user.Level {
		query := `
			UPDATE users
			SET level = ?, updated_at = ?
			WHERE id = ?
		`
		_, err = r.db.Exec(query, newLevel, time.Now(), userID)
		return err
	}

	return nil
}

// GetActiveTasks 获取活跃且已到期的任务
func (r *BusinessRepository) GetActiveTasks() ([]*model.Task, error) {
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at,
			industries, video_duration, video_aspect, video_resolution,
			creative_style, award_price,
			public, service_fee_rate, service_fee_amount,
			COALESCE((SELECT COUNT(*) FROM claims WHERE task_id = tasks.id AND status = 2), 0) AS pending_review_count
		FROM tasks
		WHERE status IN (?, ?) AND end_at IS NOT NULL AND end_at < ?
		ORDER BY end_at ASC
	`
	now := time.Now()
	return r.queryTasks(query, model.TaskStatusOnline, model.TaskStatusOngoing, now)
}

// UpdateTaskStatus 更新任务状态
func (r *BusinessRepository) UpdateTaskStatus(taskID int64, status model.TaskStatus) error {
	query := `UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, time.Now(), taskID)
	return err
}

// FinalizeTaskIfCompleted marks a task as ended once all slots are filled and no claims are pending review.
func (r *BusinessRepository) FinalizeTaskIfCompleted(taskID int64) (bool, error) {
	result, err := r.db.Exec(`
		UPDATE tasks
		SET status = ?, updated_at = ?
		WHERE id = ?
		  AND status IN (?, ?)
		  AND remaining_count <= 0
		  AND NOT EXISTS (
		    SELECT 1 FROM claims
		    WHERE claims.task_id = tasks.id AND claims.status = ?
		  )
	`, model.TaskStatusEnded, time.Now(), taskID, model.TaskStatusOnline, model.TaskStatusOngoing, model.ClaimStatusSubmitted)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

// UpdateClaimStatus 更新认领状态
func (r *BusinessRepository) UpdateClaimStatus(claimID int64, status model.ClaimStatus) error {
	query := `UPDATE claims SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, time.Now(), claimID)
	return err
}

// DeleteClaim 物理删除认领记录
func (r *BusinessRepository) DeleteClaim(claimID int64) error {
	query := `DELETE FROM claims WHERE id = ?`
	_, err := r.db.Exec(query, claimID)
	return err
}

// DeleteClaimMaterials 删除某认领的所有媒体文件记录
func (r *BusinessRepository) DeleteClaimMaterials(claimID int64) error {
	query := `DELETE FROM claim_materials WHERE claim_id = ?`
	_, err := r.db.Exec(query, claimID)
	return err
}

// UpdateClaimReward 更新认领的奖励金额（用于拒绝时只支付基础奖励）
func (r *BusinessRepository) UpdateClaimReward(claimID int64, creatorReward, platformFee float64) error {
	query := `
		UPDATE claims
		SET creator_reward = ?, platform_fee = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, creatorReward, platformFee, time.Now(), claimID)
	return err
}

// UpdateUserReportCount 更新用户被举报次数
func (r *BusinessRepository) UpdateUserReportCount(userID int64, reportCount int) error {
	query := `
		UPDATE users
		SET report_count = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, reportCount, time.Now(), userID)
	return err
}

// UpdateCreatorAdoptedCount 更新创作者累计采纳数
func (r *BusinessRepository) UpdateCreatorAdoptedCount(userID int64, adoptedCount int) error {
	query := `
		UPDATE users
		SET adopted_count = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, adoptedCount, time.Now(), userID)
	return err
}

// UpdateUserBalanceTx updates user balance within a transaction
func (r *BusinessRepository) UpdateUserBalanceTx(tx database.Tx, userID int64, balance float64) error {
	query := `
		UPDATE users
		SET balance = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, balance, time.Now(), userID)
	return err
}

// UpdateUserFrozenAmountTx updates user frozen amount within a transaction
func (r *BusinessRepository) UpdateUserFrozenAmountTx(tx database.Tx, userID int64, frozenAmount float64) error {
	query := `
		UPDATE users
		SET frozen_amount = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, frozenAmount, time.Now(), userID)
	return err
}

// UpdateTaskFrozenAmountTx updates task frozen amount within a transaction
func (r *BusinessRepository) UpdateTaskFrozenAmountTx(tx database.Tx, taskID int64, frozenAmount float64) error {
	query := `
		UPDATE tasks
		SET frozen_amount = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, frozenAmount, time.Now(), taskID)
	return err
}

// UpdateUserPublishCountTx updates user publish count within a transaction
func (r *BusinessRepository) UpdateUserPublishCountTx(tx database.Tx, userID int64, count int) error {
	query := `
		UPDATE users
		SET publish_count = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, count, time.Now(), userID)
	return err
}

// CreateTransactionTx creates a transaction record within a transaction
func (r *BusinessRepository) CreateTransactionTx(tx database.Tx, t *model.Transaction) error {
	query := `
		INSERT INTO transactions (user_id, type, amount, balance_before, balance_after, remark, related_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := tx.Exec(query,
		t.UserID,
		t.Type,
		t.Amount,
		t.BalanceBefore,
		t.BalanceAfter,
		t.Remark,
		t.RelatedID,
		t.CreatedAt,
	)
	return err
}

// UpdateCreatorAdoptedCountTx updates creator adopted count within a transaction
func (r *BusinessRepository) UpdateCreatorAdoptedCountTx(tx database.Tx, userID int64, adoptedCount int) error {
	query := `
		UPDATE users
		SET adopted_count = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, adoptedCount, time.Now(), userID)
	return err
}

// ApproveClaimTx approves a claim within a transaction
func (r *BusinessRepository) ApproveClaimTx(tx database.Tx, claimID int64, reviewAt time.Time, comment string, creatorReward, platformFee float64) error {
	query := `
		UPDATE claims
		SET status = ?, review_at = ?, review_result = ?, review_comment = ?,
			creator_reward = ?, platform_fee = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query,
		model.ClaimStatusApproved,
		reviewAt,
		model.ReviewResultPass,
		comment,
		creatorReward,
		platformFee,
		time.Now(),
		claimID,
	)
	return err
}

// ReturnClaimTx returns a claim within a transaction
func (r *BusinessRepository) ReturnClaimTx(tx database.Tx, claimID int64, reviewAt time.Time, comment string) error {
	query := `
		UPDATE claims
		SET status = ?, review_at = ?, review_result = ?, review_comment = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query,
		model.ClaimStatusPending,
		reviewAt,
		model.ReviewResultReturn,
		comment,
		time.Now(),
		claimID,
	)
	return err
}

// ReportClaimTx reports a claim within a transaction
func (r *BusinessRepository) ReportClaimTx(tx database.Tx, claimID int64, reviewAt time.Time, comment string) error {
	query := `
		UPDATE claims
		SET status = ?, review_at = ?, review_result = ?, review_comment = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query,
		model.ClaimStatusPending,
		reviewAt,
		model.ReviewResultReport,
		comment,
		time.Now(),
		claimID,
	)
	return err
}

// UpdateClaimRewardTx updates claim reward within a transaction
func (r *BusinessRepository) UpdateClaimRewardTx(tx database.Tx, claimID int64, creatorReward, platformFee float64) error {
	query := `
		UPDATE claims
		SET creator_reward = ?, platform_fee = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, creatorReward, platformFee, time.Now(), claimID)
	return err
}

// UpdateUserReportCountTx updates user report count within a transaction
func (r *BusinessRepository) UpdateUserReportCountTx(tx database.Tx, userID int64, reportCount int) error {
	query := `
		UPDATE users
		SET report_count = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, reportCount, time.Now(), userID)
	return err
}

// UpdateTaskPaidAmountTx updates task paid amount within a transaction
func (r *BusinessRepository) UpdateTaskPaidAmountTx(tx database.Tx, taskID int64, paidAmount float64) error {
	query := `
		UPDATE tasks
		SET paid_amount = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, paidAmount, time.Now(), taskID)
	return err
}

// UpdateUserMarginFrozenTx updates user margin frozen within a transaction
func (r *BusinessRepository) UpdateUserMarginFrozenTx(tx database.Tx, userID int64, marginFrozen float64) error {
	query := `
		UPDATE users
		SET margin_frozen = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := tx.Exec(query, marginFrozen, time.Now(), userID)
	return err
}

// CreateTaskWithFreeze creates a task and freezes budget atomically in a single transaction.
// This ensures that if any step fails, all changes are rolled back.
func (r *BusinessRepository) CreateTaskWithFreeze(task *model.Task, materials []model.TaskMaterialInput, userID int64, totalBudget float64, oldBalance float64, oldFrozenAmount float64, oldPublishCount int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1. Create task and materials
	if err = r.CreateTask(task, materials, tx); err != nil {
		return err
	}

	// 2. Update user balance
	newBalance := oldBalance - totalBudget
	if err = r.UpdateUserBalanceTx(tx, userID, newBalance); err != nil {
		return err
	}

	rewardEscrowAmount := totalBudget - task.ServiceFeeAmount
	if rewardEscrowAmount < 0 {
		rewardEscrowAmount = 0
	}

	// 3. Update user frozen amount
	newFrozenAmount := oldFrozenAmount + rewardEscrowAmount
	if err = r.UpdateUserFrozenAmountTx(tx, userID, newFrozenAmount); err != nil {
		return err
	}

	// 4. Update task frozen amount
	if err = r.UpdateTaskFrozenAmountTx(tx, task.ID, rewardEscrowAmount); err != nil {
		return err
	}

	// 5. Create transaction record
	transaction := &model.Transaction{
		UserID:        userID,
		Type:          model.TransactionTypeFreeze,
		Amount:        totalBudget,
		BalanceBefore: oldBalance,
		BalanceAfter:  newBalance,
		Remark:        "发布任务冻结: " + task.Title,
		RelatedID:     task.ID,
		CreatedAt:     time.Now(),
	}
	if err = r.CreateTransactionTx(tx, transaction); err != nil {
		return err
	}

	// 6. Recognize service fee as platform income immediately
	if task.ServiceFeeAmount > 0 {
		serviceFeeTx := &model.Transaction{
			UserID:        0,
			Type:          model.TransactionTypePlatformIncome,
			Amount:        task.ServiceFeeAmount,
			BalanceBefore: 0,
			BalanceAfter:  0,
			Remark:        "发布服务费: " + task.Title,
			RelatedID:     task.ID,
			CreatedAt:     time.Now(),
		}
		if err = r.CreateTransactionTx(tx, serviceFeeTx); err != nil {
			return err
		}
	}

	// 7. Update publish count
	if err = r.UpdateUserPublishCountTx(tx, userID, oldPublishCount+1); err != nil {
		return err
	}

	return tx.Commit()
}
