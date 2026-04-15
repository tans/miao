-- Cleanup cancelled claims and their materials
-- This removes claims with status=4 (cancelled) and their associated materials

-- First delete orphaned claim_materials (materials whose claim no longer exists)
DELETE FROM claim_materials WHERE claim_id NOT IN (SELECT id FROM claims);

-- Delete cancelled claims (status=4)
DELETE FROM claims WHERE status = 4;