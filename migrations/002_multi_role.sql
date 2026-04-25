-- 支持多角色功能的数据库迁移

-- 1. 修改 role 字段的约束，允许多个角色（逗号分隔）
-- SQLite 不支持直接修改 CHECK 约束，需要重建表

-- 创建临时表（完整的25列）
CREATE TABLE users_new (
    id SERIAL PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL, -- 移除 CHECK 约束，允许 "creator,business" 格式
    email TEXT,
    phone TEXT,
    status INTEGER DEFAULT 1,
    nickname TEXT,
    avatar TEXT,
    real_name TEXT,
    company_name TEXT,
    balance REAL DEFAULT 0,
    frozen_amount REAL DEFAULT 0,
    credit_score INTEGER DEFAULT 100,
    level INTEGER DEFAULT 1,
    behavior_score INTEGER DEFAULT 0,
    trade_score REAL DEFAULT 0,
    total_score INTEGER DEFAULT 0,
    margin_frozen REAL DEFAULT 0,
    daily_claim_count INTEGER DEFAULT 0,
    daily_claim_reset TIMESTAMP,
    business_verified INTEGER DEFAULT 0,
    publish_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 复制数据（所有25列）
INSERT INTO users_new SELECT
    id, username, password_hash, role, email, phone, status, nickname, avatar,
    real_name, company_name, balance, frozen_amount, credit_score, level,
    behavior_score, trade_score, total_score, margin_frozen, daily_claim_count,
    daily_claim_reset, business_verified, publish_count, created_at, updated_at
FROM users;

-- 删除旧表
DROP TABLE users;

-- 重命名新表
ALTER TABLE users_new RENAME TO users;

-- 重建索引
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

-- 更新现有用户，给所有用户添加多角色支持
-- 将单一角色转换为多角色格式（creator -> creator,business）
UPDATE users SET role = 'creator,business' WHERE role = 'creator';
UPDATE users SET role = 'creator,business' WHERE role = 'business';
-- 管理员保持不变
-- UPDATE users SET role = 'admin' WHERE role = 'admin';
