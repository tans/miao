-- 迁移: 添加即梦合拍字段
-- 日期: 2026-04-17
-- 描述: 为 tasks 表添加即梦合拍链接和验证码字段

ALTER TABLE tasks ADD COLUMN jimeng_link TEXT DEFAULT '';
ALTER TABLE tasks ADD COLUMN jimeng_code TEXT DEFAULT '';