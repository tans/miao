package repository

import (
	"database/sql"
	"github.com/tans/miao/internal/database"
	"strings"
	"time"

	"github.com/tans/miao/internal/model"
)

type VideoProcessingRepository struct {
	db database.DB
}

func NewVideoProcessingRepository(db database.DB) *VideoProcessingRepository {
	return &VideoProcessingRepository{db: db}
}

func (r *VideoProcessingRepository) Create(job *model.VideoProcessingJob) error {
	now := time.Now()
	query := `
		INSERT INTO video_processing_jobs (
			job_id, material_id, biz_type, biz_id, source_url, status, attempt,
			processed_url, thumbnail_url, watermark_template, target_format, target_resolution,
			error_message, duration, width, height, watermark_applied, compressed,
			completed_at, last_callback_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	id, err := database.InsertReturningID(r.db, query,
		job.JobID, job.MaterialID, job.BizType, job.BizID, job.SourceURL, job.Status, job.Attempt,
		job.ProcessedURL, job.ThumbnailURL, job.WatermarkTemplate, job.TargetFormat, job.TargetResolution,
		job.ErrorMessage, job.Duration, job.Width, job.Height, job.WatermarkApplied, job.Compressed,
		job.CompletedAt, job.LastCallbackAt, now, now,
	)
	if err != nil {
		return err
	}
	job.ID = id
	job.CreatedAt = now
	job.UpdatedAt = now
	return nil
}

func (r *VideoProcessingRepository) GetByJobID(jobID string) (*model.VideoProcessingJob, error) {
	query := `
		SELECT id, job_id, material_id, biz_type, biz_id, source_url, status,
		       attempt, processed_url, thumbnail_url, watermark_template, target_format, target_resolution,
		       error_message, duration, width, height, watermark_applied, compressed,
		       completed_at, last_callback_at, created_at, updated_at
		FROM video_processing_jobs
		WHERE job_id = ?
	`
	job := &model.VideoProcessingJob{}
	err := r.db.QueryRow(query, jobID).Scan(
		&job.ID, &job.JobID, &job.MaterialID, &job.BizType, &job.BizID, &job.SourceURL, &job.Status,
		&job.Attempt, &job.ProcessedURL, &job.ThumbnailURL, &job.WatermarkTemplate, &job.TargetFormat, &job.TargetResolution,
		&job.ErrorMessage, &job.Duration, &job.Width, &job.Height, &job.WatermarkApplied, &job.Compressed,
		&job.CompletedAt, &job.LastCallbackAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (r *VideoProcessingRepository) UpdateDispatchStatus(jobID, status, errorMessage string) error {
	_, err := r.db.Exec(`
		UPDATE video_processing_jobs
		SET status = ?, error_message = ?, attempt = COALESCE(attempt, 1), updated_at = ?
		WHERE job_id = ?
	`, status, errorMessage, time.Now(), jobID)
	return err
}

func (r *VideoProcessingRepository) UpdateMaterialStatus(materialID int64, status, errorMessage string, processJobID string, retryCount int) error {
	_, err := r.db.Exec(`
		UPDATE claim_materials
		SET process_status = ?, process_error = ?, process_job_id = ?, process_retry_count = ?,
		    updated_at = ?, file_path = CASE WHEN ? <> ? THEN file_path ELSE '' END
		WHERE id = ?
	`, status, errorMessage, processJobID, retryCount, time.Now(), status, model.VideoProcessStatusProcessing, materialID)
	return err
}

func (r *VideoProcessingRepository) ApplyCallback(jobID string, cb *model.VideoProcessingCallback) (*model.VideoProcessingJob, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	job, err := r.getByJobIDTx(tx, jobID)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(cb.JobID) == "" {
		cb.JobID = jobID
	}
	if cb.Attempt > 0 && cb.Attempt < job.Attempt {
		return job, nil
	}
	if videoProcessingStatusPriority(cb.Status) < videoProcessingStatusPriority(job.Status) {
		return job, nil
	}

	now := time.Now()
	var completedAt interface{}
	if cb.Status == model.VideoProcessStatusDone || cb.Status == model.VideoProcessStatusFailed {
		completedAt = now
	}

	_, err = tx.Exec(`
		UPDATE video_processing_jobs
		SET status = ?, attempt = ?, processed_url = ?, thumbnail_url = ?, error_message = ?,
		    duration = ?, width = ?, height = ?, watermark_applied = ?, compressed = ?,
		    completed_at = COALESCE(?, completed_at), last_callback_at = ?, updated_at = ?
		WHERE job_id = ?
	`, cb.Status, job.Attempt, cb.ProcessedURL, cb.ThumbnailURL, cb.ErrorMessage,
		cb.Duration, cb.Width, cb.Height, cb.WatermarkApplied, cb.Compressed,
		completedAt, now, now, jobID)
	if err != nil {
		return nil, err
	}

	displayPath := ""
	processedPath := ""
	if cb.Status == model.VideoProcessStatusDone {
		displayPath = cb.ProcessedURL
		processedPath = cb.ProcessedURL
	}
	processError := cb.ErrorMessage
	_, err = tx.Exec(`
		UPDATE claim_materials
		SET file_path = ?, processed_file_path = ?, thumbnail_path = CASE WHEN ? <> '' THEN ? ELSE thumbnail_path END,
		    process_status = ?, process_error = ?, process_job_id = ?, process_retry_count = ?,
		    watermark_applied = ?, compressed = ?,
		    duration = ?, width = ?, height = ?, updated_at = ?
		WHERE id = ?
	`, displayPath, processedPath, cb.ThumbnailURL, cb.ThumbnailURL,
		cb.Status, processError, job.JobID, job.Attempt, cb.WatermarkApplied, cb.Compressed,
		cb.Duration, cb.Width, cb.Height, now, job.MaterialID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByJobID(jobID)
}

func videoProcessingStatusPriority(status string) int {
	switch strings.TrimSpace(status) {
	case model.VideoProcessStatusPending:
		return 0
	case model.VideoProcessStatusProcessing:
		return 1
	case model.VideoProcessStatusFailed:
		return 2
	case model.VideoProcessStatusDone:
		return 3
	default:
		return 0
	}
}

func (r *VideoProcessingRepository) getByJobIDTx(tx database.Tx, jobID string) (*model.VideoProcessingJob, error) {
	query := `
		SELECT id, job_id, material_id, biz_type, biz_id, source_url, status,
		       attempt, processed_url, thumbnail_url, watermark_template, target_format, target_resolution,
		       error_message, duration, width, height, watermark_applied, compressed,
		       completed_at, last_callback_at, created_at, updated_at
		FROM video_processing_jobs
		WHERE job_id = ?
	`
	job := &model.VideoProcessingJob{}
	err := tx.QueryRow(query, jobID).Scan(
		&job.ID, &job.JobID, &job.MaterialID, &job.BizType, &job.BizID, &job.SourceURL, &job.Status,
		&job.Attempt, &job.ProcessedURL, &job.ThumbnailURL, &job.WatermarkTemplate, &job.TargetFormat, &job.TargetResolution,
		&job.ErrorMessage, &job.Duration, &job.Width, &job.Height, &job.WatermarkApplied, &job.Compressed,
		&job.CompletedAt, &job.LastCallbackAt, &job.CreatedAt, &job.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return job, nil
}
