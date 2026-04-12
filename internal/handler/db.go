package handler

import (
	"database/sql"
	"sync"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/repository"
)

var (
	db          *sql.DB
	taskRepo    *repository.TaskRepository
	accountRepo *repository.AccountRepository
	once        sync.Once
	initErr     error
)

func initDB() error {
	once.Do(func() {
		cfg := config.Load()
		var err error
		db, err = database.InitDB(cfg.Database.Path)
		if err != nil {
			initErr = err
			return
		}
		taskRepo = repository.NewTaskRepository(db)
		accountRepo = repository.NewAccountRepository(db)
	})
	return initErr
}

func init() {
	if err := initDB(); err != nil {
		panic("failed to initialize database: " + err.Error())
	}
}

func GetTaskRepo() *repository.TaskRepository {
	return taskRepo
}

func GetAccountRepo() *repository.AccountRepository {
	return accountRepo
}

func GetDB() *sql.DB {
	return db
}
