package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type SubmissionRepository struct {
	db *sql.DB
}

func NewSubmissionRepository(db *sql.DB) *SubmissionRepository {
	return &SubmissionRepository{db: db}
}

// CreateSubmission creates a new submission
func (r *SubmissionRepository) CreateSubmission(sub *model.Submission) error {
	query := `
		INSERT INTO submissions (task_id, creator_id, content, status, award_level,
			score, review_comment, reward_amount, is_used, is_top, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	now := time.Now()
	result, err := r.db.Exec(query,
		sub.TaskID,
		sub.CreatorID,
		sub.Content,
		sub.Status,
		sub.AwardLevel,
		sub.Score,
		sub.ReviewComment,
		sub.RewardAmount,
		sub.IsUsed,
		sub.IsTop,
		now,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	sub.ID = id
	sub.CreatedAt = now
	return nil
}

// GetSubmissionByID retrieves a submission by ID with user info
func (r *SubmissionRepository) GetSubmissionByID(id int64) (*model.Submission, error) {
	query := `
		SELECT s.id, s.task_id, s.creator_id, u.nickname, u.avatar,
			s.content, s.status, s.award_level, s.score, s.review_comment,
			s.reward_amount, s.is_used, s.is_top, s.created_at, s.reviewed_at
		FROM submissions s
		LEFT JOIN users u ON s.creator_id = u.id
		WHERE s.id = ?
	`
	sub := &model.Submission{}
	var reviewedAt sql.NullTime
	var creatorName, creatorAvatar sql.NullString

	err := r.db.QueryRow(query, id).Scan(
		&sub.ID,
		&sub.TaskID,
		&sub.CreatorID,
		&creatorName,
		&creatorAvatar,
		&sub.Content,
		&sub.Status,
		&sub.AwardLevel,
		&sub.Score,
		&sub.ReviewComment,
		&sub.RewardAmount,
		&sub.IsUsed,
		&sub.IsTop,
		&sub.CreatedAt,
		&reviewedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if creatorName.Valid {
		sub.CreatorName = creatorName.String
	}
	if creatorAvatar.Valid {
		sub.CreatorAvatar = creatorAvatar.String
	}
	if reviewedAt.Valid {
		sub.ReviewedAt = &reviewedAt.Time
	}

	return sub, nil
}

// UpdateSubmission updates a submission
func (r *SubmissionRepository) UpdateSubmission(sub *model.Submission) error {
	query := `
		UPDATE submissions
		SET content = ?, status = ?, award_level = ?, score = ?,
			review_comment = ?, reward_amount = ?, is_used = ?, is_top = ?, reviewed_at = ?
		WHERE id = ?
	`
	_, err := r.db.Exec(query,
		sub.Content,
		sub.Status,
		sub.AwardLevel,
		sub.Score,
		sub.ReviewComment,
		sub.RewardAmount,
		sub.IsUsed,
		sub.IsTop,
		sub.ReviewedAt,
		sub.ID,
	)
	return err
}

// ListSubmissionsByTaskID retrieves all submissions for a task with user info
func (r *SubmissionRepository) ListSubmissionsByTaskID(taskID int64) ([]*model.Submission, error) {
	query := `
		SELECT s.id, s.task_id, s.creator_id, u.nickname, u.avatar,
			s.content, s.status, s.award_level, s.score, s.review_comment,
			s.reward_amount, s.is_used, s.is_top, s.created_at, s.reviewed_at
		FROM submissions s
		LEFT JOIN users u ON s.creator_id = u.id
		WHERE s.task_id = ?
		ORDER BY s.created_at DESC
	`
	return r.querySubmissions(query, taskID)
}

// ListSubmissionsByCreatorID retrieves all submissions by a creator with user info
func (r *SubmissionRepository) ListSubmissionsByCreatorID(creatorID int64) ([]*model.Submission, error) {
	query := `
		SELECT s.id, s.task_id, s.creator_id, u.nickname, u.avatar,
			s.content, s.status, s.award_level, s.score, s.review_comment,
			s.reward_amount, s.is_used, s.is_top, s.created_at, s.reviewed_at
		FROM submissions s
		LEFT JOIN users u ON s.creator_id = u.id
		WHERE s.creator_id = ?
		ORDER BY s.created_at DESC
	`
	return r.querySubmissions(query, creatorID)
}

// GetSubmissionByTaskAndCreator retrieves a submission by task and creator with user info
func (r *SubmissionRepository) GetSubmissionByTaskAndCreator(taskID, creatorID int64) (*model.Submission, error) {
	query := `
		SELECT s.id, s.task_id, s.creator_id, u.nickname, u.avatar,
			s.content, s.status, s.award_level, s.score, s.review_comment,
			s.reward_amount, s.is_used, s.is_top, s.created_at, s.reviewed_at
		FROM submissions s
		LEFT JOIN users u ON s.creator_id = u.id
		WHERE s.task_id = ? AND s.creator_id = ?
	`
	sub := &model.Submission{}
	var reviewedAt sql.NullTime
	var creatorName, creatorAvatar sql.NullString

	err := r.db.QueryRow(query, taskID, creatorID).Scan(
		&sub.ID,
		&sub.TaskID,
		&sub.CreatorID,
		&creatorName,
		&creatorAvatar,
		&sub.Content,
		&sub.Status,
		&sub.AwardLevel,
		&sub.Score,
		&sub.ReviewComment,
		&sub.RewardAmount,
		&sub.IsUsed,
		&sub.IsTop,
		&sub.CreatedAt,
		&reviewedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if creatorName.Valid {
		sub.CreatorName = creatorName.String
	}
	if creatorAvatar.Valid {
		sub.CreatorAvatar = creatorAvatar.String
	}
	if reviewedAt.Valid {
		sub.ReviewedAt = &reviewedAt.Time
	}

	return sub, nil
}

// ListSubmissionsByStatus retrieves submissions by status with user info
func (r *SubmissionRepository) ListSubmissionsByStatus(status model.SubmissionStatus) ([]*model.Submission, error) {
	query := `
		SELECT s.id, s.task_id, s.creator_id, u.nickname, u.avatar,
			s.content, s.status, s.award_level, s.score, s.review_comment,
			s.reward_amount, s.is_used, s.is_top, s.created_at, s.reviewed_at
		FROM submissions s
		LEFT JOIN users u ON s.creator_id = u.id
		WHERE s.status = ?
		ORDER BY s.created_at DESC
	`
	return r.querySubmissions(query, status)
}

// CountSubmissionsByTaskID counts submissions for a task
func (r *SubmissionRepository) CountSubmissionsByTaskID(taskID int64) (int, error) {
	query := `SELECT COUNT(*) FROM submissions WHERE task_id = ?`
	var count int
	err := r.db.QueryRow(query, taskID).Scan(&count)
	return count, err
}

// querySubmissions is a helper to scan submission results with user info
func (r *SubmissionRepository) querySubmissions(query string, args ...interface{}) ([]*model.Submission, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []*model.Submission
	for rows.Next() {
		sub := &model.Submission{}
		var reviewedAt sql.NullTime
		var creatorName, creatorAvatar sql.NullString

		err := rows.Scan(
			&sub.ID,
			&sub.TaskID,
			&sub.CreatorID,
			&creatorName,
			&creatorAvatar,
			&sub.Content,
			&sub.Status,
			&sub.AwardLevel,
			&sub.Score,
			&sub.ReviewComment,
			&sub.RewardAmount,
			&sub.IsUsed,
			&sub.IsTop,
			&sub.CreatedAt,
			&reviewedAt,
		)
		if err != nil {
			return nil, err
		}

		if creatorName.Valid {
			sub.CreatorName = creatorName.String
		}
		if creatorAvatar.Valid {
			sub.CreatorAvatar = creatorAvatar.String
		}
		if reviewedAt.Valid {
			sub.ReviewedAt = &reviewedAt.Time
		}

		submissions = append(submissions, sub)
	}

	return submissions, rows.Err()
}

// GetApprovedWork retrieves an approved submission (work) by ID
func (r *SubmissionRepository) GetApprovedWork(id int64) (*model.Submission, error) {
	query := `
		SELECT s.id, s.task_id, s.creator_id, u.nickname, u.avatar,
			s.content, s.status, s.award_level, s.score, s.review_comment,
			s.reward_amount, s.is_used, s.is_top, s.created_at, s.reviewed_at
		FROM submissions s
		LEFT JOIN users u ON s.creator_id = u.id
		WHERE s.id = ? AND s.status = ?
	`
	sub := &model.Submission{}
	var reviewedAt sql.NullTime
	var creatorName, creatorAvatar sql.NullString

	err := r.db.QueryRow(query, id, model.SubmissionPassed).Scan(
		&sub.ID,
		&sub.TaskID,
		&sub.CreatorID,
		&creatorName,
		&creatorAvatar,
		&sub.Content,
		&sub.Status,
		&sub.AwardLevel,
		&sub.Score,
		&sub.ReviewComment,
		&sub.RewardAmount,
		&sub.IsUsed,
		&sub.IsTop,
		&sub.CreatedAt,
		&reviewedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if creatorName.Valid {
		sub.CreatorName = creatorName.String
	}
	if creatorAvatar.Valid {
		sub.CreatorAvatar = creatorAvatar.String
	}
	if reviewedAt.Valid {
		sub.ReviewedAt = &reviewedAt.Time
	}

	return sub, nil
}

// ListApprovedSubmissions retrieves approved submissions with pagination and sorting
func (r *SubmissionRepository) ListApprovedSubmissions(limit, offset int, sort string) ([]*model.Submission, int, error) {
	// Validate sort parameter and build ORDER BY clause
	var orderClause string
	switch sort {
	case "likes":
		orderClause = "s.score DESC, s.created_at DESC"
	case "views":
		orderClause = "s.is_top DESC, s.created_at DESC"
	default:
		orderClause = "s.created_at DESC"
	}

	// Get total count
	countQuery := `
		SELECT COUNT(*)
		FROM submissions s
		WHERE s.status = ?
	`
	var total int
	err := r.db.QueryRow(countQuery, model.SubmissionPassed).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get submissions with pagination
	query := `
		SELECT s.id, s.task_id, s.creator_id, u.nickname, u.avatar,
			s.content, s.status, s.award_level, s.score, s.review_comment,
			s.reward_amount, s.is_used, s.is_top, s.created_at, s.reviewed_at
		FROM submissions s
		LEFT JOIN users u ON s.creator_id = u.id
		WHERE s.status = ?
		ORDER BY ` + orderClause + `
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, model.SubmissionPassed, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var submissions []*model.Submission
	for rows.Next() {
		sub := &model.Submission{}
		var reviewedAt sql.NullTime
		var creatorName, creatorAvatar sql.NullString

		err := rows.Scan(
			&sub.ID,
			&sub.TaskID,
			&sub.CreatorID,
			&creatorName,
			&creatorAvatar,
			&sub.Content,
			&sub.Status,
			&sub.AwardLevel,
			&sub.Score,
			&sub.ReviewComment,
			&sub.RewardAmount,
			&sub.IsUsed,
			&sub.IsTop,
			&sub.CreatedAt,
			&reviewedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		if creatorName.Valid {
			sub.CreatorName = creatorName.String
		}
		if creatorAvatar.Valid {
			sub.CreatorAvatar = creatorAvatar.String
		}
		if reviewedAt.Valid {
			sub.ReviewedAt = &reviewedAt.Time
		}

		submissions = append(submissions, sub)
	}

	return submissions, total, rows.Err()
}

// GetSubmissionMaterials retrieves all materials for a submission
func (r *SubmissionRepository) GetSubmissionMaterials(submissionID int64) ([]*model.SubmissionMaterial, error) {
	query := `
		SELECT id, submission_id, file_name, file_path, file_size, file_type, thumbnail_path, created_at
		FROM submission_materials
		WHERE submission_id = ?
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(query, submissionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var materials []*model.SubmissionMaterial
	for rows.Next() {
		m := &model.SubmissionMaterial{}
		var fileSize sql.NullInt64
		var fileType, thumbnailPath sql.NullString

		err := rows.Scan(
			&m.ID,
			&m.SubmissionID,
			&m.FileName,
			&m.FilePath,
			&fileSize,
			&fileType,
			&thumbnailPath,
			&m.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		if fileSize.Valid {
			m.FileSize = fileSize.Int64
		}
		if fileType.Valid {
			m.FileType = fileType.String
		}
		if thumbnailPath.Valid {
			m.ThumbnailPath = thumbnailPath.String
		}

		materials = append(materials, m)
	}

	return materials, rows.Err()
}

// IncrementViewCount increments the view count for a submission (using score field temporarily)
func (r *SubmissionRepository) IncrementViewCount(submissionID int64) error {
	// Note: Using score field temporarily as view count since there's no dedicated view_count column
	query := `UPDATE submissions SET score = COALESCE(score, 0) + 1 WHERE id = ?`
	_, err := r.db.Exec(query, submissionID)
	return err
}

// CountSubmissionsByCreatorID counts total submissions by a creator
func (r *SubmissionRepository) CountSubmissionsByCreatorID(creatorID int64) (int, error) {
	query := `SELECT COUNT(*) FROM submissions WHERE creator_id = ? AND status = ?`
	var count int
	err := r.db.QueryRow(query, creatorID, model.SubmissionPassed).Scan(&count)
	return count, err
}
