package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type BusinessRepository struct {
	db *sql.DB
}

func NewBusinessRepository(db *sql.DB) *BusinessRepository {
	return &BusinessRepository{db: db}
}

// GetUserByID retrieves a user by ID
func (r *BusinessRepository) GetUserByID(id int64) (*model.User, error) {
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

// CreateTask 创建任务
func (r *BusinessRepository) CreateTask(task *model.Task) error {
	query := `
		INSERT INTO tasks (business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, total_budget, frozen_amount, paid_amount,
			end_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		task.BusinessID,
		task.Title,
		task.Description,
		task.Category,
		task.UnitPrice,
		task.TotalCount,
		task.RemainingCount,
		task.Status,
		task.TotalBudget,
		task.FrozenAmount,
		task.PaidAmount,
		task.EndAt,
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
	task.ID = id
	task.CreatedAt = now
	task.UpdatedAt = now
	return nil
}

// GetTaskByID 获取任务
func (r *BusinessRepository) GetTaskByID(id int64) (*model.Task, error) {
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at
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
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
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
			created_at, updated_at
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

// UpdateUserScore 更新用户积分
func (r *BusinessRepository) UpdateUserScore(userID int64, behaviorScore int, tradeScore float64, totalScore int) error {
	query := `
		UPDATE users
		SET behavior_score = ?, trade_score = ?, total_score = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, behaviorScore, tradeScore, totalScore, time.Now(), userID)
	return err
}

// UpdateCreatorLevel 更新创作者等级（移除角色检查，所有用户都是创作者）
func (r *BusinessRepository) UpdateCreatorLevel(userID int64) error {
	user, err := r.GetUserByID(userID)
	if err != nil || user == nil {
		return err
	}

	// 计算已完成订单数和通过率
	// 等级条件:
	// 青铜: 新注册 (默认)
	// 白银: 完成10单+通过率≥70%
	// 黄金: 总积分≥800+完成50单
	// 钻石: 总积分≥1500+完成200单

	var totalClaims, approvedClaims int
	err = r.db.QueryRow("SELECT COUNT(*) FROM claims WHERE creator_id = ?", userID).Scan(&totalClaims)
	if err != nil {
		return err
	}

	err = r.db.QueryRow("SELECT COUNT(*) FROM claims WHERE creator_id = ? AND status = ?", userID, model.ClaimStatusApproved).Scan(&approvedClaims)
	if err != nil {
		return err
	}

	passRate := 0.0
	if totalClaims > 0 {
		passRate = float64(approvedClaims) / float64(totalClaims)
	}

	var newLevel model.UserLevel
	totalScore := user.TotalScore

	// 钻石: 总积分≥1500 + 完成200单
	if totalScore >= 1500 && approvedClaims >= 200 {
		newLevel = model.LevelDiamond
	// 黄金: 总积分≥800 + 完成50单
	} else if totalScore >= 800 && approvedClaims >= 50 {
		newLevel = model.LevelGold
	// 白银: 完成10单 + 通过率≥70%
	} else if approvedClaims >= 10 && passRate >= 0.70 {
		newLevel = model.LevelSilver
	} else {
		newLevel = model.LevelBronze
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
			created_at, updated_at
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

// UpdateClaimStatus 更新认领状态
func (r *BusinessRepository) UpdateClaimStatus(claimID int64, status model.ClaimStatus) error {
	query := `UPDATE claims SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, time.Now(), claimID)
	return err
}
