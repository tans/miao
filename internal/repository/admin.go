package repository

import (
	"database/sql"
	"strings"
	"time"

	"github.com/tans/miao/internal/model"
)

type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// GetDashboardStats returns dashboard statistics
func (r *AdminRepository) GetDashboardStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// User count by role
	userCountQuery := `SELECT COUNT(*) FROM users`
	var totalUsers int
	if err := r.db.QueryRow(userCountQuery).Scan(&totalUsers); err != nil {
		return nil, err
	}
	stats["total_users"] = totalUsers

	// Creator count
	creatorCountQuery := `SELECT COUNT(*) FROM users WHERE role = 'creator'`
	var totalCreators int
	if err := r.db.QueryRow(creatorCountQuery).Scan(&totalCreators); err != nil {
		return nil, err
	}
	stats["total_creators"] = totalCreators

	// Business count
	businessCountQuery := `SELECT COUNT(*) FROM users WHERE role = 'business'`
	var totalBusinesses int
	if err := r.db.QueryRow(businessCountQuery).Scan(&totalBusinesses); err != nil {
		return nil, err
	}
	stats["total_businesses"] = totalBusinesses

	// Task count
	taskCountQuery := `SELECT COUNT(*) FROM tasks`
	var totalTasks int
	if err := r.db.QueryRow(taskCountQuery).Scan(&totalTasks); err != nil {
		return nil, err
	}
	stats["total_tasks"] = totalTasks

	// Pending tasks (待审核)
	pendingTaskQuery := `SELECT COUNT(*) FROM tasks WHERE status = 1`
	var pendingTasks int
	if err := r.db.QueryRow(pendingTaskQuery).Scan(&pendingTasks); err != nil {
		return nil, err
	}
	stats["pending_tasks"] = pendingTasks

	// Claim count
	claimCountQuery := `SELECT COUNT(*) FROM claims`
	var totalClaims int
	if err := r.db.QueryRow(claimCountQuery).Scan(&totalClaims); err != nil {
		return nil, err
	}
	stats["total_claims"] = totalClaims

	// Transaction amount (sum of all transactions)
	transactionAmountQuery := `SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 1`
	var totalTransactionAmount float64
	if err := r.db.QueryRow(transactionAmountQuery).Scan(&totalTransactionAmount); err != nil {
		return nil, err
	}
	stats["total_transaction_amount"] = totalTransactionAmount

	// Pending appeals count
	appealCountQuery := `SELECT COUNT(*) FROM appeals WHERE status = 1`
	var pendingAppeals int
	if err := r.db.QueryRow(appealCountQuery).Scan(&pendingAppeals); err != nil {
		return nil, err
	}
	stats["pending_appeals"] = pendingAppeals

	return stats, nil
}

// ListUsers retrieves users with optional filters
func (r *AdminRepository) ListUsers(role *int, status *int, keyword string, limit, offset int) ([]*model.User, error) {
	// Build query
	query := `
		SELECT id, username, password_hash, role, phone, nickname, avatar,
			balance, frozen_amount,
			level, behavior_score, trade_score, total_score, margin_frozen,
			daily_claim_count, daily_claim_reset,
			business_verified, publish_count,
			status, created_at, updated_at
		FROM users
		WHERE 1=1`
	args := []interface{}{}

	if role != nil && *role > 0 {
		roleStr := roleIntToString(*role)
		query += ` AND role = ?`
		args = append(args, roleStr)
	}
	if status != nil && *status > 0 {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	if keyword != "" {
		query += ` AND (username LIKE ? OR phone LIKE ? OR nickname LIKE ?)`
		// Escape special LIKE characters to prevent injection
		likeKeyword := "%" + escapeLikeKeyword(keyword) + "%"
		args = append(args, likeKeyword, likeKeyword, likeKeyword)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	return r.queryUsers(query, args...)
}

// queryUsers is a helper to scan user results
func (r *AdminRepository) queryUsers(query string, args ...interface{}) ([]*model.User, error) {
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
			&user.Role,
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
			return nil, err
		}

		user.Nickname = nickname.String
		user.Avatar = avatar.String

		users = append(users, user)
	}

	return users, rows.Err()
}

// roleIntToString converts role int to string
func roleIntToString(role int) string {
	switch role {
	case 1:
		return "admin"
	case 2:
		return "business"
	case 3:
		return "creator"
	default:
		return "creator"
	}
}

// UpdateUserStatus updates a user's status
func (r *AdminRepository) UpdateUserStatus(userID int64, status int) error {
	query := `UPDATE users SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, time.Now(), userID)
	return err
}

// UpdateUserScore updates a user's score
func (r *AdminRepository) UpdateUserScore(userID int64, behaviorScore int, tradeScore float64, totalScore int) error {
	query := `UPDATE users SET behavior_score = ?, trade_score = ?, total_score = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, behaviorScore, tradeScore, totalScore, time.Now(), userID)
	return err
}

// CreateCreditLog creates a credit log entry
func (r *AdminRepository) CreateCreditLog(creditLog *model.CreditLog) error {
	query := `INSERT INTO credit_logs (user_id, type, change, reason, related_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, creditLog.UserID, creditLog.Type, creditLog.Change, creditLog.Reason, creditLog.RelatedID, creditLog.CreatedAt)
	return err
}

// UpdateCreatorLevel updates creator level based on score
func (r *AdminRepository) UpdateCreatorLevel(userID int64) error {
	user, err := r.GetUserByID(userID)
	if err != nil || user == nil {
		return err
	}

	if user.Role != "creator" {
		return nil
	}

	var newLevel model.UserLevel
	totalScore := user.TotalScore

	if totalScore >= 1500 {
		newLevel = model.LevelDiamond
	} else if totalScore >= 800 {
		newLevel = model.LevelGold
	} else if totalScore >= 100 {
		newLevel = model.LevelSilver
	} else {
		newLevel = model.LevelBronze
	}

	if newLevel != user.Level {
		query := `UPDATE users SET level = ?, updated_at = ? WHERE id = ?`
		_, err = r.db.Exec(query, newLevel, time.Now(), userID)
		return err
	}

	return nil
}

// GetUserByID retrieves a user by ID
func (r *AdminRepository) GetUserByID(id int64) (*model.User, error) {
	query := `
		SELECT id, username, password_hash, role, phone, nickname, avatar,
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
		&user.Role,
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

// UpdateUserBalance updates user balance
func (r *AdminRepository) UpdateUserBalance(userID int64, balance float64) error {
	query := `UPDATE users SET balance = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, balance, time.Now(), userID)
	return err
}

// UpdateUserFrozenAmount updates user frozen amount
func (r *AdminRepository) UpdateUserFrozenAmount(userID int64, frozenAmount float64) error {
	query := `UPDATE users SET frozen_amount = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, frozenAmount, time.Now(), userID)
	return err
}

// GetTaskByID retrieves a task by ID
func (r *AdminRepository) GetTaskByID(id int64) (*model.Task, error) {
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

// ApproveTask 审核通过任务
func (r *AdminRepository) ApproveTask(taskID int64, reviewAt time.Time) error {
	query := `UPDATE tasks SET status = ?, review_at = ?, publish_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, model.TaskStatusOnline, reviewAt, reviewAt, time.Now(), taskID)
	return err
}

// RejectTask 审核拒绝任务
func (r *AdminRepository) RejectTask(taskID int64, reviewAt time.Time, comment string) error {
	query := `UPDATE tasks SET status = ?, review_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, model.TaskStatusCancelled, reviewAt, time.Now(), taskID)
	return err
}

// ListTasks 获取任务列表
func (r *AdminRepository) ListTasks(status *int, keyword string, limit, offset int) ([]*model.Task, error) {
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at
		FROM tasks
		WHERE 1=1`
	args := []interface{}{}

	if status != nil && *status > 0 {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	if keyword != "" {
		query += ` AND (title LIKE ? OR description LIKE ?)`
		likeKeyword := "%" + escapeLikeKeyword(keyword) + "%"
		args = append(args, likeKeyword, likeKeyword)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	return r.queryTasks(query, args...)
}

// queryTasks is a helper to scan task results
func (r *AdminRepository) queryTasks(query string, args ...interface{}) ([]*model.Task, error) {
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

// ListClaims 获取认领列表
func (r *AdminRepository) ListClaims(status *int, limit, offset int) ([]*model.Claim, error) {
	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned,
			created_at, updated_at
		FROM claims
		WHERE 1=1`
	args := []interface{}{}

	if status != nil && *status > 0 {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	return r.queryClaims(query, args...)
}

// queryClaims is a helper to scan claim results
func (r *AdminRepository) queryClaims(query string, args ...interface{}) ([]*model.Claim, error) {
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
func (r *AdminRepository) CreateTransaction(tx *model.Transaction) error {
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

// GetAllAppeals retrieves all appeals (for admin)
func (r *AdminRepository) GetAllAppeals(status, appealType, limit, offset int) ([]*model.Appeal, int, error) {
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
		SELECT id, user_id, type, target_id, reason, status, result, created_at
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
		var result sql.NullString
		if err := rows.Scan(
			&appeal.ID,
			&appeal.UserID,
			&appeal.Type,
			&appeal.TargetID,
			&appeal.Reason,
			&appeal.Status,
			&result,
			&appeal.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		appeal.Result = result.String
		appeals = append(appeals, appeal)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return appeals, total, nil
}

// ResolveAppeal resolves an appeal
func (r *AdminRepository) ResolveAppeal(id int64, result string) error {
	query := `UPDATE appeals SET status = 2, result = ? WHERE id = ?`
	_, err := r.db.Exec(query, result, id)
	return err
}

// escapeLikeKeyword escapes special characters in LIKE queries
func escapeLikeKeyword(keyword string) string {
	keyword = strings.ReplaceAll(keyword, "\\", "\\\\")
	keyword = strings.ReplaceAll(keyword, "%", "\\%")
	keyword = strings.ReplaceAll(keyword, "_", "\\_")
	return keyword
}
