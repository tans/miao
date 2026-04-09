-- 迁移: 添加 v1.md 规范的任务字段
-- 日期: 2026-04-10

-- 为 tasks 表添加 v1.md 规范的新字段
ALTER TABLE tasks ADD COLUMN industries TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN video_duration TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN video_aspect TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN video_resolution TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN creative_style TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN award_price REAL DEFAULT 0;
ALTER TABLE tasks ADD COLUMN award_count INTEGER DEFAULT 0;
