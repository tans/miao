package repository

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/tans/miao/internal/model"
)

func newBusinessEconomyTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	t.Cleanup(func() {
		_ = db.Close()
	})

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			username TEXT,
			password_hash TEXT,
			balance REAL DEFAULT 0,
			frozen_amount REAL DEFAULT 0,
			publish_count INTEGER DEFAULT 0,
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
			total_budget REAL NOT NULL,
			frozen_amount REAL DEFAULT 0,
			paid_amount REAL DEFAULT 0,
			end_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			industries TEXT DEFAULT '',
			video_duration TEXT DEFAULT '',
			video_aspect TEXT DEFAULT '',
			video_resolution TEXT DEFAULT '',
			creative_style TEXT DEFAULT '',
			award_price REAL DEFAULT 0,
			public INTEGER DEFAULT 0,
			service_fee_rate REAL DEFAULT 0.10,
			service_fee_amount REAL DEFAULT 0
		);
		CREATE TABLE task_materials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id INTEGER NOT NULL,
			file_name TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_size INTEGER DEFAULT 0,
			file_type TEXT NOT NULL,
			sort_order INTEGER DEFAULT 0,
			thumbnail_path TEXT,
			created_at DATETIME
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

func TestCreateTaskWithFreezeRecognizesServiceFeeImmediately(t *testing.T) {
	db := newBusinessEconomyTestDB(t)
	repo := NewBusinessRepository(db)

	_, err := db.Exec(`
		INSERT INTO users (id, username, password_hash, balance, frozen_amount, publish_count)
		VALUES (1, 'merchant', 'hash', 1000, 0, 0)
	`)
	require.NoError(t, err)

	task := &model.Task{
		BusinessID:       1,
		Title:            "服务费测试任务",
		Description:      "test",
		Category:         model.CategoryVideo,
		UnitPrice:        10,
		AwardPrice:       20,
		TotalCount:       2,
		RemainingCount:   2,
		Status:           model.TaskStatusOnline,
		TotalBudget:      66,
		FrozenAmount:     0,
		PaidAmount:       0,
		ServiceFeeRate:   0.10,
		ServiceFeeAmount: 6,
		Public:           false,
	}

	err = repo.CreateTaskWithFreeze(
		task,
		[]model.TaskMaterialInput{{FileName: "cover.jpg", FilePath: "/cover.jpg", FileType: "image"}},
		1,
		task.TotalBudget,
		1000,
		0,
		0,
	)
	require.NoError(t, err)

	var balance, frozen float64
	require.NoError(t, db.QueryRow(`SELECT balance, frozen_amount FROM users WHERE id = 1`).Scan(&balance, &frozen))
	require.Equal(t, 934.0, balance)
	require.Equal(t, 60.0, frozen)

	var taskFrozen, taskTotal, serviceFee float64
	require.NoError(t, db.QueryRow(`SELECT frozen_amount, total_budget, service_fee_amount FROM tasks WHERE id = ?`, task.ID).Scan(&taskFrozen, &taskTotal, &serviceFee))
	require.Equal(t, 60.0, taskFrozen)
	require.Equal(t, 66.0, taskTotal)
	require.Equal(t, 6.0, serviceFee)

	var platformIncomeCount int
	var platformIncomeAmount float64
	require.NoError(t, db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE user_id = 0 AND type = ?
	`, model.TransactionTypePlatformIncome).Scan(&platformIncomeCount, &platformIncomeAmount))
	require.Equal(t, 1, platformIncomeCount)
	require.Equal(t, 6.0, platformIncomeAmount)
}
