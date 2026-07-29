-- Increase only the current Plus/Pro/Max catalog quotas.
--
-- Historical Lite/Pro/Max/Ultra/Apex groups are deliberately excluded so
-- existing customers retain the entitlement they originally purchased.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

WITH target_groups(name, monthly_limit) AS (
    VALUES
        ('GPT Plus 月卡组'::TEXT, 300::NUMERIC),
        ('Claude Plus 月卡组'::TEXT, 300::NUMERIC),
        ('GPT Pro V3 月卡组'::TEXT, 900::NUMERIC),
        ('Claude Pro V3 月卡组'::TEXT, 900::NUMERIC),
        ('GPT Max V3 月卡组'::TEXT, 2000::NUMERIC),
        ('Claude Max V3 月卡组'::TEXT, 2000::NUMERIC)
),
updated_groups AS (
    UPDATE groups AS g
    SET monthly_limit_usd = target_groups.monthly_limit,
        updated_at = NOW()
    FROM target_groups
    WHERE g.deleted_at IS NULL
      AND g.name = target_groups.name
      AND g.monthly_limit_usd IS DISTINCT FROM target_groups.monthly_limit
    RETURNING g.id, g.name, g.monthly_limit_usd
)
INSERT INTO scheduler_outbox (event_type, group_id, payload)
SELECT
    'group_changed',
    id,
    jsonb_build_object(
        'reason', 'monthly_catalog_quota_increase',
        'name', name,
        'monthly_limit_usd', monthly_limit_usd,
        'pricing_table_version', 'v3-2026-07-28'
    )
FROM updated_groups;

-- The agreed 3:7 GPT account-cost mix is:
-- (0.15 * 30% + 0.20 * 70%) / 0.50 group multiplier = ¥0.37 per raw credit.
-- Only replace the previous V3 default/value; preserve any independently
-- reviewed operator override.
ALTER TABLE affiliate_program_settings
    ALTER COLUMN stress_cost_per_raw_credit_micros SET DEFAULT 370000;

UPDATE affiliate_program_settings
SET stress_cost_per_raw_credit_micros = 370000,
    revision = revision + 1,
    updated_at = NOW()
WHERE id = 1
  AND stress_cost_per_raw_credit_micros = 530000;
