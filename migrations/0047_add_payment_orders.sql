-- 支付订单表
CREATE TABLE IF NOT EXISTS payment_orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    order_no TEXT UNIQUE NOT NULL,
    amount REAL NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    pay_result TEXT,
    wechat_order_id TEXT,
    paid_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_user_id ON payment_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_order_no ON payment_orders(order_no);
CREATE INDEX IF NOT EXISTS idx_status ON payment_orders(status);
