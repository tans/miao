package handler

import (
	"log"
	"sync"

	"github.com/tans/miao/internal/config"
	"github.com/tans/miao/internal/database"
	"github.com/tans/miao/internal/repository"
)

var (
	db          database.DB
	taskRepo    *repository.TaskRepository
	accountRepo *repository.AccountRepository
	once        sync.Once
	initErr     error
)

func initDB() error {
	once.Do(func() {
		cfg := config.Load()
		var err error
		db, err = database.InitDB(cfg.Database)
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
		log.Fatalf("failed to initialize database: %v", err)
	}
}

func GetTaskRepo() *repository.TaskRepository {
	return taskRepo
}

func GetAccountRepo() *repository.AccountRepository {
	return accountRepo
}

func GetDB() database.DB {
	return db
}
