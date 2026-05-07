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

func newAppealTestDB(t *testing.T) database.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "appeals.db")
	db, err := database.InitDB(config.DatabaseConfig{
		Driver: string(database.DriverSQLite),
		Path:   dbPath,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})

	_, err = db.Exec(`
		CREATE TABLE appeals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			type INTEGER NOT NULL,
			claim_id INTEGER DEFAULT NULL,
			target_id INTEGER NOT NULL,
			reason TEXT NOT NULL,
			evidence TEXT,
			status INTEGER DEFAULT 1,
			result TEXT,
			admin_id INTEGER,
			handle_at DATETIME,
			created_at DATETIME NOT NULL
		);
	`)
	require.NoError(t, err)

	return db
}

func TestAppealRepositoryCreateGetAndUpdate(t *testing.T) {
	db := newAppealTestDB(t)
	repo := NewAppealRepository(db)

	appeal := &model.Appeal{
		UserID:   7,
		Type:     model.AppealTypeTask,
		ClaimID:  99,
		TargetID: 99,
		Reason:   "请复核任务审核结果",
		Evidence: "evidence-1.jpg,evidence-2.jpg",
		Status:   model.AppealStatusPending,
	}

	require.NoError(t, repo.CreateAppeal(appeal))
	require.NotZero(t, appeal.ID)
	require.False(t, appeal.CreatedAt.IsZero())

	fetched, err := repo.GetAppealByID(appeal.ID)
	require.NoError(t, err)
	require.Equal(t, appeal.ID, fetched.ID)
	require.Equal(t, appeal.UserID, fetched.UserID)
	require.Equal(t, appeal.Type, fetched.Type)
	require.Equal(t, appeal.TargetID, fetched.TargetID)
	require.Equal(t, appeal.Reason, fetched.Reason)
	require.Equal(t, appeal.Evidence, fetched.Evidence)
	require.Equal(t, appeal.Status, fetched.Status)
	require.Empty(t, fetched.Result)
	require.Zero(t, fetched.AdminID)
	require.Nil(t, fetched.HandleAt)

	require.NoError(t, repo.UpdateAppealStatus(appeal.ID, int(model.AppealStatusResolved), "已通过复核"))
	fetched, err = repo.GetAppealByID(appeal.ID)
	require.NoError(t, err)
	require.Equal(t, model.AppealStatusResolved, fetched.Status)
	require.Equal(t, "已通过复核", fetched.Result)
	require.Zero(t, fetched.AdminID)
	require.NotNil(t, fetched.HandleAt)

	require.NoError(t, repo.UpdateAppealWithAdmin(appeal.ID, int(model.AppealStatusResolved), "管理员已处理", 88))
	fetched, err = repo.GetAppealByID(appeal.ID)
	require.NoError(t, err)
	require.Equal(t, model.AppealStatusResolved, fetched.Status)
	require.Equal(t, "管理员已处理", fetched.Result)
	require.Equal(t, int64(88), fetched.AdminID)
	require.NotNil(t, fetched.HandleAt)
}

func TestAppealRepositoryListFiltersAndPagination(t *testing.T) {
	db := newAppealTestDB(t)
	repo := NewAppealRepository(db)

	base := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	seed := []struct {
		userID     int64
		appealType int
		targetID   int64
		reason     string
		status     int
		createdAt  time.Time
	}{
		{userID: 1, appealType: 1, targetID: 11, reason: "first", status: 1, createdAt: base.Add(-2 * time.Hour)},
		{userID: 1, appealType: 1, targetID: 12, reason: "second", status: 2, createdAt: base.Add(-1 * time.Hour)},
		{userID: 2, appealType: 1, targetID: 13, reason: "third", status: 1, createdAt: base},
		{userID: 1, appealType: 1, targetID: 14, reason: "fourth", status: 1, createdAt: base.Add(1 * time.Hour)},
	}

	for _, item := range seed {
		_, err := db.Exec(`
			INSERT INTO appeals (user_id, type, claim_id, target_id, reason, evidence, status, result, admin_id, handle_at, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, item.userID, item.appealType, item.targetID, item.targetID, item.reason, "", item.status, "", nil, nil, item.createdAt)
		require.NoError(t, err)
	}

	appeals, total, err := repo.ListAppealsByUserID(1, 2, 0)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, appeals, 2)
	require.Equal(t, "fourth", appeals[0].Reason)
	require.Equal(t, "second", appeals[1].Reason)

	filtered, total, err := repo.ListAppeals(1, 1, 10, 0)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, filtered, 3)

	paged, total, err := repo.ListAppeals(1, 1, 1, 1)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, paged, 1)
	require.Equal(t, "third", paged[0].Reason)

	targetAppeals, err := repo.GetAppealsByTargetID(13, 1)
	require.NoError(t, err)
	require.Len(t, targetAppeals, 1)
	require.Equal(t, "third", targetAppeals[0].Reason)
}

func TestAppealRepositoryGetAppealByIDNotFound(t *testing.T) {
	db := newAppealTestDB(t)
	repo := NewAppealRepository(db)

	appeal, err := repo.GetAppealByID(999)
	require.ErrorIs(t, err, ErrNotFound)
	require.Nil(t, appeal)
}
