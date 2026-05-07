package repository

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/model"
)

func newVideoProcessingTestDB(t *testing.T) database.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "video_processing.db")
	db, err := database.InitDB(config.DatabaseConfig{
		Driver: string(database.DriverSQLite),
		Path:   dbPath,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	_, err = db.Exec(`
		CREATE TABLE video_processing_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			job_id TEXT NOT NULL UNIQUE,
			material_id INTEGER NOT NULL,
			biz_type TEXT NOT NULL DEFAULT '',
			biz_id INTEGER NOT NULL DEFAULT 0,
			source_url TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT '',
			attempt INTEGER DEFAULT 1,
			processed_url TEXT DEFAULT '',
			thumbnail_url TEXT DEFAULT '',
			watermark_template TEXT DEFAULT '',
			target_format TEXT DEFAULT '',
			target_resolution TEXT DEFAULT '',
			error_message TEXT DEFAULT '',
			duration REAL DEFAULT 0,
			width INTEGER DEFAULT 0,
			height INTEGER DEFAULT 0,
			watermark_applied INTEGER DEFAULT 0,
			compressed INTEGER DEFAULT 0,
			completed_at DATETIME,
			last_callback_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE TABLE claim_materials (
			id INTEGER PRIMARY KEY,
			claim_id INTEGER DEFAULT 0,
			file_path TEXT DEFAULT '',
			source_file_path TEXT DEFAULT '',
			processed_file_path TEXT DEFAULT '',
			thumbnail_path TEXT DEFAULT '',
			process_status TEXT DEFAULT '',
			process_error TEXT DEFAULT '',
			process_job_id TEXT DEFAULT '',
			process_retry_count INTEGER DEFAULT 0,
			watermark_applied INTEGER DEFAULT 0,
			compressed INTEGER DEFAULT 0,
			duration REAL DEFAULT 0,
			width INTEGER DEFAULT 0,
			height INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	require.NoError(t, err)

	return db
}

func TestVideoProcessingApplyCallbackIgnoresLowerPriorityStatus(t *testing.T) {
	db := newVideoProcessingTestDB(t)
	repo := NewVideoProcessingRepository(db)

	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO claim_materials (
			id, claim_id, file_path, processed_file_path, thumbnail_path, process_status, process_error,
			process_job_id, process_retry_count, watermark_applied, compressed, duration, width, height
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, 1, 99, "https://example.com/processed.mp4", "https://example.com/processed.mp4", "https://example.com/thumb.jpg",
		model.VideoProcessStatusDone, "", "job-1", 0, 1, 1, 30.0, 1920, 1080)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO video_processing_jobs (
			job_id, material_id, biz_type, biz_id, source_url, status, attempt,
			processed_url, thumbnail_url, watermark_template, target_format, target_resolution,
			error_message, duration, width, height, watermark_applied, compressed,
			completed_at, last_callback_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "job-1", 1, "claim_submission", 123, "https://example.com/source.mp4", model.VideoProcessStatusDone, 1,
		"https://cdn.example.com/processed.mp4", "https://cdn.example.com/thumb.jpg", "default", "mp4", "1080P",
		"", 30.0, 1920, 1080, 1, 1, now, now, now, now)
	require.NoError(t, err)

	job, err := repo.ApplyCallback("job-1", &model.VideoProcessingCallback{
		JobID:        "job-1",
		Status:       model.VideoProcessStatusFailed,
		Attempt:      1,
		ErrorMessage: "late failure",
	})
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, model.VideoProcessStatusDone, job.Status)
	require.Equal(t, 1, job.Attempt)

	var status, filePath, processedPath, processStatus, processError string
	require.NoError(t, db.QueryRow(`
		SELECT status, processed_url FROM video_processing_jobs WHERE job_id = ?
	`, "job-1").Scan(&status, &filePath))
	require.Equal(t, model.VideoProcessStatusDone, status)
	require.Equal(t, "https://cdn.example.com/processed.mp4", filePath)

	require.NoError(t, db.QueryRow(`
		SELECT file_path, processed_file_path, process_status, process_error
		FROM claim_materials WHERE id = ?
	`, 1).Scan(&filePath, &processedPath, &processStatus, &processError))
	require.Equal(t, "https://example.com/processed.mp4", filePath)
	require.Equal(t, "https://example.com/processed.mp4", processedPath)
	require.Equal(t, model.VideoProcessStatusDone, processStatus)
	require.Empty(t, processError)
}

func TestVideoProcessingApplyCallbackIgnoresStaleRetriedJob(t *testing.T) {
	db := newVideoProcessingTestDB(t)
	repo := NewVideoProcessingRepository(db)

	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO claim_materials (
			id, claim_id, file_path, processed_file_path, thumbnail_path, process_status, process_error,
			process_job_id, process_retry_count, watermark_applied, compressed, duration, width, height
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, 1, 99, "", "", "", model.VideoProcessStatusProcessing, "", "job-2", 1, 0, 0, 0, 0, 0)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO video_processing_jobs (
			job_id, material_id, biz_type, biz_id, source_url, status, attempt,
			processed_url, thumbnail_url, watermark_template, target_format, target_resolution,
			error_message, duration, width, height, watermark_applied, compressed,
			completed_at, last_callback_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "job-1", 1, "claim_submission", 123, "https://example.com/source.mp4", model.VideoProcessStatusProcessing, 1,
		"", "", "default", "mp4", "1080P",
		"", 0.0, 0, 0, 0, 0, now, now, now, now)
	require.NoError(t, err)

	job, err := repo.ApplyCallback("job-1", &model.VideoProcessingCallback{
		JobID:        "job-1",
		Status:       model.VideoProcessStatusDone,
		Attempt:      1,
		ProcessedURL: "https://cdn.example.com/old.mp4",
	})
	require.NoError(t, err)
	require.Nil(t, job)

	var status, processJobID string
	require.NoError(t, db.QueryRow(`
		SELECT process_status, process_job_id FROM claim_materials WHERE id = ?
	`, 1).Scan(&status, &processJobID))
	require.Equal(t, model.VideoProcessStatusProcessing, status)
	require.Equal(t, "job-2", processJobID)
}
