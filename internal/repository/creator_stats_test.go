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

func newCreatorStatsTestDB(t *testing.T) database.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "creator_stats.db")
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
			username TEXT,
			password_hash TEXT,
			is_admin INTEGER DEFAULT 0,
			phone TEXT DEFAULT '',
			nickname TEXT DEFAULT '',
			avatar TEXT DEFAULT '',
			balance REAL DEFAULT 0,
			frozen_amount REAL DEFAULT 0,
			level INTEGER DEFAULT 2,
			adopted_count INTEGER DEFAULT 0,
			margin_frozen REAL DEFAULT 0,
			daily_claim_count INTEGER DEFAULT 0,
			daily_claim_reset DATETIME,
			business_verified INTEGER DEFAULT 0,
			publish_count INTEGER DEFAULT 0,
			status INTEGER DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			business_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			category INTEGER NOT NULL,
			unit_price REAL NOT NULL,
			total_count INTEGER NOT NULL,
			remaining_count INTEGER NOT NULL,
			status INTEGER DEFAULT 1,
			review_at DATETIME,
			publish_at DATETIME,
			end_at DATETIME,
			review_deadline_at DATETIME,
			total_budget REAL NOT NULL,
			frozen_amount REAL DEFAULT 0,
			paid_amount REAL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE claims (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id INTEGER NOT NULL,
			creator_id INTEGER NOT NULL,
			status INTEGER NOT NULL,
			content TEXT DEFAULT '',
			submit_at DATETIME,
			expires_at DATETIME,
			review_at DATETIME,
			review_result INTEGER,
			review_comment TEXT DEFAULT '',
			creator_reward REAL DEFAULT 0,
			platform_fee REAL DEFAULT 0,
			margin_returned REAL DEFAULT 0,
			likes INTEGER DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			type INTEGER NOT NULL,
			amount REAL NOT NULL,
			balance_before REAL NOT NULL,
			balance_after REAL NOT NULL,
			remark TEXT,
			related_id INTEGER,
			created_at DATETIME
		);
	`)
	require.NoError(t, err)

	return db
}

func TestCountPendingClaimsByCreatorIDExcludesReviewedClaims(t *testing.T) {
	db := newCreatorStatsTestDB(t)
	repo := NewCreatorRepository(db)

	_, err := db.Exec(`
		INSERT INTO users (id, username, password_hash, balance, frozen_amount, adopted_count)
		VALUES (1, 'creator', 'hash', 100, 0, 3)
	`)
	require.NoError(t, err)

	now := time.Now()
	_, err = db.Exec(`
		INSERT INTO claims (task_id, creator_id, status, content, submit_at, review_result, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, 1, 1, model.ClaimStatusPending, "", nil, nil, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO claims (task_id, creator_id, status, content, submit_at, review_result, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, 2, 1, model.ClaimStatusPending, "submitted", now, nil, now, now)
	require.NoError(t, err)

	reviewResult := int(model.ReviewResultReturn)
	_, err = db.Exec(`
		INSERT INTO claims (task_id, creator_id, status, content, submit_at, review_result, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, 3, 1, model.ClaimStatusPending, "rejected", now, reviewResult, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO claims (task_id, creator_id, status, content, submit_at, review_result, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, 4, 1, model.ClaimStatusPending, "reported", now, int(model.ReviewResultReport), now, now)
	require.NoError(t, err)

	pending, err := repo.CountPendingClaimsByCreatorID(1)
	require.NoError(t, err)
	require.Equal(t, 1, pending)

	ongoing, err := repo.CountOngoingClaimsByCreatorID(1)
	require.NoError(t, err)
	require.Equal(t, 2, ongoing)
}
