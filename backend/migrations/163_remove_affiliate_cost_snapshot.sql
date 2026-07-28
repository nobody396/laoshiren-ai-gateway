-- The stress cost is a manually configured conservative input, not an
-- automatically refreshed market snapshot. Remove the artificial expiry
-- fields and keep the real margin gate on every settings update/card issue.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

ALTER TABLE affiliate_program_settings
    DROP CONSTRAINT IF EXISTS chk_affiliate_cost_snapshot_age,
    DROP COLUMN IF EXISTS stress_cost_snapshot_at,
    DROP COLUMN IF EXISTS cost_snapshot_max_age_hours;

-- Also repair databases that already applied the original V3 migration before
-- its singleton default was aligned with the V3 check constraint.
ALTER TABLE affiliate_program_settings
    ALTER COLUMN program_version SET DEFAULT 'v3';
