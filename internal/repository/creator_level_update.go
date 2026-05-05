package repository

import (
	"database/sql"
	"time"

	"github.com/tans/miao/internal/model"
)

type sqlExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func updateCreatorAdoptedCountAndLevel(exec sqlExecer, userID int64, adoptedCount int) error {
	level := model.CalculateCreatorLevel(adoptedCount)
	query := `
		UPDATE users
		SET adopted_count = ?, level = ?, updated_at = ?
		WHERE id = ?
	`
	_, err := exec.Exec(query, adoptedCount, level, time.Now(), userID)
	return err
}

func refreshCreatorLevelFromAdoptedCount(exec sqlExecer, userID int64) error {
	var adoptedCount int
	if err := exec.QueryRow(`SELECT adopted_count FROM users WHERE id = ?`, userID).Scan(&adoptedCount); err != nil {
		return err
	}
	return updateCreatorAdoptedCountAndLevel(exec, userID, adoptedCount)
}
