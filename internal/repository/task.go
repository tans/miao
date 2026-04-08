package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// CreateTask creates a new task
func (r *TaskRepository) CreateTask(task *model.Task) error {
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

// GetTaskByID retrieves a task by ID
func (r *TaskRepository) GetTaskByID(id int64) (*model.Task, error) {
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
func (r *TaskRepository) DecrementRemainingCount(taskID int64) (bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// 查询当前值
	var remaining int
	err = tx.QueryRow("SELECT remaining_count FROM tasks WHERE id = ?", taskID).Scan(&remaining)
	if err != nil {
		return false, err
	}

	// 检查是否还有剩余
	if remaining <= 0 {
		return false, nil
	}

	// 原子性减少
	newRemaining := remaining - 1
	var newStatus model.TaskStatus
	if newRemaining == 0 {
		newStatus = model.TaskStatusOngoing
	} else {
		// 获取当前状态
		var currentStatus int
		tx.QueryRow("SELECT status FROM tasks WHERE id = ?", taskID).Scan(&currentStatus)
		newStatus = model.TaskStatus(currentStatus)
	}

	_, err = tx.Exec("UPDATE tasks SET remaining_count = ?, status = ?, updated_at = ? WHERE id = ? AND remaining_count > 0",
		newRemaining, newStatus, time.Now(), taskID)
	if err != nil {
		return false, err
	}

	err = tx.Commit()
	if err != nil {
		return false, err
	}

	return true, nil
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
				status, review_at, publish_at, end_at,
				total_budget, frozen_amount, paid_amount,
				created_at, updated_at
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
				status, review_at, publish_at, end_at,
				total_budget, frozen_amount, paid_amount,
				created_at, updated_at
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
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at
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
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at
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
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at
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

// ListTasksWithPagination 分页查询任务列表（支持搜索和排序）
func (r *TaskRepository) ListTasksWithPagination(category int, keyword string, sort string, limit, offset int) ([]*model.Task, int, error) {
	// 构建查询条件
	whereClause := "WHERE status = ?"
	args := []interface{}{model.TaskStatusOnline}

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
			status, review_at, publish_at, end_at,
			total_budget, frozen_amount, paid_amount,
			created_at, updated_at
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
