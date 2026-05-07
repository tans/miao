package repository

import (
	"database/sql"
	"github.com/tans/miao/internal/database"
	"time"

	"github.com/tans/miao/internal/model"
)

type TaskRepository struct {
	db database.DB
}

func NewTaskRepository(db database.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// CreateTask creates a new task
func (r *TaskRepository) CreateTask(task *model.Task) error {
	if task.Status == 0 {
		task.Status = model.TaskStatusOnline
	}
	if task.Status == model.TaskStatusOnline && task.PublishAt == nil {
		now := time.Now()
		task.PublishAt = &now
	}

	query := `
		INSERT INTO tasks (business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, total_budget, frozen_amount, paid_amount,
			end_at, review_deadline_at, created_at, updated_at,
			industries, video_duration, video_aspect, video_resolution,
			creative_style, award_price,
			jimeng_link, jimeng_code,
			public, service_fee_rate, service_fee_amount)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)

	`
	now := time.Now()
	id, err := database.InsertReturningID(r.db, query,
		task.BusinessID,
		task.Title,
		task.Description,
		task.Category,
		task.UnitPrice,
		task.TotalCount,
		task.RemainingCount,
		task.Status,
		task.ReviewAt,
		task.PublishAt,
		task.TotalBudget,
		task.FrozenAmount,
		task.PaidAmount,
		task.EndAt,
		task.ReviewDeadlineAt,
		now,
		now,
		// v1.md 规范新增字段
		task.Industries,
		task.VideoDuration,
		task.VideoAspect,
		task.VideoResolution,
		task.Styles,
		task.AwardPrice,
		// 即梦合拍字段
		task.JimengLink,
		task.JimengCode,
		task.Public,
		task.ServiceFeeRate,
		task.ServiceFeeAmount,
	)
	if err != nil {
		return err
	}
	task.ID = id
	task.CreatedAt = now
	task.UpdatedAt = now
	return nil
}

// CreateTaskMaterials inserts task material records. Must be called inside a transaction.
func (r *TaskRepository) CreateTaskMaterials(tx database.Tx, taskID int64, materials []model.TaskMaterialInput) error {
	for i, m := range materials {
		sortOrder := m.SortOrder
		if sortOrder == 0 {
			sortOrder = i
		}
		_, err := tx.Exec(`
			INSERT INTO task_materials (task_id, file_name, file_path, file_size, file_type, sort_order, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			taskID, m.FileName, m.FilePath, m.FileSize, m.FileType, sortOrder, time.Now(),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetTaskMaterials returns all materials for a task ordered by sort_order.
func (r *TaskRepository) GetTaskMaterials(taskID int64) ([]model.TaskMaterial, error) {
	rows, err := r.db.Query(`
		SELECT id, task_id, file_name, file_path, file_size, file_type, sort_order, created_at
		FROM task_materials
		WHERE task_id = ?
		ORDER BY sort_order ASC, id ASC`,
		taskID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var materials []model.TaskMaterial
	for rows.Next() {
		var m model.TaskMaterial
		if err := rows.Scan(&m.ID, &m.TaskID, &m.FileName, &m.FilePath, &m.FileSize, &m.FileType, &m.SortOrder, &m.CreatedAt); err != nil {
			return nil, err
		}
		materials = append(materials, m)
	}
	return materials, rows.Err()
}

// GetTaskByID retrieves a task by ID
func (r *TaskRepository) GetTaskByID(id int64) (*model.Task, error) {
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at, review_deadline_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at,
			industries, video_duration, video_aspect, video_resolution,
			creative_style, award_price,
			jimeng_link, jimeng_code,
			public, service_fee_rate, service_fee_amount
		FROM tasks
		WHERE id = ?
	`
	task := &model.Task{}
	var reviewAt, publishAt, endAt, reviewDeadlineAt sql.NullTime

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
		&reviewDeadlineAt,
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
		// 即梦合拍字段
		&task.JimengLink,
		&task.JimengCode,
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
	if reviewDeadlineAt.Valid {
		task.ReviewDeadlineAt = &reviewDeadlineAt.Time
	}

	// Load materials
	mats, err := r.GetTaskMaterials(task.ID)
	if err == nil {
		task.Materials = mats
	}

	return task, nil
}

// UpdateTask updates a task
func (r *TaskRepository) UpdateTask(task *model.Task) error {
	query := `
		UPDATE tasks
		SET remaining_count = ?, status = ?, updated_at = ?
		WHERE id = ?
	`
	task.UpdatedAt = time.Now()
	_, err := r.db.Exec(query, task.RemainingCount, task.Status, task.UpdatedAt, task.ID)
	return err
}

// DecrementRemainingCount 原子性减少任务剩余数量
// 返回是否成功（false表示已被认领完）
// 优化：使用单条原子UPDATE语句，避免多次查询
func (r *TaskRepository) DecrementRemainingCount(taskID int64) (bool, error) {
	// 使用原子UPDATE：减少数量并检查条件一次性完成
	// CASE WHEN 确保remaining_count > 0 时才减少
	result, err := r.db.Exec(`
		UPDATE tasks
		SET remaining_count = remaining_count - 1,
		    status = CASE WHEN remaining_count <= 1 THEN ? ELSE status END,
		    updated_at = ?
		WHERE id = ? AND remaining_count > 0
	`, model.TaskStatusOngoing, time.Now(), taskID)
	if err != nil {
		return false, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	// 如果没有行被影响，说明任务不存在或已被认领完
	return rowsAffected > 0, nil
}

// IncrementRemainingCount 原子性增加任务剩余数量（用于取消认领时归还名额）
func (r *TaskRepository) IncrementRemainingCount(taskID int64) error {
	_, err := r.db.Exec(`
		UPDATE tasks
		SET remaining_count = remaining_count + 1,
		    status = CASE WHEN status = ? THEN ? ELSE status END,
		    updated_at = ?
		WHERE id = ?
	`, model.TaskStatusOngoing, model.TaskStatusOnline, time.Now(), taskID)
	return err
}

// DeleteTask deletes a task (soft delete by setting status to cancelled)
func (r *TaskRepository) DeleteTask(id int64) error {
	query := `
		UPDATE tasks
		SET status = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query, model.TaskStatusCancelled, time.Now(), id)
	return err
}

// ListTasks lists tasks with pagination and optional status filter
func (r *TaskRepository) ListTasks(status int, limit, offset int) ([]*model.Task, error) {
	var query string
	var args []interface{}

	if status > 0 {
		query = `
			SELECT id, business_id, title, description, category,
				unit_price, total_count, remaining_count,
				status, review_at, publish_at, end_at, review_deadline_at,
				total_budget, frozen_amount, paid_amount,
				created_at, updated_at,
				industries, video_duration, video_aspect, video_resolution,
				creative_style, award_price,
				jimeng_link, jimeng_code,
				public, service_fee_rate, service_fee_amount
			FROM tasks
			WHERE status = ?
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{status, limit, offset}
	} else {
		query = `
			SELECT id, business_id, title, description, category,
				unit_price, total_count, remaining_count,
				status, review_at, publish_at, end_at, review_deadline_at,
				total_budget, frozen_amount, paid_amount,
				created_at, updated_at,
				industries, video_duration, video_aspect, video_resolution,
				creative_style, award_price,
				jimeng_link, jimeng_code,
				public, service_fee_rate, service_fee_amount
			FROM tasks
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{limit, offset}
	}

	return r.queryTasks(query, args...)
}

// ListTasksByBusinessID lists all tasks for a specific merchant
func (r *TaskRepository) ListTasksByBusinessID(businessID int64) ([]*model.Task, error) {
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at, review_deadline_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at,
			industries, video_duration, video_aspect, video_resolution,
			creative_style, award_price,
			jimeng_link, jimeng_code,
			public, service_fee_rate, service_fee_amount
		FROM tasks
		WHERE business_id = ?
		ORDER BY created_at DESC
	`
	return r.queryTasks(query, businessID)
}

// ListAvailableTasks lists available tasks (status=2, remaining>0)
func (r *TaskRepository) ListAvailableTasks(limit, offset int) ([]*model.Task, error) {
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at, review_deadline_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at,
			industries, video_duration, video_aspect, video_resolution,
			creative_style, award_price,
			jimeng_link, jimeng_code,
			public, service_fee_rate, service_fee_amount
		FROM tasks
		WHERE status = ? AND remaining_count > 0
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	return r.queryTasks(query, model.TaskStatusOnline, limit, offset)
}

// ListPublicTasksByCategory lists public tasks filtered by category
func (r *TaskRepository) ListPublicTasksByCategory(category model.TaskCategory, limit, offset int) ([]*model.Task, error) {
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at, review_deadline_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at,
			industries, video_duration, video_aspect, video_resolution,
			creative_style, award_price,
			jimeng_link, jimeng_code,
			public, service_fee_rate, service_fee_amount
		FROM tasks
		WHERE status = ? AND category = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`
	return r.queryTasks(query, model.TaskStatusOnline, category, limit, offset)
}

// queryTasks is a helper to scan task results
func (r *TaskRepository) queryTasks(query string, args ...interface{}) ([]*model.Task, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*model.Task
	for rows.Next() {
		task := &model.Task{}
		var reviewAt, publishAt, endAt, reviewDeadlineAt sql.NullTime

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
			&reviewDeadlineAt,
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
			// 即梦合拍字段
			&task.JimengLink,
			&task.JimengCode,
			&task.Public,
			&task.ServiceFeeRate,
			&task.ServiceFeeAmount,
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
		if reviewDeadlineAt.Valid {
			task.ReviewDeadlineAt = &reviewDeadlineAt.Time
		}

		tasks = append(tasks, task)
	}

	// Populate materials for each task
	for _, t := range tasks {
		mats, err := r.GetTaskMaterials(t.ID)
		if err == nil {
			t.Materials = mats
		}
	}

	return tasks, rows.Err()
}

// ListTasksWithPagination 分页查询任务列表（支持搜索和排序）
// status: 正值表示按指定状态查询；0或负数表示显示所有非取消的任务
func (r *TaskRepository) ListTasksWithPagination(category int, keyword string, sort string, limit, offset int, status ...model.TaskStatus) ([]*model.Task, int, error) {
	// 构建查询条件
	var whereClause string
	var args []interface{}
	if len(status) > 0 && status[0] > 0 {
		whereClause = "WHERE status = ?"
		args = []interface{}{status[0]}
	} else {
		// 显示所有可用任务（已上架、进行中），排除待审核和已取消
		whereClause = "WHERE status IN (?, ?)"
		args = []interface{}{model.TaskStatusOnline, model.TaskStatusOngoing}
	}

	if category > 0 {
		whereClause += " AND category = ?"
		args = append(args, category)
	}

	if keyword != "" {
		whereClause += " AND (title LIKE ? OR description LIKE ?)"
		searchPattern := "%" + keyword + "%"
		args = append(args, searchPattern, searchPattern)
	}

	// 统计总数
	countQuery := "SELECT COUNT(*) FROM tasks " + whereClause
	var total int
	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 构建排序
	orderClause := "ORDER BY created_at DESC"
	switch sort {
	case "price_asc":
		orderClause = "ORDER BY unit_price ASC"
	case "price_desc":
		orderClause = "ORDER BY unit_price DESC"
	case "deadline_asc":
		orderClause = "ORDER BY end_at ASC"
	}

	// 查询数据
	query := `
		SELECT id, business_id, title, description, category,
			unit_price, total_count, remaining_count,
			status, review_at, publish_at, end_at, review_deadline_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at,
			industries, video_duration, video_aspect, video_resolution,
			creative_style, award_price,
			jimeng_link, jimeng_code,
			public, service_fee_rate, service_fee_amount
		FROM tasks
		` + whereClause + `
		` + orderClause + `
		LIMIT ? OFFSET ?
	`
	args = append(args, limit, offset)

	tasks, err := r.queryTasks(query, args...)
	if err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// CountTasksByBusinessID 统计商家任务数
func (r *TaskRepository) CountTasksByBusinessID(businessID int64, status *int) (int, error) {
	var query string
	var args []interface{}

	if status != nil {
		query = "SELECT COUNT(*) FROM tasks WHERE business_id = ? AND status = ?"
		args = []interface{}{businessID, *status}
	} else {
		query = "SELECT COUNT(*) FROM tasks WHERE business_id = ?"
		args = []interface{}{businessID}
	}

	var count int
	err := r.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// GetTaskClaims 获取任务的所有认领列表
func (r *TaskRepository) GetTaskClaims(taskID int64) ([]*model.Claim, error) {
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

// queryClaims is a helper to scan claim results
func (r *TaskRepository) queryClaims(query string, args ...interface{}) ([]*model.Claim, error) {
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
