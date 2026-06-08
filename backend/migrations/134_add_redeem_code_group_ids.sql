ALTER TABLE redeem_codes
ADD COLUMN IF NOT EXISTS group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

COMMENT ON COLUMN redeem_codes.group_ids IS 'Subscription bundle group ids. Empty array preserves legacy group_id behavior.';

-- Backfill existing single-group subscription codes for easier inspection while
-- keeping redemption logic backward compatible with group_id.
UPDATE redeem_codes
SET group_ids = jsonb_build_array(group_id)
WHERE type = 'subscription'
  AND group_id IS NOT NULL
  AND (group_ids IS NULL OR group_ids = '[]'::jsonb);
