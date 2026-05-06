package database

import (
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
-- 澶氳鑹叉敮鎸侊細鎵€鏈夌敤鎴峰悓鏃舵槸鍟嗗鍜屽垱浣滆€?ALTER TABLE users ADD COLUMN level INTEGER DEFAULT 2;
ALTER TABLE users ADD COLUMN behavior_score INTEGER DEFAULT 100;
ALTER TABLE users ADD COLUMN trade_score REAL DEFAULT 0;
ALTER TABLE users ADD COLUMN total_score INTEGER DEFAULT 100;
ALTER TABLE users ADD COLUMN margin_frozen REAL DEFAULT 0;
ALTER TABLE users ADD COLUMN daily_claim_count INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN daily_claim_reset TIMESTAMP;
ALTER TABLE users ADD COLUMN business_verified INTEGER DEFAULT 0;
ALTER TABLE users ADD COLUMN publish_count INTEGER DEFAULT 0;
`,
	},
	{
		Version: 3,
		Name:    "remove_role_add_is_admin",
		SQL: `
-- 绉婚櫎 role 瀛楁锛堟墍鏈夌敤鎴烽兘鏄晢瀹?鍒涗綔鑰咃級锛屾坊鍔?is_admin
ALTER TABLE users ADD COLUMN is_admin INTEGER DEFAULT 0;
`,
	},
	{
		Version: 4,
		Name:    "task_v1_fields",
		SQL: `
-- v1.md 瑙勮寖锛氭坊鍔犱换鍔℃墿灞曞瓧娈?ALTER TABLE tasks ADD COLUMN industries TEXT DEFAULT '';
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
-- 娣诲姞寰俊 openid 瀛楁
ALTER TABLE users ADD COLUMN wechat_openid TEXT;
`,
	},
	{
		Version: 6,
		Name:    "performance_indexes",
		SQL: `
-- 鎬ц兘浼樺寲绱㈠紩锛氫换鍔℃煡璇㈠父瑙佹ā寮?CREATE INDEX IF NOT EXISTS idx_tasks_status_remaining ON tasks(status, remaining_count);
CREATE INDEX IF NOT EXISTS idx_claims_task_status ON claims(task_id, status);
CREATE INDEX IF NOT EXISTS idx_claims_creator_status ON claims(creator_id, status);
CREATE INDEX IF NOT EXISTS idx_transactions_user_created ON transactions(user_id, created_at);
`,
	},
	{
		Version: 7,
		Name:    "drop_deprecated_tables",
		SQL: `
DROP TABLE IF EXISTS submission_materials;
DROP TABLE IF EXISTS submissions;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS accounts;
`,
	},
	{
		Version: 8,
		Name:    "notifications_table",
		SQL: `
CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    type INTEGER NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    related_id INTEGER,
    is_read INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read ON notifications(is_read);
`,
	},
	{
		Version: 9,
		Name:    "claim_materials",
		SQL: `
CREATE TABLE IF NOT EXISTS claim_materials (
    id SERIAL PRIMARY KEY,
    claim_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER DEFAULT 0,
    file_type TEXT NOT NULL,
    thumbnail_path TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (claim_id) REFERENCES claims(id)
);

CREATE INDEX IF NOT EXISTS idx_claim_materials_claim_id ON claim_materials(claim_id);
`,
	},
	{
		Version: 10,
		Name:    "task_materials",
		SQL: `
CREATE TABLE IF NOT EXISTS task_materials (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER DEFAULT 0,
    file_type TEXT NOT NULL,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);

CREATE INDEX IF NOT EXISTS idx_task_materials_task_id ON task_materials(task_id);
`,
	},
	{
		Version: 11,
		Name:    "nickname_default",
		SQL: `
-- 璁剧疆鐢ㄦ埛鏄电О榛樿鍊煎柕鍠?UPDATE users SET nickname = '鍠靛柕' WHERE nickname IS NULL OR nickname = '';
`,
	},
	{
		Version: 12,
		Name:    "review_deadline_at",
		SQL: `
-- 娣诲姞瀹℃牳鎴鏃ユ湡瀛楁
ALTER TABLE tasks ADD COLUMN review_deadline_at TIMESTAMP DEFAULT NULL;
`,
	},
	{
		Version: 13,
		Name:    "inspirations",
		SQL: `
CREATE TABLE IF NOT EXISTS inspirations (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT DEFAULT '',
    creator_name TEXT DEFAULT '',
    creator_avatar TEXT DEFAULT '',
    cover_url TEXT DEFAULT '',
    cover_type TEXT DEFAULT 'image',
    status INTEGER DEFAULT 1,
    views INTEGER DEFAULT 0,
    likes INTEGER DEFAULT 0,
    sort_order INTEGER DEFAULT 0,
    created_by INTEGER NOT NULL,
    source_claim_id INTEGER,
    published_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS inspiration_materials (
    id SERIAL PRIMARY KEY,
    inspiration_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER DEFAULT 0,
    file_type TEXT NOT NULL,
    thumbnail_path TEXT DEFAULT '',
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (inspiration_id) REFERENCES inspirations(id)
);

CREATE INDEX IF NOT EXISTS idx_inspirations_status_sort ON inspirations(status, sort_order, published_at, created_at);
CREATE INDEX IF NOT EXISTS idx_inspirations_created_by ON inspirations(created_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_inspirations_source_claim_id ON inspirations(source_claim_id);
CREATE INDEX IF NOT EXISTS idx_inspiration_materials_inspiration_id ON inspiration_materials(inspiration_id);
`,
	},
	{
		Version: 14,
		Name:    "inspiration_likes",
		SQL: `
CREATE TABLE IF NOT EXISTS inspiration_likes (
    id SERIAL PRIMARY KEY,
    inspiration_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (inspiration_id) REFERENCES inspirations(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_inspiration_likes_unique ON inspiration_likes(inspiration_id, user_id);
CREATE INDEX IF NOT EXISTS idx_inspiration_likes_user_id ON inspiration_likes(user_id);
`,
	},
	{
		Version: 15,
		Name:    "inspiration_tags",
		SQL: `
ALTER TABLE inspirations ADD COLUMN tags TEXT DEFAULT '';
`,
	},
	{
		Version: 16,
		Name:    "inspiration_source_claim_id",
		SQL: `
ALTER TABLE inspirations ADD COLUMN source_claim_id INTEGER DEFAULT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_inspirations_source_claim_id ON inspirations(source_claim_id);
`,
	},
	{
		Version: 17,
		Name:    "creator_report_count",
		SQL: `
-- 创作者被举报次数字段（超过5次无法提交作品）
ALTER TABLE users ADD COLUMN report_count INTEGER DEFAULT 0;
`,
	},
	{
		Version: 18,
		Name:    "new_creator_level_system",
		SQL: `
-- 新版创作者积分体系：6级 Lv0-Lv5，基于累计采纳数
-- 将 level 默认值从 2 改为 0
-- 新增 adopted_count 字段记录累计采纳数
-- 删除 behavior_score, trade_score, total_score（不再使用）
ALTER TABLE users ADD COLUMN adopted_count INTEGER DEFAULT 0;
UPDATE users SET adopted_count = 0;
`,
	},
	{
		Version: 19,
		Name:    "work_likes",
		SQL: `
ALTER TABLE claims ADD COLUMN likes INTEGER DEFAULT 0;
CREATE TABLE IF NOT EXISTS work_likes (
    id SERIAL PRIMARY KEY,
    work_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (work_id) REFERENCES claims(id),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_work_likes_unique ON work_likes(work_id, user_id);
CREATE INDEX IF NOT EXISTS idx_work_likes_user_id ON work_likes(user_id);
`,
	},
	{
		Version: 20,
		Name:    "system_settings",
		SQL: `
CREATE TABLE IF NOT EXISTS system_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    review_days INTEGER DEFAULT 7,
    submit_days INTEGER DEFAULT 7,
    grace_days INTEGER DEFAULT 7,
    report_action INTEGER DEFAULT 1,
    min_unit_price REAL DEFAULT 2.0,
    min_award_price REAL DEFAULT 8.0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default settings if not exists
INSERT OR IGNORE INTO system_settings (id, review_days, submit_days, grace_days, report_action, min_unit_price, min_award_price)
VALUES (1, 7, 7, 7, 1, 2.0, 8.0);
`,
	},
	{
		Version: 21,
		Name:    "inspiration_cover_dimensions",
		SQL: `
-- Add cover image dimensions for accurate waterfall layout
ALTER TABLE inspirations ADD COLUMN cover_width INTEGER DEFAULT 0;
ALTER TABLE inspirations ADD COLUMN cover_height INTEGER DEFAULT 0;
`,
	},
	{
		Version: 22,
		Name:    "video_processing_jobs",
		SQL: `
ALTER TABLE claim_materials ADD COLUMN source_file_path TEXT DEFAULT '';
ALTER TABLE claim_materials ADD COLUMN processed_file_path TEXT DEFAULT '';
ALTER TABLE claim_materials ADD COLUMN process_status TEXT DEFAULT '';
ALTER TABLE claim_materials ADD COLUMN process_error TEXT DEFAULT '';
ALTER TABLE claim_materials ADD COLUMN process_job_id TEXT DEFAULT '';
ALTER TABLE claim_materials ADD COLUMN process_retry_count INTEGER DEFAULT 0;
ALTER TABLE claim_materials ADD COLUMN watermark_applied INTEGER DEFAULT 0;
ALTER TABLE claim_materials ADD COLUMN compressed INTEGER DEFAULT 0;
ALTER TABLE claim_materials ADD COLUMN duration REAL DEFAULT 0;
ALTER TABLE claim_materials ADD COLUMN width INTEGER DEFAULT 0;
ALTER TABLE claim_materials ADD COLUMN height INTEGER DEFAULT 0;
ALTER TABLE claim_materials ADD COLUMN updated_at TIMESTAMP;

UPDATE claim_materials
SET source_file_path = CASE WHEN source_file_path = '' THEN file_path ELSE source_file_path END
WHERE file_type = 'video';

CREATE TABLE IF NOT EXISTS video_processing_jobs (
    id SERIAL PRIMARY KEY,
    job_id TEXT NOT NULL UNIQUE,
    material_id INTEGER NOT NULL,
    biz_type TEXT NOT NULL,
    biz_id INTEGER NOT NULL,
    source_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempt INTEGER NOT NULL DEFAULT 1,
    processed_url TEXT DEFAULT '',
    thumbnail_url TEXT DEFAULT '',
    watermark_template TEXT DEFAULT '',
    target_format TEXT DEFAULT 'mp4',
    target_resolution TEXT DEFAULT '1080P',
    error_message TEXT DEFAULT '',
    duration REAL DEFAULT 0,
    width INTEGER DEFAULT 0,
    height INTEGER DEFAULT 0,
    watermark_applied INTEGER DEFAULT 0,
    compressed INTEGER DEFAULT 0,
    completed_at TIMESTAMP,
    last_callback_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (material_id) REFERENCES claim_materials(id)
);

CREATE INDEX IF NOT EXISTS idx_video_processing_jobs_material_id ON video_processing_jobs(material_id);
CREATE INDEX IF NOT EXISTS idx_video_processing_jobs_status ON video_processing_jobs(status);
`,
	},
	{
		Version: 23,
		Name:    "payment_orders",
		SQL: `
CREATE TABLE IF NOT EXISTS payment_orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    order_no TEXT UNIQUE NOT NULL,
    amount REAL NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    pay_result TEXT,
    wechat_order_id TEXT,
    paid_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_payment_orders_user_id ON payment_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_payment_orders_order_no ON payment_orders(order_no);
CREATE INDEX IF NOT EXISTS idx_payment_orders_status ON payment_orders(status);
`,
	},
	{
		Version: 24,
		Name:    "withdraw_orders",
		SQL: `
CREATE TABLE IF NOT EXISTS withdraw_orders (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    withdraw_no TEXT UNIQUE NOT NULL,
    idempotency_key TEXT,
    amount REAL NOT NULL,
    actual_amount REAL NOT NULL,
    commission_amount REAL NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    channel_txn_id TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE(user_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_withdraw_orders_user_id ON withdraw_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_withdraw_orders_withdraw_no ON withdraw_orders(withdraw_no);
CREATE INDEX IF NOT EXISTS idx_withdraw_orders_status ON withdraw_orders(status);
`,
	},
	{
		Version: 25,
		Name:    "task_public_service_fee",
		SQL: `
ALTER TABLE tasks ADD COLUMN public INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN service_fee_rate REAL DEFAULT 0.10;
ALTER TABLE tasks ADD COLUMN service_fee_amount REAL DEFAULT 0;
`,
	},
	{
		Version: 26,
		Name:    "remove_task_pending_status",
		SQL: `
UPDATE tasks
SET status = 2,
    publish_at = COALESCE(publish_at, review_at, created_at, updated_at, CURRENT_TIMESTAMP),
    updated_at = CURRENT_TIMESTAMP
WHERE status = 1;
`,
	},
	{
		Version: 27,
		Name:    "rename_task_visibility_column_to_public",
		SQL: `
ALTER TABLE tasks RENAME COLUMN open_submission TO public;
`,
	},
	{
		Version: 28,
		Name:    "ai_model_settings",
		SQL: `
ALTER TABLE system_settings ADD COLUMN ai_api_key TEXT DEFAULT '';
ALTER TABLE system_settings ADD COLUMN ai_api_endpoint TEXT DEFAULT '';
ALTER TABLE system_settings ADD COLUMN ai_model TEXT DEFAULT '';
`,
	},
	{
		Version: 29,
		Name:    "merchant_auth_applications",
		SQL: `
CREATE TABLE IF NOT EXISTS merchant_auth_applications (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL UNIQUE,
	company_name TEXT NOT NULL DEFAULT '',
	credit_code TEXT NOT NULL DEFAULT '',
	contact_name TEXT NOT NULL DEFAULT '',
	contact_phone TEXT NOT NULL DEFAULT '',
	license_url TEXT NOT NULL DEFAULT '',
	status INTEGER NOT NULL DEFAULT 0,
	review_comment TEXT DEFAULT '',
	reviewed_at TIMESTAMP,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_merchant_auth_applications_status ON merchant_auth_applications(status);
`,
	},
	{
		Version: 30,
		Name:    "ocr_settings",
		SQL: `
ALTER TABLE system_settings ADD COLUMN ocr_access_key_id TEXT DEFAULT '';
ALTER TABLE system_settings ADD COLUMN ocr_access_key_secret TEXT DEFAULT '';
ALTER TABLE system_settings ADD COLUMN ocr_endpoint TEXT DEFAULT '';
ALTER TABLE system_settings ADD COLUMN ocr_security_token TEXT DEFAULT '';
`,
	},
	{
		Version: 31,
		Name:    "merchant_auth_verified_backfill",
		SQL: `
-- 只有审核通过的商家认证申请才算已认证；历史默认值误设为 1，需要回填修正。
UPDATE users
SET business_verified = CASE
	WHEN EXISTS (
		SELECT 1
		FROM merchant_auth_applications maa
		WHERE maa.user_id = users.id AND maa.status = 1
	) THEN 1
	ELSE 0
END;
`,
	},
	{
		Version: 32,
		Name:    "merchant_auth_id_backfill",
		SQL: `
-- SQLite 里历史记录的 id 可能为空，用 rowid 回填，避免后续读取失败。
UPDATE merchant_auth_applications
SET id = rowid
WHERE id IS NULL OR id = 0;
`,
	},
	{
		Version: 33,
		Name:    "appeals_schema_compatibility",
		SQL: `
ALTER TABLE appeals ADD COLUMN evidence TEXT;
ALTER TABLE appeals ADD COLUMN admin_id INTEGER;
ALTER TABLE appeals ADD COLUMN handle_at TIMESTAMP;
`,
	},
	{
		Version: 34,
		Name:    "appeals_claim_id",
		SQL: `
ALTER TABLE appeals ADD COLUMN claim_id INTEGER DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_appeals_claim_id ON appeals(claim_id);
`,
	},
	{
		Version: 35,
		Name:    "video_processing_retry_fields",
		SQL: `
ALTER TABLE claim_materials ADD COLUMN process_job_id TEXT DEFAULT '';
ALTER TABLE claim_materials ADD COLUMN process_retry_count INTEGER DEFAULT 0;
ALTER TABLE claim_materials ADD COLUMN updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE video_processing_jobs ADD COLUMN attempt INTEGER DEFAULT 1;
`,
	},
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	username TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	is_admin INTEGER DEFAULT 0,
	email TEXT,
	phone TEXT,
	status INTEGER DEFAULT 1,
	nickname TEXT,
	avatar TEXT,
	real_name TEXT,
	real_name_verified INTEGER DEFAULT 0,
	company_name TEXT,
	balance REAL DEFAULT 0,
	frozen_amount REAL DEFAULT 0,
	wechat_openid TEXT,

	-- Creator fields (all users have these)
	level INTEGER DEFAULT 0,
	adopted_count INTEGER DEFAULT 0,
	margin_frozen REAL DEFAULT 0,
	daily_claim_count INTEGER DEFAULT 0,
	daily_claim_reset TIMESTAMP,
	report_count INTEGER DEFAULT 0,

	-- Business fields (all users have these)
	business_verified INTEGER DEFAULT 0,
	publish_count INTEGER DEFAULT 0,

	credit_score INTEGER DEFAULT 100,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users(is_admin);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_wechat_openid ON users(wechat_openid);

CREATE TABLE IF NOT EXISTS merchant_auth_applications (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL UNIQUE,
	company_name TEXT NOT NULL DEFAULT '',
	credit_code TEXT NOT NULL DEFAULT '',
	contact_name TEXT NOT NULL DEFAULT '',
	contact_phone TEXT NOT NULL DEFAULT '',
	license_url TEXT NOT NULL DEFAULT '',
	status INTEGER NOT NULL DEFAULT 0,
	review_comment TEXT DEFAULT '',
	reviewed_at TIMESTAMP,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_merchant_auth_applications_status ON merchant_auth_applications(status);

CREATE TABLE IF NOT EXISTS tasks (
	id SERIAL PRIMARY KEY,
	business_id INTEGER NOT NULL,
	title TEXT NOT NULL,
	description TEXT,
	category INTEGER NOT NULL,
	unit_price REAL NOT NULL,
	total_count INTEGER NOT NULL,
	remaining_count INTEGER NOT NULL,
	status INTEGER DEFAULT 1,
	review_at TIMESTAMP,
	publish_at TIMESTAMP,
	end_at TIMESTAMP,
	total_budget REAL NOT NULL,
	frozen_amount REAL DEFAULT 0,
	paid_amount REAL DEFAULT 0,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	industries TEXT DEFAULT '',
	video_duration TEXT DEFAULT '',
	video_aspect TEXT DEFAULT '',
	video_resolution TEXT DEFAULT '',
	creative_style TEXT DEFAULT '',
	award_price REAL DEFAULT 0,
	award_count INTEGER DEFAULT 0,
	public INTEGER DEFAULT 0,
	service_fee_rate REAL DEFAULT 0.10,
	service_fee_amount REAL DEFAULT 0,
	FOREIGN KEY (business_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS claims (
	id SERIAL PRIMARY KEY,
	task_id INTEGER NOT NULL,
	creator_id INTEGER NOT NULL,
	status INTEGER DEFAULT 1,
	content TEXT,
	submit_at TIMESTAMP,
	expires_at TIMESTAMP,
	review_at TIMESTAMP,
	review_result INTEGER,
	review_comment TEXT,
	creator_reward REAL DEFAULT 0,
	platform_fee REAL DEFAULT 0,
	margin_returned REAL DEFAULT 0,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (task_id) REFERENCES tasks(id),
	FOREIGN KEY (creator_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS transactions (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL,
	type INTEGER NOT NULL,
	amount REAL NOT NULL,
	balance_before REAL NOT NULL,
	balance_after REAL NOT NULL,
	remark TEXT,
	related_id INTEGER,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS credit_logs (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL,
	type INTEGER NOT NULL,
	change INTEGER NOT NULL,
	reason TEXT,
	related_id INTEGER,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS appeals (
	id SERIAL PRIMARY KEY,
	user_id INTEGER NOT NULL,
	type INTEGER NOT NULL,
	claim_id INTEGER,
	target_id INTEGER NOT NULL,
	reason TEXT NOT NULL,
	evidence TEXT,
	status INTEGER DEFAULT 1,
	result TEXT,
	admin_id INTEGER,
	handle_at TIMESTAMP,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (user_id) REFERENCES users(id)
);

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
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_category ON tasks(category);
CREATE INDEX IF NOT EXISTS idx_tasks_business_id ON tasks(business_id);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);
`

// RunAllMigrations runs all pending migrations
func RunAllMigrations(db DB) error {
	// Enable foreign keys only for SQLite
	if db.Dialect() == DriverSQLite {
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return fmt.Errorf("failed to enable foreign keys: %w", err)
		}
	}

	// Create migrations table if not exists
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
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
				// Errors that can be safely ignored
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
				} else if strings.Contains(errStr, "does not exist") {
					skip = true
				} else if strings.Contains(errStr, "duplicate") {
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
func GetDBVersion(db DB) (int, error) {
	var maxVersion int
	err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&maxVersion)
	if err != nil {
		// Table might not exist yet
		return 0, nil
	}
	return maxVersion, nil
}
