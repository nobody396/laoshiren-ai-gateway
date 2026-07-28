-- Affiliate V2 source-aware consumption attribution.
-- All monetary and credit values are stored as integer micro-units.
-- Existing balances/subscriptions are explicitly grandfathered as ineligible.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS balance_lots (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT,
    source_key VARCHAR(180) NOT NULL UNIQUE,
    original_amount_micros BIGINT NOT NULL,
    remaining_amount_micros BIGINT NOT NULL,
    affiliate_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_balance_lot_source_type CHECK (
        source_type IN (
            'paid_redeem', 'paid_topup', 'gift', 'referral_bonus',
            'customer_rebate', 'commission_conversion', 'compensation',
            'internal_test', 'legacy_unattributed', 'admin_adjustment'
        )
    ),
    CONSTRAINT chk_balance_lot_amounts CHECK (
        original_amount_micros > 0 AND
        remaining_amount_micros >= 0 AND
        remaining_amount_micros <= original_amount_micros
    )
);

CREATE INDEX IF NOT EXISTS idx_balance_lots_fifo
    ON balance_lots (user_id, occurred_at, id)
    WHERE remaining_amount_micros > 0;
CREATE INDEX IF NOT EXISTS idx_balance_lots_source
    ON balance_lots (source_type, source_id)
    WHERE source_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS balance_lot_consumptions (
    id BIGSERIAL PRIMARY KEY,
    balance_lot_id BIGINT NOT NULL REFERENCES balance_lots(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    usage_log_id BIGINT,
    usage_event_key VARCHAR(180) NOT NULL,
    amount_micros BIGINT NOT NULL,
    affiliate_eligible_amount_micros BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_balance_lot_consumption UNIQUE (balance_lot_id, usage_event_key),
    CONSTRAINT chk_balance_lot_consumption_amount CHECK (
        amount_micros > 0 AND
        affiliate_eligible_amount_micros >= 0 AND
        affiliate_eligible_amount_micros <= amount_micros
    )
);

CREATE INDEX IF NOT EXISTS idx_balance_lot_consumptions_user
    ON balance_lot_consumptions (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_balance_lot_consumptions_usage_log
    ON balance_lot_consumptions (usage_log_id)
    WHERE usage_log_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS monthly_entitlement_cycles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT,
    source_key VARCHAR(180) NOT NULL UNIQUE,
    product_code VARCHAR(100) NOT NULL DEFAULT '',
    sale_price_micros BIGINT NOT NULL DEFAULT 0,
    credit_limit_micros BIGINT NOT NULL,
    used_credit_micros BIGINT NOT NULL DEFAULT 0,
    confirmed_consumption_micros BIGINT NOT NULL DEFAULT 0,
    affiliate_eligible BOOLEAN NOT NULL DEFAULT FALSE,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_monthly_entitlement_source_type CHECK (
        source_type IN (
            'paid_redeem', 'paid_topup', 'gift', 'compensation',
            'internal_test', 'legacy_unattributed', 'admin_adjustment'
        )
    ),
    CONSTRAINT chk_monthly_entitlement_values CHECK (
        sale_price_micros >= 0 AND
        credit_limit_micros > 0 AND
        used_credit_micros >= 0 AND
        used_credit_micros <= credit_limit_micros AND
        confirmed_consumption_micros >= 0 AND
        confirmed_consumption_micros <= sale_price_micros
    ),
    CONSTRAINT chk_monthly_entitlement_window CHECK (ends_at > starts_at),
    CONSTRAINT chk_monthly_entitlement_eligibility CHECK (
        affiliate_eligible = FALSE OR sale_price_micros > 0
    )
);

CREATE INDEX IF NOT EXISTS idx_monthly_entitlement_user_window
    ON monthly_entitlement_cycles (user_id, starts_at, ends_at, id);
CREATE INDEX IF NOT EXISTS idx_monthly_entitlement_source
    ON monthly_entitlement_cycles (source_type, source_id)
    WHERE source_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS monthly_entitlement_cycle_subscriptions (
    cycle_id BIGINT NOT NULL REFERENCES monthly_entitlement_cycles(id) ON DELETE RESTRICT,
    user_subscription_id BIGINT NOT NULL REFERENCES user_subscriptions(id) ON DELETE RESTRICT,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (cycle_id, user_subscription_id)
);

CREATE INDEX IF NOT EXISTS idx_monthly_cycle_subscription_lookup
    ON monthly_entitlement_cycle_subscriptions (user_subscription_id, cycle_id);

-- Freeze every pre-V2 balance as usable but ineligible. A later reconciliation
-- lot follows the same rule if sub-micro precision or an old writer creates a gap.
INSERT INTO balance_lots (
    user_id,
    source_type,
    source_key,
    original_amount_micros,
    remaining_amount_micros,
    affiliate_eligible,
    occurred_at
)
SELECT
    u.id,
    'legacy_unattributed',
    'legacy_balance:user:' || u.id::text,
    FLOOR(u.balance::numeric * 1000000)::bigint,
    FLOOR(u.balance::numeric * 1000000)::bigint,
    FALSE,
    NOW()
FROM users u
WHERE u.deleted_at IS NULL
  AND u.balance >= 0.000001
ON CONFLICT (source_key) DO NOTHING;

-- Existing subscriptions remain usable and visible to the attribution engine,
-- but can never create affiliate rewards. Future paid extensions create their
-- own eligible cycle beginning after this grandfathered entitlement.
INSERT INTO monthly_entitlement_cycles (
    user_id,
    source_type,
    source_key,
    product_code,
    sale_price_micros,
    credit_limit_micros,
    used_credit_micros,
    confirmed_consumption_micros,
    affiliate_eligible,
    starts_at,
    ends_at
)
SELECT
    us.user_id,
    'legacy_unattributed',
    'legacy_subscription:' || us.id::text,
    'legacy-group-' || us.group_id::text,
    0,
    GREATEST(
        1,
        FLOOR(
            COALESCE(
                NULLIF(g.monthly_limit_usd, 0),
                NULLIF(g.daily_limit_usd, 0) * GREATEST(1, g.default_validity_days),
                0.000001
            )::numeric * 1000000
        )::bigint
    ),
    LEAST(
        GREATEST(
            1,
            FLOOR(
                COALESCE(
                    NULLIF(g.monthly_limit_usd, 0),
                    NULLIF(g.daily_limit_usd, 0) * GREATEST(1, g.default_validity_days),
                    0.000001
                )::numeric * 1000000
            )::bigint
        ),
        GREATEST(0, FLOOR(us.monthly_usage_usd::numeric * 1000000)::bigint)
    ),
    0,
    FALSE,
    COALESCE(us.monthly_window_start, us.starts_at),
    GREATEST(
        us.expires_at,
        COALESCE(us.monthly_window_start, us.starts_at) + INTERVAL '1 second'
    )
FROM user_subscriptions us
JOIN groups g ON g.id = us.group_id
WHERE us.deleted_at IS NULL
  AND g.deleted_at IS NULL
ON CONFLICT (source_key) DO NOTHING;

INSERT INTO monthly_entitlement_cycle_subscriptions (
    cycle_id,
    user_subscription_id,
    group_id
)
SELECT
    c.id,
    us.id,
    us.group_id
FROM user_subscriptions us
JOIN monthly_entitlement_cycles c
  ON c.source_key = 'legacy_subscription:' || us.id::text
WHERE us.deleted_at IS NULL
ON CONFLICT (cycle_id, user_subscription_id) DO NOTHING;

COMMENT ON TABLE balance_lots IS 'FIFO source lots for paid and non-paid platform balance';
COMMENT ON TABLE balance_lot_consumptions IS 'Per-request FIFO balance attribution; eligible amount alone may drive affiliate events';
COMMENT ON TABLE monthly_entitlement_cycles IS 'One purchased monthly-card pool; shared group subscriptions map to one cycle';
