-- Migration schema for 创意喵平台

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
    company_name TEXT,
    balance REAL DEFAULT 0,
    frozen_amount REAL DEFAULT 0,

    -- Creator fields (all users have these)
    level INTEGER DEFAULT 0,
    adopted_count INTEGER DEFAULT 0,
    margin_frozen REAL DEFAULT 0,
    daily_claim_count INTEGER DEFAULT 0,
    daily_claim_reset TIMESTAMP,
    report_count INTEGER DEFAULT 0,

    -- Business fields (all users have these)
    business_verified INTEGER DEFAULT 1,
    publish_count INTEGER DEFAULT 0,

    credit_score INTEGER DEFAULT 100,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tasks (
    id SERIAL PRIMARY KEY,
    business_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    category INTEGER NOT NULL,
    status INTEGER DEFAULT 1,
    total_budget REAL NOT NULL,
    base_reward REAL DEFAULT 0,
    base_reward_limit INTEGER DEFAULT 0,
    award1_amount REAL DEFAULT 0,
    award1_count INTEGER DEFAULT 0,
    award2_amount REAL DEFAULT 0,
    award2_count INTEGER DEFAULT 0,
    award3_amount REAL DEFAULT 0,
    award3_count INTEGER DEFAULT 0,
    award_good_amount REAL DEFAULT 0,
    award_good_count INTEGER DEFAULT 0,
    max_per_user INTEGER DEFAULT 1,
    is_public INTEGER DEFAULT 1,
    allow_duplicate INTEGER DEFAULT 0,
    enable_check INTEGER DEFAULT 0,
    public INTEGER DEFAULT 0,
    service_fee_rate REAL DEFAULT 0.10,
    service_fee_amount REAL DEFAULT 0,
    deadline TIMESTAMP NOT NULL,
    submissions INTEGER DEFAULT 0,
    reviewed_count INTEGER DEFAULT 0,
    passed_count INTEGER DEFAULT 0,
    total_rewarded REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    FOREIGN KEY (business_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS submissions (
    id SERIAL PRIMARY KEY,
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
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    reviewed_at TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id),
    FOREIGN KEY (creator_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS submission_materials (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER,
    file_type TEXT,
    thumbnail_path TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (submission_id) REFERENCES submissions(id)
);

CREATE TABLE IF NOT EXISTS task_materials (
    id SERIAL PRIMARY KEY,
    task_id INTEGER NOT NULL,
    file_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER,
    file_type TEXT,
    is_key INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);

CREATE TABLE IF NOT EXISTS accounts (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL UNIQUE,
    balance REAL DEFAULT 0,
    frozen_amount REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    account_id INTEGER NOT NULL,
    type INTEGER NOT NULL,
    amount REAL NOT NULL,
    balance_before REAL NOT NULL,
    balance_after REAL NOT NULL,
    remark TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (account_id) REFERENCES accounts(id)
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_tasks_business_id ON tasks(business_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_deadline ON tasks(deadline);
CREATE INDEX IF NOT EXISTS idx_submissions_task_id ON submissions(task_id);
CREATE INDEX IF NOT EXISTS idx_submissions_creator_id ON submissions(creator_id);
CREATE INDEX IF NOT EXISTS idx_submissions_status ON submissions(status);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_is_admin ON users(is_admin);
CREATE INDEX IF NOT EXISTS idx_accounts_user_id ON accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_account_id ON transactions(account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_created_at ON transactions(created_at);
