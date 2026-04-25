package database

import (
	"database/sql"
	"fmt"
	"strings"
)

type Driver string

const (
	DriverSQLite   Driver = "sqlite"
	DriverPostgres Driver = "postgres"
)

type DB interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Begin() (Tx, error)
	Ping() error
	Close() error
	Dialect() Driver
}

type Tx interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	Commit() error
	Rollback() error
	Dialect() Driver
}

type conn struct {
	db      *sql.DB
	dialect Driver
}

type txConn struct {
	tx      *sql.Tx
	dialect Driver
}

func newConn(db *sql.DB, dialect Driver) *conn {
	return &conn{db: db, dialect: dialect}
}

func (c *conn) Exec(query string, args ...interface{}) (sql.Result, error) {
	return c.db.Exec(rebind(query, c.dialect), args...)
}

func (c *conn) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return c.db.Query(rebind(query, c.dialect), args...)
}

func (c *conn) QueryRow(query string, args ...interface{}) *sql.Row {
	return c.db.QueryRow(rebind(query, c.dialect), args...)
}

func (c *conn) Begin() (Tx, error) {
	tx, err := c.db.Begin()
	if err != nil {
		return nil, err
	}
	return &txConn{tx: tx, dialect: c.dialect}, nil
}

func (c *conn) Ping() error     { return c.db.Ping() }
func (c *conn) Close() error    { return c.db.Close() }
func (c *conn) Dialect() Driver { return c.dialect }

func (t *txConn) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.Exec(rebind(query, t.dialect), args...)
}

func (t *txConn) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.Query(rebind(query, t.dialect), args...)
}

func (t *txConn) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRow(rebind(query, t.dialect), args...)
}

func (t *txConn) Commit() error   { return t.tx.Commit() }
func (t *txConn) Rollback() error { return t.tx.Rollback() }
func (t *txConn) Dialect() Driver { return t.dialect }

func rebind(query string, dialect Driver) string {
	if dialect != DriverPostgres {
		return query
	}

	var b strings.Builder
	b.Grow(len(query) + 8)
	idx := 1
	inSingle := false
	inDouble := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(query); i++ {
		ch := query[i]
		next := byte(0)
		if i+1 < len(query) {
			next = query[i+1]
		}

		switch {
		case inLineComment:
			b.WriteByte(ch)
			if ch == '\n' {
				inLineComment = false
			}
			continue
		case inBlockComment:
			b.WriteByte(ch)
			if ch == '*' && next == '/' {
				b.WriteByte(next)
				i++
				inBlockComment = false
			}
			continue
		case inSingle:
			b.WriteByte(ch)
			if ch == '\'' {
				if next == '\'' {
					b.WriteByte(next)
					i++
				} else {
					inSingle = false
				}
			}
			continue
		case inDouble:
			b.WriteByte(ch)
			if ch == '"' {
				inDouble = false
			}
			continue
		}

		if ch == '-' && next == '-' {
			b.WriteByte(ch)
			b.WriteByte(next)
			i++
			inLineComment = true
			continue
		}
		if ch == '/' && next == '*' {
			b.WriteByte(ch)
			b.WriteByte(next)
			i++
			inBlockComment = true
			continue
		}
		if ch == '\'' {
			inSingle = true
			b.WriteByte(ch)
			continue
		}
		if ch == '"' {
			inDouble = true
			b.WriteByte(ch)
			continue
		}
		if ch == '?' {
			b.WriteString(fmt.Sprintf("$%d", idx))
			idx++
			continue
		}
		b.WriteByte(ch)
	}

	return b.String()
}
