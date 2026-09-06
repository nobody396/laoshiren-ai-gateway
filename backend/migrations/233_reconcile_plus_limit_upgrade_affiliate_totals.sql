-- Preserve monotonic affiliate confirmation across the 2026-09-01 Plus
-- credit-limit upgrade (300 -> 380). The cutover raised credit_limit_micros
-- without rebasing confirmed_consumption_micros. The next usage therefore
-- lowered the cycle's confirmation and later re-confirmed the same value.
--
-- Existing customer rebates and partner cash are deliberately preserved as a
-- platform-funded goodwill decision. This migration only repairs performance
-- and qualification totals, with an explicit non-financial reversal event.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

WITH params AS (
    SELECT '2026-08-31 16:00:00+00'::timestamptz AS cutoff
), upgraded_cycles AS (
    SELECT DISTINCT c.id, c.user_id, c.sale_price_micros,
           c.confirmed_consumption_micros, c.starts_at, c.ends_at
    FROM monthly_entitlement_cycles c
    JOIN monthly_entitlement_cycle_subscriptions link ON link.cycle_id = c.id
    CROSS JOIN params p
    WHERE EXISTS (
        SELECT 1 FROM monthly_commercial_cutovers cutover
        WHERE cutover.effective_at = p.cutoff
    )
      AND link.group_id IN (40, 41, 48)
      AND c.source_type IN ('paid_redeem', 'paid_topup')
      AND c.credit_limit_micros = 380000000
      AND c.starts_at < p.cutoff
      AND c.ends_at > p.cutoff
), pre_cutover AS (
    SELECT c.id,
           COALESCE(SUM(CASE e.event_type
               WHEN 'confirmed_consumption' THEN e.amount_micros
               ELSE -e.amount_micros
           END), 0)::bigint AS confirmed_micros
    FROM upgraded_cycles c
    CROSS JOIN params p
    LEFT JOIN affiliate_performance_events e
      ON e.user_id = c.user_id
     AND e.source_type IN ('monthly_usage', 'performance_reversal')
     AND e.event_type IN ('confirmed_consumption', 'consumption_reversal')
     AND e.occurred_at >= c.starts_at
     AND e.occurred_at < LEAST(c.ends_at, p.cutoff)
    GROUP BY c.id
), expected AS (
    SELECT c.id,
           LEAST(
               c.sale_price_micros,
               GREATEST(c.confirmed_consumption_micros, p.confirmed_micros)
           )::bigint AS confirmed_micros
    FROM upgraded_cycles c
    JOIN pre_cutover p ON p.id = c.id
)
UPDATE monthly_entitlement_cycles c
SET confirmed_consumption_micros = expected.confirmed_micros,
    updated_at = NOW()
FROM expected
WHERE c.id = expected.id
  AND c.confirmed_consumption_micros < expected.confirmed_micros;

WITH params AS (
    SELECT '2026-08-31 16:00:00+00'::timestamptz AS cutoff
), upgraded_cycles AS (
    SELECT DISTINCT c.id, c.user_id, c.direct_partner_id,
           c.confirmed_consumption_micros, c.starts_at, c.ends_at
    FROM monthly_entitlement_cycles c
    JOIN monthly_entitlement_cycle_subscriptions link ON link.cycle_id = c.id
    CROSS JOIN params p
    WHERE EXISTS (
        SELECT 1 FROM monthly_commercial_cutovers cutover
        WHERE cutover.effective_at = p.cutoff
    )
      AND link.group_id IN (40, 41, 48)
      AND c.source_type IN ('paid_redeem', 'paid_topup')
      AND c.credit_limit_micros = 380000000
      AND c.starts_at < p.cutoff
      AND c.ends_at > p.cutoff
), event_totals AS (
    SELECT c.id,
           COALESCE(SUM(CASE e.event_type
               WHEN 'confirmed_consumption' THEN e.amount_micros
               ELSE -e.amount_micros
           END), 0)::bigint AS net_micros,
           MAX(e.occurred_at) AS last_event_at
    FROM upgraded_cycles c
    LEFT JOIN affiliate_performance_events e
      ON e.user_id = c.user_id
     AND e.source_type IN ('monthly_usage', 'performance_reversal')
     AND e.event_type IN ('confirmed_consumption', 'consumption_reversal')
     AND e.occurred_at >= c.starts_at
     AND e.occurred_at < c.ends_at
    GROUP BY c.id
), corrections AS (
    SELECT c.id, c.user_id, c.direct_partner_id,
           GREATEST(e.net_micros - c.confirmed_consumption_micros, 0)::bigint AS excess_micros,
           COALESCE(e.last_event_at, p.cutoff) AS occurred_at
    FROM upgraded_cycles c
    JOIN event_totals e ON e.id = c.id
    CROSS JOIN params p
)
INSERT INTO affiliate_performance_events (
    user_id, direct_agent_id, event_type, amount_micros,
    source_type, source_id, event_key, occurred_at, metadata
)
SELECT c.user_id, c.direct_partner_id, 'consumption_reversal', c.excess_micros,
       'performance_reversal', c.id,
       'reconcile:plus-limit-upgrade-20260901:cycle:' || c.id::text,
       c.occurred_at,
       jsonb_build_object(
           'program_mode', 'live',
           'reason', 'plus_limit_upgrade_confirmation_rebase',
           'financial_settlement_preserved', true,
           'cutover_at', (SELECT cutoff FROM params)
       )
FROM corrections c
WHERE c.excess_micros > 0
ON CONFLICT (event_key) DO NOTHING;

RESET statement_timeout;
RESET lock_timeout;
