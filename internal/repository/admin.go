package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/model"
)

type AdminRepository struct {
	db database.DB
}

func NewAdminRepository(db database.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// GetDashboardStats returns dashboard statistics
func (r *AdminRepository) GetDashboardStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total user count
	userCountQuery := `SELECT COUNT(*) FROM users`
	var totalUsers int
	if err := r.db.QueryRow(userCountQuery).Scan(&totalUsers); err != nil {
		return nil, err
	}
	stats["total_users"] = totalUsers

	// Admin count
	adminCountQuery := `SELECT COUNT(*) FROM users WHERE is_admin = 1`
	var totalAdmins int
	if err := r.db.QueryRow(adminCountQuery).Scan(&totalAdmins); err != nil {
		return nil, err
	}
	stats["total_admins"] = totalAdmins

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

	// Inspiration count
	inspirationCountQuery := `SELECT COUNT(*) FROM inspirations`
	var totalInspirations int
	if err := r.db.QueryRow(inspirationCountQuery).Scan(&totalInspirations); err != nil {
		return nil, err
	}
	stats["total_inspirations"] = totalInspirations

	// Active tasks (进行中, status = 3)
	activeTaskQuery := `SELECT COUNT(*) FROM tasks WHERE status = 3`
	var activeTasks int
	if err := r.db.QueryRow(activeTaskQuery).Scan(&activeTasks); err != nil {
		return nil, err
	}
	stats["active_tasks"] = activeTasks

	// Total works (approved claims, status = 3)
	totalWorksQuery := `SELECT COUNT(*) FROM claims WHERE status = 3`
	var totalWorks int
	if err := r.db.QueryRow(totalWorksQuery).Scan(&totalWorks); err != nil {
		return nil, err
	}
	stats["total_works"] = totalWorks

	// Total revenue (platform income from transactions type = 11)
	var totalRevenue float64
	if err := r.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 11").Scan(&totalRevenue); err != nil {
		return nil, err
	}
	stats["total_revenue"] = totalRevenue

	// Today's revenue
	todayStart := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Now().Location())
	var todayRevenue float64
	if err := r.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 11 AND created_at >= ?", todayStart).Scan(&todayRevenue); err != nil {
		return nil, err
	}
	stats["today_revenue"] = todayRevenue

	return stats, nil
}

// ListUsers retrieves users with optional filters
func (r *AdminRepository) ListUsers(isAdmin *bool, status *int, keyword string, limit, offset int) ([]*model.User, error) {
	// Build query
	query := `
		SELECT id, username, password_hash, is_admin, phone, nickname, avatar,
			balance, frozen_amount,
			level, adopted_count, margin_frozen,
			daily_claim_count, daily_claim_reset,
			business_verified, publish_count,
			status, created_at, updated_at
		FROM users
		WHERE 1=1`
	args := []interface{}{}

	if isAdmin != nil {
		query += ` AND is_admin = ?`
		args = append(args, *isAdmin)
	}
	if status != nil && *status > 0 {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	if keyword != "" {
		query += ` AND (username LIKE ? OR phone LIKE ? OR nickname LIKE ? OR wechat_openid LIKE ?)`
		// Escape special LIKE characters to prevent injection
		likeKeyword := "%" + escapeLikeKeyword(keyword) + "%"
		args = append(args, likeKeyword, likeKeyword, likeKeyword, likeKeyword)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	return r.queryUsers(query, args...)
}

// CountUsers counts users with optional filters (for pagination)
func (r *AdminRepository) CountUsers(isAdmin *bool, status *int, keyword string) (int, error) {
	query := `SELECT COUNT(*) FROM users WHERE 1=1`
	args := []interface{}{}

	if isAdmin != nil {
		query += ` AND is_admin = ?`
		args = append(args, *isAdmin)
	}
	if status != nil && *status > 0 {
		query += ` AND status = ?`
		args = append(args, *status)
	}
	if keyword != "" {
		query += ` AND (username LIKE ? OR phone LIKE ? OR nickname LIKE ? OR wechat_openid LIKE ?)`
		likeKeyword := "%" + escapeLikeKeyword(keyword) + "%"
		args = append(args, likeKeyword, likeKeyword, likeKeyword, likeKeyword)
	}

	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// ListUsersAdvanced retrieves users with extended filters for admin panel
func (r *AdminRepository) ListUsersAdvanced(isAdmin *bool, businessVerified *bool, status *int, keyword string, limit, offset int) ([]*model.User, int, error) {
	// Build WHERE clause
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	if isAdmin != nil {
		whereClause += " AND is_admin = ?"
		args = append(args, *isAdmin)
	}
	if businessVerified != nil {
		whereClause += " AND business_verified = ?"
		args = append(args, *businessVerified)
	}
	if status != nil && *status > 0 {
		whereClause += " AND status = ?"
		args = append(args, *status)
	}
	if keyword != "" {
		whereClause += " AND (username LIKE ? OR phone LIKE ? OR nickname LIKE ? OR wechat_openid LIKE ?)"
		likeKeyword := "%" + escapeLikeKeyword(keyword) + "%"
		args = append(args, likeKeyword, likeKeyword, likeKeyword, likeKeyword)
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM users " + whereClause
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get users with task counts via subqueries
	query := `
		SELECT u.id, u.username, u.password_hash, u.is_admin, u.phone, u.nickname, u.avatar,
			u.balance, u.frozen_amount,
			u.level, u.adopted_count, u.margin_frozen,
			u.daily_claim_count, u.daily_claim_reset,
			u.business_verified, u.publish_count,
			u.status, u.created_at, u.updated_at,
			COALESCE((SELECT COUNT(*) FROM tasks WHERE business_id = u.id), 0) as created_tasks_count,
			COALESCE((SELECT COUNT(DISTINCT task_id) FROM claims WHERE creator_id = u.id), 0) as claimed_tasks_count,
			COALESCE((SELECT COUNT(*) FROM claims WHERE creator_id = u.id), 0) as submitted_works_count
		FROM users u ` + whereClause + ` ORDER BY u.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	users, err := r.queryUsers(query, args...)
	return users, total, err
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
		var createdTasksCount, claimedTasksCount, submittedWorksCount int

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
			&createdTasksCount,
			&claimedTasksCount,
			&submittedWorksCount,
		)
		if err != nil {
			return nil, err
		}

		user.Nickname = nickname.String
		user.Avatar = avatar.String
		user.CreatedTasksCount = createdTasksCount
		user.ClaimedTasksCount = claimedTasksCount
		user.SubmittedWorksCount = submittedWorksCount
		normalizeCreatorUser(user)

		users = append(users, user)
	}

	return users, rows.Err()
}

// UpdateUserStatus updates a user's status
func (r *AdminRepository) UpdateUserStatus(userID int64, status int) error {
	query := `UPDATE users SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, time.Now(), userID)
	return err
}

// UpdateUserAdoptedCount updates a user's adopted count
func (r *AdminRepository) UpdateUserAdoptedCount(userID int64, adoptedCount int) error {
	return updateCreatorAdoptedCountAndLevel(r.db, userID, adoptedCount)
}

// CreateCreditLog creates a credit log entry
func (r *AdminRepository) CreateCreditLog(creditLog *model.CreditLog) error {
	query := `INSERT INTO credit_logs (user_id, type, change, reason, related_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query, creditLog.UserID, creditLog.Type, creditLog.Change, creditLog.Reason, creditLog.RelatedID, creditLog.CreatedAt)
	return err
}

// UpdateCreatorLevel updates creator level based on adopted count
func (r *AdminRepository) UpdateCreatorLevel(userID int64) error {
	return refreshCreatorLevelFromAdoptedCount(r.db, userID)
}

// GetUserByID retrieves a user by ID
func (r *AdminRepository) GetUserByID(id int64) (*model.User, error) {
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
	normalizeCreatorUser(user)

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

// CountTasks 获取任务总数
func (r *AdminRepository) CountTasks(status *int, keyword string) (int, error) {
	query := `SELECT COUNT(*) FROM tasks WHERE 1=1`
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

	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
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

// UpdateAppealResult updates the result of an appeal (used by business)
func (r *AdminRepository) UpdateAppealResult(id int64, result string) error {
	query := `UPDATE appeals SET status = 2, result = ?, handle_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, result, time.Now(), id)
	return err
}

// GetTaskIDsByBusinessID retrieves all task IDs owned by a business
func (r *AdminRepository) GetTaskIDsByBusinessID(businessID int64) ([]int64, error) {
	query := `SELECT id FROM tasks WHERE business_id = ?`
	rows, err := r.db.Query(query, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetAppealsByTaskIDs retrieves appeals for a list of task IDs
func (r *AdminRepository) GetAppealsByTaskIDs(taskIDs []int64, limit, offset int) ([]*model.Appeal, int, error) {
	if len(taskIDs) == 0 {
		return []*model.Appeal{}, 0, nil
	}

	// Build placeholders for IN clause
	placeholders := ""
	args := []interface{}{}
	for i, id := range taskIDs {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, id)
	}

	// Get total count
	countQuery := `SELECT COUNT(*) FROM appeals WHERE target_id IN (` + placeholders + `) AND type = 1`
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get appeals
	query := `
		SELECT id, user_id, type, target_id, reason, evidence, status, result, admin_id, handle_at, created_at
		FROM appeals
		WHERE target_id IN (` + placeholders + `) AND type = 1
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
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

	return appeals, total, rows.Err()
}

// GetTasksByBusinessID retrieves all tasks created by a business user
func (r *AdminRepository) GetTasksByBusinessID(businessID int64, limit, offset int) ([]*model.Task, int, error) {
	// Count total
	countQuery := `SELECT COUNT(*) FROM tasks WHERE business_id = ?`
	var total int
	if err := r.db.QueryRow(countQuery, businessID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get tasks
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at
		FROM tasks
		WHERE business_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	rows, err := r.db.Query(query, businessID, limit, offset)
	if err != nil {
		return nil, 0, err
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
			return nil, 0, err
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

	return tasks, total, rows.Err()
}

// GetClaimsByCreatorID retrieves all claims (participated tasks) for a creator
func (r *AdminRepository) GetClaimsByCreatorID(creatorID int64, limit, offset int) ([]*model.Claim, int, error) {
	// Count total
	countQuery := `SELECT COUNT(*) FROM claims WHERE creator_id = ?`
	var total int
	if err := r.db.QueryRow(countQuery, creatorID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get claims
	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned,
			created_at, updated_at
		FROM claims
		WHERE creator_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	claims, err := r.queryClaims(query, creatorID, limit, offset)
	return claims, total, err
}

// GetSubmittedWorksByCreatorID retrieves submitted works (claims with status >= submitted) for a creator
func (r *AdminRepository) GetSubmittedWorksByCreatorID(creatorID int64, limit, offset int) ([]*model.Claim, int, error) {
	// Count total
	countQuery := `SELECT COUNT(*) FROM claims WHERE creator_id = ? AND status >= 2`
	var total int
	if err := r.db.QueryRow(countQuery, creatorID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get submitted claims
	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned,
			created_at, updated_at
		FROM claims
		WHERE creator_id = ? AND status >= 2
		ORDER BY submit_at DESC
		LIMIT ? OFFSET ?
	`
	claims, err := r.queryClaims(query, creatorID, limit, offset)
	return claims, total, err
}

// ListWorksAdmin 获取所有作品（已验收的认领）
func (r *AdminRepository) ListWorksAdmin(keyword string, limit, offset int) ([]*model.Claim, int, error) {
	// Build WHERE clause
	whereClause := "WHERE status = ?"
	args := []interface{}{model.ClaimStatusApproved}

	if keyword != "" {
		whereClause += " AND (content LIKE ? OR creator_id IN (SELECT id FROM users WHERE username LIKE ? OR nickname LIKE ?))"
		likeKeyword := "%" + escapeLikeKeyword(keyword) + "%"
		args = append(args, likeKeyword, likeKeyword, likeKeyword)
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM claims " + whereClause
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get works
	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned,
			created_at, updated_at
		FROM claims ` + whereClause + ` ORDER BY review_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	claims, err := r.queryClaims(query, args...)
	return claims, total, err
}

// GetWorkByIDAdmin 获取作品详情（用于管理员）
func (r *AdminRepository) GetWorkByIDAdmin(id int64) (*model.Claim, error) {
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
		res := int(reviewResult.Int64)
		claim.ReviewResult = &res
	}

	return claim, nil
}

// UpdateWorkContentAdmin 更新作品内容（管理员）
func (r *AdminRepository) UpdateWorkContentAdmin(id int64, content string) error {
	query := `UPDATE claims SET content = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, content, time.Now(), id)
	return err
}

// UpdateWorkReviewResultAdmin 更新作品审核结果（管理员）
func (r *AdminRepository) UpdateWorkReviewResultAdmin(id int64, result int, comment string) error {
	query := `UPDATE claims SET review_result = ?, review_comment = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, result, comment, time.Now(), id)
	return err
}

// DeleteWorkAdmin 删除作品（将状态改为已取消）
func (r *AdminRepository) DeleteWorkAdmin(id int64) error {
	query := `UPDATE claims SET status = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, model.ClaimStatusCancelled, time.Now(), id)
	return err
}

// GetFinanceStats returns finance statistics for admin
func (r *AdminRepository) GetFinanceStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total platform balance (sum of all user balances)
	var totalBalance float64
	err := r.db.QueryRow("SELECT COALESCE(SUM(balance), 0) FROM users").Scan(&totalBalance)
	if err != nil {
		return nil, err
	}
	stats["total_balance"] = totalBalance

	// Total recharge amount (type = 1)
	var totalRecharge float64
	err = r.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 1").Scan(&totalRecharge)
	if err != nil {
		return nil, err
	}
	stats["total_recharge"] = totalRecharge

	// Total withdraw amount (type = 6)
	var totalWithdraw float64
	err = r.db.QueryRow("SELECT COALESCE(SUM(ABS(amount)), 0) FROM transactions WHERE type = 6").Scan(&totalWithdraw)
	if err != nil {
		return nil, err
	}
	stats["total_withdraw"] = totalWithdraw

	// Total transaction count
	var totalTransactions int
	err = r.db.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&totalTransactions)
	if err != nil {
		return nil, err
	}
	stats["total_transactions"] = totalTransactions

	// Monthly data for charts (last 6 months)
	monthlyData := make(map[string]interface{})
	rechargeByMonth := make([]float64, 6)
	withdrawByMonth := make([]float64, 6)
	labels := make([]string, 6)

	now := time.Now()
	for i := 5; i >= 0; i-- {
		month := now.AddDate(0, -i, 0)
		monthStart := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, month.Location())
		monthEnd := monthStart.AddDate(0, 1, -1)

		labels[5-i] = fmt.Sprintf("%d月", month.Month())

		var monthRecharge, monthWithdraw float64
		r.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 1 AND created_at >= ? AND created_at <= ?", monthStart, monthEnd).Scan(&monthRecharge)
		r.db.QueryRow("SELECT COALESCE(SUM(ABS(amount)), 0) FROM transactions WHERE type = 6 AND created_at >= ? AND created_at <= ?", monthStart, monthEnd).Scan(&monthWithdraw)

		rechargeByMonth[5-i] = monthRecharge
		withdrawByMonth[5-i] = monthWithdraw
	}
	monthlyData["labels"] = labels
	monthlyData["recharge"] = rechargeByMonth
	monthlyData["withdraw"] = withdrawByMonth
	stats["monthly_data"] = monthlyData

	// Type distribution
	typeDistribution := make(map[string]int)
	var rechargeCount, withdrawCount, taskRewardCount, taskPaymentCount, refundCount int
	r.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE type = 1").Scan(&rechargeCount)
	r.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE type = 6").Scan(&withdrawCount)
	r.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE type = 5").Scan(&taskRewardCount)
	r.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE type = 2").Scan(&taskPaymentCount)
	r.db.QueryRow("SELECT COUNT(*) FROM transactions WHERE type = 7").Scan(&refundCount)

	typeDistribution["recharge"] = rechargeCount
	typeDistribution["withdraw"] = withdrawCount
	typeDistribution["task_reward"] = taskRewardCount
	typeDistribution["task_payment"] = taskPaymentCount
	typeDistribution["refund"] = refundCount
	stats["type_distribution"] = typeDistribution

	return stats, nil
}

// ListAllTransactions retrieves all transactions with filters (for admin finance page)
func (r *AdminRepository) ListAllTransactions(txType string, timeRange string, keyword string, limit, offset int) ([]*model.Transaction, int, error) {
	// Build WHERE clause
	whereClause := "WHERE 1=1"
	args := []interface{}{}

	// Filter by type
	if txType != "" {
		typeMap := map[string]int{
			"recharge":              1,
			"task_payment":          2,
			"freeze":                3,
			"unfreeze":              4,
			"task_reward":           5,
			"withdraw":              6,
			"refund":                7,
			"commission":            8,
			"participation_payment": 9,
			"award_payment":         10,
			"platform_income":       11,
		}
		if t, ok := typeMap[txType]; ok {
			whereClause += " AND type = ?"
			args = append(args, t)
		}
	}

	// Filter by time range
	if timeRange != "" {
		now := time.Now()
		var startTime time.Time
		switch timeRange {
		case "today":
			startTime = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		case "week":
			startTime = now.AddDate(0, 0, -7)
		case "month":
			startTime = now.AddDate(0, -1, 0)
		case "year":
			startTime = now.AddDate(-1, 0, 0)
		}
		if !startTime.IsZero() {
			whereClause += " AND created_at >= ?"
			args = append(args, startTime)
		}
	}

	// Filter by keyword (user name)
	if keyword != "" {
		whereClause += " AND user_id IN (SELECT id FROM users WHERE username LIKE ? OR nickname LIKE ?)"
		likeKeyword := "%" + escapeLikeKeyword(keyword) + "%"
		args = append(args, likeKeyword, likeKeyword)
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM transactions " + whereClause
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Get transactions
	query := `
		SELECT id, user_id, type, amount, balance_before, balance_after, remark, related_id, created_at
		FROM transactions ` + whereClause + ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transactions []*model.Transaction
	for rows.Next() {
		t := &model.Transaction{}
		if err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.Type,
			&t.Amount,
			&t.BalanceBefore,
			&t.BalanceAfter,
			&t.Remark,
			&t.RelatedID,
			&t.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, t)
	}

	return transactions, total, rows.Err()
}

// GetTransactionByID retrieves a transaction by ID
func (r *AdminRepository) GetTransactionByID(id int64) (*model.Transaction, error) {
	query := `
		SELECT id, user_id, type, amount, balance_before, balance_after, remark, related_id, created_at
		FROM transactions
		WHERE id = ?
	`
	tx := &model.Transaction{}
	err := r.db.QueryRow(query, id).Scan(
		&tx.ID,
		&tx.UserID,
		&tx.Type,
		&tx.Amount,
		&tx.BalanceBefore,
		&tx.BalanceAfter,
		&tx.Remark,
		&tx.RelatedID,
		&tx.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return tx, nil
}

// GetClaimsByTaskID retrieves all claims for a task
func (r *AdminRepository) GetClaimsByTaskID(taskID int64, limit, offset int) ([]*model.Claim, error) {
	query := `
		SELECT id, task_id, creator_id, status, content, submit_at, expires_at,
			review_at, review_result, review_comment,
			creator_reward, platform_fee, margin_returned,
			created_at, updated_at
		FROM claims
		WHERE task_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	claims, err := r.queryClaims(query, taskID, limit, offset)
	return claims, err
}

// GetSettings retrieves the system settings
func (r *AdminRepository) GetSettings() (*model.SystemSettings, error) {
	settings := &model.SystemSettings{}
	err := r.db.QueryRow(`
		SELECT review_days, submit_days, grace_days, report_action, min_unit_price, min_award_price
		FROM system_settings WHERE id = 1
	`).Scan(&settings.ReviewDays, &settings.SubmitDays, &settings.GraceDays, &settings.ReportAction, &settings.MinUnitPrice, &settings.MinAwardPrice)
	if err != nil {
		return nil, err
	}
	return settings, nil
}

// UpdateSettings updates the system settings
func (r *AdminRepository) UpdateSettings(settings *model.SystemSettings) error {
	_, err := r.db.Exec(`
		UPDATE system_settings
		SET review_days = ?, submit_days = ?, grace_days = ?, report_action = ?,
		    min_unit_price = ?, min_award_price = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, settings.ReviewDays, settings.SubmitDays, settings.GraceDays, settings.ReportAction, settings.MinUnitPrice, settings.MinAwardPrice)
	return err
}

// GetAISettings retrieves AI model configuration.
func (r *AdminRepository) GetAISettings() (*model.AISettings, error) {
	if err := r.ensureAISettingsColumns(); err != nil {
		return nil, err
	}
	settings := &model.AISettings{}
	err := r.db.QueryRow(`
		SELECT ai_api_key, ai_api_endpoint, ai_model,
		       ocr_access_key_id, ocr_access_key_secret, ocr_endpoint, ocr_security_token
		FROM system_settings WHERE id = 1
	`).Scan(&settings.APIKey, &settings.APIEndpoint, &settings.Model,
		&settings.OCRAccessKeyID, &settings.OCRAccessKeySecret, &settings.OCREndpoint, &settings.OCRSecurityToken)
	if err != nil {
		if err == sql.ErrNoRows {
			_, _ = r.db.Exec(`INSERT OR IGNORE INTO system_settings (id) VALUES (1)`)
			return &model.AISettings{}, nil
		}
		return nil, err
	}
	return settings, nil
}

// UpdateAISettings updates AI model configuration.
func (r *AdminRepository) UpdateAISettings(settings *model.AISettings) error {
	if err := r.ensureAISettingsColumns(); err != nil {
		return err
	}
	_, _ = r.db.Exec(`INSERT OR IGNORE INTO system_settings (id) VALUES (1)`)

	if _, err := r.db.Exec(`
		UPDATE system_settings
		SET ai_api_key = ?, ai_api_endpoint = ?, ai_model = ?,
		    ocr_access_key_id = ?, ocr_access_key_secret = ?, ocr_endpoint = ?, ocr_security_token = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, settings.APIKey, settings.APIEndpoint, settings.Model,
		settings.OCRAccessKeyID, settings.OCRAccessKeySecret, settings.OCREndpoint, settings.OCRSecurityToken); err == nil {
		return nil
	}

	_ = r.ensureAISettingsColumns()
	_, err := r.db.Exec(`
		UPDATE system_settings
		SET ai_api_key = ?, ai_api_endpoint = ?, ai_model = ?,
		    ocr_access_key_id = ?, ocr_access_key_secret = ?, ocr_endpoint = ?, ocr_security_token = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, settings.APIKey, settings.APIEndpoint, settings.Model,
		settings.OCRAccessKeyID, settings.OCRAccessKeySecret, settings.OCREndpoint, settings.OCRSecurityToken)
	return err
}

func (r *AdminRepository) ensureAISettingsColumns() error {
	stmts := []string{
		`ALTER TABLE system_settings ADD COLUMN ai_api_key TEXT DEFAULT ''`,
		`ALTER TABLE system_settings ADD COLUMN ai_api_endpoint TEXT DEFAULT ''`,
		`ALTER TABLE system_settings ADD COLUMN ai_model TEXT DEFAULT ''`,
		`ALTER TABLE system_settings ADD COLUMN ocr_access_key_id TEXT DEFAULT ''`,
		`ALTER TABLE system_settings ADD COLUMN ocr_access_key_secret TEXT DEFAULT ''`,
		`ALTER TABLE system_settings ADD COLUMN ocr_endpoint TEXT DEFAULT ''`,
		`ALTER TABLE system_settings ADD COLUMN ocr_security_token TEXT DEFAULT ''`,
	}
	for _, stmt := range stmts {
		_, _ = r.db.Exec(stmt)
	}
	return nil
}
