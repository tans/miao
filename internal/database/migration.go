package database

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
)

// Migration represents a single database migration
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// migrations holds all database migrations in order
var migrations = []Migration{
	{
		Version: 1,
		Name:    "initial_schema",
		SQL:     schemaSQL,
	},
	{
		Version: 2,
		Name:    "multi_role",
		SQL: `
-- 多角色支持：所有用户同时是商家和创作者
ALTER TABLE users ADD COLUMN level INTEGER DEFAULT 2;
ALTER TABLE users ADD COLUMN behavior_score INTEGER DEFAULT 100;
ALTER TABLE users ADD COLUMN trade_score REAL DEFAULT 0;
ALTER TABLE users ADD COLUMN total_score INTEGER DEFAULT 100;
ALTER TABLE users ADD COLUMN margin_frozen REAL DEFAULT 0;
ALTER TABLE users ADD COLUMN daily_claim_count INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN daily_claim_reset DATETIME;
ALTER TABLE users ADD COLUMN business_verified INTEGER DEFAULT 1;
ALTER TABLE users ADD COLUMN publish_count INTEGER DEFAULT 0;
`,
	},
	{
		Version: 3,
		Name:    "remove_role_add_is_admin",
		SQL: `
-- 移除 role 字段（所有用户都是商家+创作者），添加 is_admin
ALTER TABLE users ADD COLUMN is_admin INTEGER DEFAULT 0;
`,
	},
	{
		Version: 4,
		Name:    "task_v1_fields",
		SQL: `
-- v1.md 规范：添加任务扩展字段
ALTER TABLE tasks ADD COLUMN industries TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN video_duration TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN video_aspect TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN video_resolution TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN creative_style TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN award_price REAL DEFAULT 0;
ALTER TABLE tasks ADD COLUMN award_count INTEGER DEFAULT 0;
`,
	},
	{
		Version: 5,
		Name:    "wechat_openid",
		SQL: `
-- 添加微信 openid 字段
ALTER TABLE users ADD COLUMN wechat_openid TEXT;
`,
	},
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	is_admin INTEGER DEFAULT 0,
	email TEXT,
	phone TEXT,
	status INTEGER DEFAULT 1,
	nickname TEXT,
	avatar TEXT,
	real_name TEXT,
	company_name TEXT,
	balance REAL DEFAULT 0,
	frozen_amount REAL DEFAULT 0,
	wechat_openid TEXT,

	-- Creator fields (all users have these)
	level INTEGER DEFAULT 2,
	behavior_score INTEGER DEFAULT 100,
	trade_score REAL DEFAULT 0,
	total_score INTEGER DEFAULT 100,
	margin_frozen REAL DEFAULT 0,
	daily_claim_count INTEGER DEFAULT 0,
	daily_claim_reset DATETIME,

	-- Business fields (all users have these)
	business_verified INTEGER DEFAULT 1,
	publish_count INTEGER DEFAULT 0,

	credit_score INTEGER DEFAULT 100,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users(is_admin);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_wechat_openid ON users(wechat_openid);

CREATE TABLE IF NOT EXISTS tasks (
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
	total_budget REAL NOT NULL,
	frozen_amount REAL DEFAULT 0,
	paid_amount REAL DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	industries TEXT DEFAULT '',
	video_duration TEXT DEFAULT '',
	video_aspect TEXT DEFAULT '',
	video_resolution TEXT DEFAULT '',
	creative_style TEXT DEFAULT '',
	award_price REAL DEFAULT 0,
	award_count INTEGER DEFAULT 0,
	FOREIGN KEY (business_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS claims (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	task_id INTEGER NOT NULL,
	creator_id INTEGER NOT NULL,
	status INTEGER DEFAULT 1,
	content TEXT,
	submit_at DATETIME,
	expires_at DATETIME,
	review_at DATETIME,
	review_result INTEGER,
	review_comment TEXT,
	creator_reward REAL DEFAULT 0,
	platform_fee REAL DEFAULT 0,
	margin_returned REAL DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (task_id) REFERENCES tasks(id),
	FOREIGN KEY (creator_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS transactions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	type INTEGER NOT NULL,
	amount REAL NOT NULL,
	balance_before REAL NOT NULL,
	balance_after REAL NOT NULL,
	remark TEXT,
	related_id INTEGER,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS credit_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	type INTEGER NOT NULL,
	change INTEGER NOT NULL,
	reason TEXT,
	related_id INTEGER,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS appeals (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	type INTEGER NOT NULL,
	target_id INTEGER NOT NULL,
	reason TEXT NOT NULL,
	status INTEGER DEFAULT 1,
	result TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL,
	type INTEGER NOT NULL,
	title TEXT NOT NULL,
	content TEXT NOT NULL,
	related_id INTEGER,
	is_read INTEGER DEFAULT 0,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS submissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    creator_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    status INTEGER DEFAULT 1,
    award_level INTEGER DEFAULT 0,
    score INTEGER,
    review_comment TEXT,
    reward_amount REAL DEFAULT 0,
    is_used INTEGER DEFAULT 0,
    is_top INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    reviewed_at DATETIME,
    FOREIGN KEY (task_id) REFERENCES tasks(id),
    FOREIGN KEY (creator_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS submission_materials (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    submission_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER,
    file_type TEXT,
    thumbnail_path TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (submission_id) REFERENCES submissions(id)
);

CREATE INDEX IF NOT EXISTS idx_submissions_task_id ON submissions(task_id);
CREATE INDEX IF NOT EXISTS idx_submissions_creator_id ON submissions(creator_id);
CREATE INDEX IF NOT EXISTS idx_submissions_status ON submissions(status);

CREATE INDEX IF NOT EXISTS idx_claims_task_id ON claims(task_id);
CREATE INDEX IF NOT EXISTS idx_claims_creator_id ON claims(creator_id);
CREATE INDEX IF NOT EXISTS idx_claims_status ON claims(status);
CREATE INDEX IF NOT EXISTS idx_claims_expires_at ON claims(expires_at);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(type);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at);
CREATE INDEX IF NOT EXISTS idx_credit_logs_user_id ON credit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_appeals_user_id ON appeals(user_id);
CREATE INDEX IF NOT EXISTS idx_appeals_status ON appeals(status);
CREATE INDEX IF NOT EXISTS idx_messages_user_id ON messages(user_id);
CREATE INDEX IF NOT EXISTS idx_messages_is_read ON messages(is_read);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_category ON tasks(category);
CREATE INDEX IF NOT EXISTS idx_tasks_business_id ON tasks(business_id);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);
`

// RunAllMigrations runs all pending migrations
func RunAllMigrations(db *sql.DB) error {
	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create migrations table if not exists
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get already applied migrations
	applied := make(map[int]bool)
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("failed to query migrations: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("failed to scan migration version: %w", err)
		}
		applied[v] = true
	}
	rows.Close()

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	// Run pending migrations
	for _, m := range migrations {
		if applied[m.Version] {
			log.Printf("Migration %d (%s) already applied, skipping", m.Version, m.Name)
			continue
		}

		log.Printf("Running migration %d (%s)...", m.Version, m.Name)

		// Split SQL into individual statements and execute each
		statements := splitSQLStatements(m.SQL)
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("migration %d: failed to begin transaction: %w", m.Version, err)
		}

		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			_, err := tx.Exec(stmt)
			if err != nil {
				// SQLite errors that can be safely ignored
				// - "duplicate column name": column already exists (ALTER TABLE ADD COLUMN)
				// - "no such table": table doesn't exist yet (CREATE INDEX on non-existent table)
				// - "no such column": table exists but column not present yet
				// - "table ... already exists": CREATE TABLE IF NOT EXISTS
				errStr := err.Error()
				skip := false
				if strings.Contains(errStr, "duplicate column name") {
					skip = true
				} else if strings.Contains(errStr, "no such table") {
					skip = true
				} else if strings.Contains(errStr, "no such column") {
					skip = true
				} else if strings.Contains(errStr, "already exists") {
					skip = true
				}
				if skip {
					log.Printf("  Skipping statement (already applied): %s", truncateString(stmt, 50))
					continue
				}
				tx.Rollback()
				return fmt.Errorf("migration %d: failed to execute: %s\nError: %w", m.Version, stmt, err)
			}
		}

		// Record migration
		_, err = tx.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.Version, m.Name)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: failed to record migration: %w", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d: failed to commit: %w", m.Version, err)
		}

		log.Printf("Migration %d (%s) applied successfully", m.Version, m.Name)
	}

	return nil
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// splitSQLStatements splits SQL into individual statements
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(sql); i++ {
		c := sql[i]

		if inString {
			current.WriteByte(c)
			if c == stringChar && (i == 0 || sql[i-1] != '\\') {
				inString = false
			}
			continue
		}

		if c == '\'' || c == '"' {
			inString = true
			stringChar = c
			current.WriteByte(c)
		} else if c == ';' {
			stmt := current.String()
			if strings.TrimSpace(stmt) != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		} else {
			current.WriteByte(c)
		}
	}

	// Don't forget the last statement
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// GetDBVersion returns the current database version
func GetDBVersion(db *sql.DB) (int, error) {
	var maxVersion int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&maxVersion)
	if err != nil {
		// Table might not exist yet
		return 0, nil
	}
	return maxVersion, nil
}
