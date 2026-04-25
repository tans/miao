-- 迁移: 添加审核截止日期字段
-- 日期: 2026-04-16

-- 为 tasks 表添加审核截止日期字段
ALTER TABLE tasks ADD COLUMN review_deadline_at TIMESTAMP DEFAULT NULL;