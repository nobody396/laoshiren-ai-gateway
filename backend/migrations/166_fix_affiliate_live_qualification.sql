-- Affiliate V3 live-readiness qualification fixes.
--
-- Qualification is revenue-equivalent confirmed consumption:
--   * balance products count only consumed paid balance;
--   * monthly products count sale_price * used_credits / credit_limit,
--     capped at the actual sale price;
--   * one internal billing unit equals one CNY unit. No FX conversion applies.
--
-- Legacy history is deliberately conservative. The cutover baseline is capped
-- by both known paid acquisition value and observed billed usage. Missing
-- external purchase evidence may be added later as a separate audited entry;
-- raw model list cost must never be treated as CNY consumption.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS affiliate_qualification_baseline_entries (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source_key VARCHAR(180) NOT NULL UNIQUE,
    source_type VARCHAR(32) NOT NULL,
    confirmed_consumption_micros BIGINT NOT NULL,
    cutoff_at TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_qualification_baseline_source_type CHECK (
        source_type IN ('legacy_auto', 'manual_verified')
    ),
    CONSTRAINT chk_affiliate_qualification_baseline_amount CHECK (
        confirmed_consumption_micros > 0
    ),
    CONSTRAINT chk_affiliate_qualification_baseline_metadata CHECK (
        jsonb_typeof(metadata) = 'object'
    )
);

CREATE INDEX IF NOT EXISTS idx_affiliate_qualification_baseline_user
    ON affiliate_qualification_baseline_entries (user_id, cutoff_at, id);

COMMENT ON TABLE affiliate_qualification_baseline_entries IS
    'Qualification-only historical confirmed consumption; never a reward or commission source';
COMMENT ON COLUMN affiliate_qualification_baseline_entries.confirmed_consumption_micros IS
    'Revenue-equivalent consumption in internal 1-unit-equals-1-CNY micros; never converted by FX';

-- Preserve every durable legacy inviter relationship in the V3 binding ledger.
-- Existing V3 bindings are immutable and always win.
INSERT INTO affiliate_bindings (
    customer_user_id,
    inviter_user_id,
    binding_kind,
    customer_rebate_rate_snapshot_bps,
    agent_commission_rate_snapshot_bps,
    bound_at,
    updated_at
)
SELECT
    customer.id,
    customer.inviter_id,
    'ordinary',
    0,
    0,
    customer.created_at,
    NOW()
FROM users customer
JOIN users inviter
  ON inviter.id = customer.inviter_id
 AND inviter.deleted_at IS NULL
WHERE customer.deleted_at IS NULL
  AND customer.inviter_id IS NOT NULL
  AND customer.id <> customer.inviter_id
ON CONFLICT (customer_user_id) DO NOTHING;

-- Refresh the single automatic legacy baseline immediately before the first
-- Shadow -> Live cutover. The caller supplies the frozen Live started_at value
-- and executes this function in the same database transaction as the settings
-- revision update.
CREATE OR REPLACE FUNCTION refresh_affiliate_qualification_legacy_baseline(
    p_cutoff_at TIMESTAMPTZ
)
RETURNS BIGINT
LANGUAGE plpgsql
AS $$
DECLARE
    affected_rows BIGINT := 0;
BEGIN
    IF p_cutoff_at IS NULL THEN
        RAISE EXCEPTION 'affiliate qualification cutoff_at is required';
    END IF;

    WITH paid_sources AS (
        -- Native balance recharge. 100 fen = 1 internal billing unit = ¥1.
        SELECT
            o.user_id,
            (o.amount_cny_fen::bigint * 10000)::bigint AS amount_micros
        FROM topup_orders o
        WHERE o.status = 'completed'
          AND COALESCE(o.completed_at, o.updated_at, o.created_at) < p_cutoff_at

        UNION ALL

        -- Native monthly-card payment. amount_cents is CNY fen despite the
        -- historical column name.
        SELECT
            o.user_id,
            (o.amount_cents::bigint * 10000)::bigint AS amount_micros
        FROM payment_orders o
        WHERE o.status = 'completed'
          AND COALESCE(o.completed_at, o.updated_at, o.created_at) < p_cutoff_at

        UNION ALL

        -- Sold card codes are external paid acquisitions. Inventory, gifts,
        -- compensation, tests, migration grants, and concurrency cards never
        -- qualify.
        SELECT
            r.used_by AS user_id,
            FLOOR(r.value::numeric * 1000000)::bigint AS amount_micros
        FROM redeem_codes r
        WHERE r.status = 'used'
          AND r.used_by IS NOT NULL
          AND r.purpose = 'sale_recharge'
          AND r.sales_status = 'sold'
          AND r.type IN ('balance', 'subscription')
          AND r.used_at IS NOT NULL
          AND r.used_at < p_cutoff_at
    ),
    known_paid AS (
        SELECT
            user_id,
            GREATEST(SUM(amount_micros), 0)::bigint AS amount_micros
        FROM paid_sources
        GROUP BY user_id
    ),
    observed_usage AS (
        SELECT
            l.user_id,
            GREATEST(
                FLOOR(SUM(GREATEST(l.actual_cost, 0))::numeric * 1000000),
                0
            )::bigint AS amount_micros
        FROM usage_logs l
        WHERE l.created_at < p_cutoff_at
        GROUP BY l.user_id
    ),
    baseline AS (
        SELECT
            p.user_id,
            p.amount_micros AS known_paid_micros,
            u.amount_micros AS observed_usage_micros,
            LEAST(p.amount_micros, u.amount_micros)::bigint
                AS confirmed_consumption_micros
        FROM known_paid p
        JOIN observed_usage u ON u.user_id = p.user_id
        JOIN users customer
          ON customer.id = p.user_id
         AND customer.deleted_at IS NULL
        WHERE p.amount_micros > 0
          AND u.amount_micros > 0
    ),
    upserted AS (
        INSERT INTO affiliate_qualification_baseline_entries (
            user_id,
            source_key,
            source_type,
            confirmed_consumption_micros,
            cutoff_at,
            metadata
        )
        SELECT
            b.user_id,
            'legacy-auto-v1:user:' || b.user_id::text,
            'legacy_auto',
            b.confirmed_consumption_micros,
            p_cutoff_at,
            jsonb_build_object(
                'method', 'min_known_paid_and_observed_actual_cost',
                'method_version', 'v1',
                'known_paid_micros', b.known_paid_micros,
                'observed_usage_micros', b.observed_usage_micros,
                'unit', '1_internal_unit_equals_1_cny'
            )
        FROM baseline b
        WHERE b.confirmed_consumption_micros > 0
        ON CONFLICT (source_key) DO NOTHING
        RETURNING 1
    )
    SELECT COUNT(*) INTO affected_rows FROM upserted;

    RETURN affected_rows;
END;
$$;
