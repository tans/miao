-- Rename columns for cleaner API field names
ALTER TABLE tasks RENAME COLUMN creative_style TO styles;
ALTER TABLE tasks RENAME COLUMN open_submission TO public;
