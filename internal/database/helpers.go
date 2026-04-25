package database

import (
	"database/sql"
	"fmt"
	"strings"
)

type inserter interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Dialect() Driver
}

// InsertReturningID inserts a row and returns the generated id.
// PostgreSQL uses RETURNING id, while SQLite relies on LastInsertId.
func InsertReturningID(conn inserter, query string, args ...interface{}) (int64, error) {
	if conn.Dialect() == DriverPostgres {
		if !strings.Contains(strings.ToUpper(query), "RETURNING") {
			query += " RETURNING id"
		}
		var id int64
		if err := conn.QueryRow(query, args...).Scan(&id); err != nil {
			return 0, err
		}
		return id, nil
	}

	result, err := conn.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert id: %w", err)
	}
	return id, nil
}
