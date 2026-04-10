-- Add wechat_openid column for Wechat Mini Program login
ALTER TABLE users ADD COLUMN wechat_openid TEXT;
CREATE INDEX IF NOT EXISTS idx_users_wechat_openid ON users(wechat_openid);
