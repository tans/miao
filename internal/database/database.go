package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/tans/miao/internal/config"
)

func InitDB(cfg config.DatabaseConfig) (DB, error) {
	driver := cfg.Driver
	if driver == "" {
		driver = string(DriverPostgres)
	}

	var (
		db  *sql.DB
		err error
	)
	switch Driver(driver) {
	case DriverPostgres:
		if cfg.DSN == "" {
			return nil, fmt.Errorf("postgres dsn is empty")
		}
		db, err = sql.Open("pgx", cfg.DSN)
	case DriverSQLite:
		// Ensure the directory exists
		dir := filepath.Dir(cfg.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
		db, err = sql.Open("sqlite3", cfg.Path)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return newConn(db, Driver(driver)), nil
}

// RunMigrations executes the schema SQL
func RunMigrations(db DB, schema string) error {
	// Enable foreign keys on SQLite only
	if db.Dialect() == DriverSQLite {
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return fmt.Errorf("failed to enable foreign keys: %w", err)
		}
	}

	// Execute schema in a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
