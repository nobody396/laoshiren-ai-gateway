-- Update monthly-card group multipliers for the shared credits product.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

WITH monthly_card_targets AS (
    SELECT
        id,
        name,
        platform,
        CASE
            WHEN platform = 'openai' THEN 0.4::decimal(10, 4)
            WHEN platform = 'anthropic' THEN 1.25::decimal(10, 4)
        END AS target_rate_multiplier
    FROM groups
    WHERE deleted_at IS NULL
      AND subscription_type = 'credit'
      AND (
          (
              platform = 'openai'
              AND name IN (
                  'GPT Lite 月卡组',
                  'GPT Pro 月卡组',
                  'GPT Max 月卡组',
                  'GPT Ultra 月卡组'
              )
          )
          OR (
              platform = 'anthropic'
              AND name IN (
                  'Claude Lite 月卡组',
                  'Claude Pro 月卡组',
                  'Claude Max 月卡组',
                  'Claude Ultra 月卡组'
              )
          )
      )
),
updated_groups AS (
    UPDATE groups AS g
    SET
        rate_multiplier = t.target_rate_multiplier,
        updated_at = NOW()
    FROM monthly_card_targets AS t
    WHERE g.id = t.id
      AND g.rate_multiplier IS DISTINCT FROM t.target_rate_multiplier
    RETURNING g.id, g.name, g.platform, g.rate_multiplier
)
INSERT INTO scheduler_outbox (event_type, group_id, payload)
SELECT
    'group_changed',
    id,
    jsonb_build_object(
        'reason', 'monthly_card_rate_multiplier_update',
        'name', name,
        'platform', platform,
        'rate_multiplier', rate_multiplier
    )
FROM updated_groups;
