-- Migration: Remove role column and add is_admin column
-- Date: 2026-04-09

-- Step 1: Add is_admin column (default false)
ALTER TABLE users ADD COLUMN is_admin INTEGER DEFAULT 0;

-- Step 2: Migrate existing admin users
UPDATE users SET is_admin = 1 WHERE role = 'admin' OR role LIKE '%admin%';

-- Step 3: Create a temporary table with the new schema
CREATE TABLE users_new (
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

-- Step 4: Copy data from old table to new table
INSERT INTO users_new (
    id, username, password_hash, is_admin, email, phone, status,
    nickname, avatar, real_name, company_name, balance, frozen_amount,
    level, behavior_score, trade_score, total_score, margin_frozen,
    daily_claim_count, daily_claim_reset, business_verified, publish_count,
    credit_score, created_at, updated_at
)
SELECT
    id, username, password_hash, is_admin, email, phone, status,
    nickname, avatar, real_name, company_name, balance, frozen_amount,
    COALESCE(level, 2), COALESCE(behavior_score, 100), COALESCE(trade_score, 0),
    COALESCE(total_score, 100), COALESCE(margin_frozen, 0),
    COALESCE(daily_claim_count, 0), daily_claim_reset,
    COALESCE(business_verified, 1), COALESCE(publish_count, 0),
    COALESCE(credit_score, 100), created_at, updated_at
FROM users;

-- Step 5: Drop old table and rename new table
DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

-- Step 6: Recreate indexes
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users(is_admin);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(phone);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
