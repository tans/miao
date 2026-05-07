package service

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/model"
	"github.com/tans/miao/internal/repository"
)

func newVideoProcessingServiceTestDB(t *testing.T) database.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "video_processing_service.db")
	db, err := database.InitDB(config.DatabaseConfig{
		Driver: string(database.DriverSQLite),
		Path:   dbPath,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			username TEXT NOT NULL DEFAULT '',
			password_hash TEXT NOT NULL DEFAULT '',
			is_admin INTEGER DEFAULT 0,
			phone TEXT DEFAULT '',
			nickname TEXT DEFAULT '',
			avatar TEXT DEFAULT '',
			balance REAL DEFAULT 0,
			frozen_amount REAL DEFAULT 0,
			level INTEGER DEFAULT 0,
			adopted_count INTEGER DEFAULT 0,
			margin_frozen REAL DEFAULT 0,
			daily_claim_count INTEGER DEFAULT 0,
			daily_claim_reset TIMESTAMP,
			business_verified INTEGER DEFAULT 0,
			publish_count INTEGER DEFAULT 0,
			status INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE tasks (
			id INTEGER PRIMARY KEY,
			business_id INTEGER NOT NULL,
			title TEXT NOT NULL DEFAULT '',
			description TEXT DEFAULT '',
			category INTEGER DEFAULT 0,
			unit_price REAL DEFAULT 0,
			total_count INTEGER DEFAULT 0,
			remaining_count INTEGER DEFAULT 0,
			status INTEGER DEFAULT 0,
			review_at TIMESTAMP,
			publish_at TIMESTAMP,
			end_at TIMESTAMP,
			total_budget REAL DEFAULT 0,
			frozen_amount REAL DEFAULT 0,
			paid_amount REAL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			industries TEXT DEFAULT '',
			video_duration TEXT DEFAULT '',
			video_aspect TEXT DEFAULT '',
			video_resolution TEXT DEFAULT '',
			creative_style TEXT DEFAULT '',
			award_price REAL DEFAULT 0,
			award_count INTEGER DEFAULT 0,
			public INTEGER DEFAULT 0,
			service_fee_rate REAL DEFAULT 0,
			service_fee_amount REAL DEFAULT 0
		);
		CREATE TABLE claims (
			id INTEGER PRIMARY KEY,
			task_id INTEGER NOT NULL,
			creator_id INTEGER NOT NULL,
			status INTEGER DEFAULT 1,
			content TEXT DEFAULT '',
			submit_at TIMESTAMP,
			expires_at TIMESTAMP,
			review_at TIMESTAMP,
			review_result INTEGER,
			review_comment TEXT DEFAULT '',
			creator_reward REAL DEFAULT 0,
			platform_fee REAL DEFAULT 0,
			margin_returned REAL DEFAULT 0,
			likes INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE claim_materials (
			id INTEGER PRIMARY KEY,
			claim_id INTEGER NOT NULL,
			file_name TEXT NOT NULL DEFAULT '',
			file_path TEXT NOT NULL DEFAULT '',
			source_file_path TEXT DEFAULT '',
			processed_file_path TEXT DEFAULT '',
			file_size INTEGER DEFAULT 0,
			file_type TEXT NOT NULL DEFAULT '',
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
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP
		);
	`)
	require.NoError(t, err)

	return db
}

func TestRetryFailedMaterialsSkipsNullUpdatedAtAndSubmittedHistory(t *testing.T) {
	db := newVideoProcessingServiceTestDB(t)

	now := time.Now()
	_, err := db.Exec(`
		INSERT INTO users (id, username, password_hash, created_at, updated_at)
		VALUES (1, 'biz', 'x', ?, ?)
	`, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO tasks (id, business_id, title, status, remaining_count, total_count, created_at, updated_at)
		VALUES (1, 1, 'task', 3, 1, 1, ?, ?)
	`, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO claims (id, task_id, creator_id, status, content, submit_at, expires_at, created_at, updated_at)
		VALUES (1, 1, 1, 1, 'content', ?, ?, ?, ?)
	`, now, now.Add(24*time.Hour), now, now)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO claim_materials (
			id, claim_id, file_name, file_path, source_file_path, file_type,
			process_status, process_error, process_job_id, process_retry_count,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, 1, 1, "video.mp4", "", "private/source/1/original.mp4", "video",
		model.VideoProcessStatusFailed, "upload failed", "", 0, now, nil)
	require.NoError(t, err)

	svc := &VideoProcessingService{
		cfg: &config.Config{
			VideoProcessing: config.VideoProcessingConfig{
				Enabled: true,
			},
		},
		creatorRepo: repository.NewCreatorRepository(db),
	}

	err = svc.RetryFailedMaterials(10)
	require.NoError(t, err)

	var retryCount int
	var processStatus string
	err = db.QueryRow(`SELECT process_retry_count, process_status FROM claim_materials WHERE id = 1`).Scan(&retryCount, &processStatus)
	require.NoError(t, err)
	require.Equal(t, 0, retryCount)
	require.Equal(t, model.VideoProcessStatusFailed, processStatus)
}
