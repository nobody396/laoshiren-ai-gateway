-- Native checkout keeps the customer on our site while an external card shop
-- collects payment and delivers a one-time redeem code.  Orders snapshot every
-- fulfillment invariant so later offer edits cannot change an in-flight order.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS native_checkout_offers (
    code VARCHAR(64) PRIMARY KEY,
    provider VARCHAR(32) NOT NULL,
    provider_goods_key VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(160) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    product_kind VARCHAR(32) NOT NULL,
    pay_amount_cny_fen BIGINT NOT NULL,
    benefit_amount_cny_fen BIGINT NOT NULL,
    redeem_type VARCHAR(20) NOT NULL,
    redeem_value NUMERIC(20,8) NOT NULL,
    redeem_paid_value NUMERIC(20,8) NOT NULL DEFAULT 0,
    redeem_purpose VARCHAR(32) NOT NULL,
    redeem_sales_status VARCHAR(32) NOT NULL,
    redeem_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    redeem_validity_days INTEGER NOT NULL DEFAULT 30,
    once_per_user BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT native_checkout_offers_provider_check
        CHECK (provider IN ('ldxp')),
    CONSTRAINT native_checkout_offers_product_kind_check
        CHECK (product_kind IN ('balance', 'subscription')),
    CONSTRAINT native_checkout_offers_amount_check
        CHECK (pay_amount_cny_fen > 0 AND benefit_amount_cny_fen > 0),
    CONSTRAINT native_checkout_offers_redeem_value_check
        CHECK (redeem_value > 0 AND redeem_paid_value >= 0 AND redeem_paid_value <= redeem_value),
    CONSTRAINT native_checkout_offers_redeem_type_check
        CHECK (redeem_type IN ('balance', 'subscription')),
    CONSTRAINT native_checkout_offers_redeem_purpose_check
        CHECK (redeem_purpose IN ('sale_recharge', 'gift', 'compensation', 'internal_test', 'migration')),
    CONSTRAINT native_checkout_offers_redeem_sales_status_check
        CHECK (redeem_sales_status IN ('inventory', 'sold', 'gifted')),
    CONSTRAINT native_checkout_offers_group_ids_check
        CHECK (jsonb_typeof(redeem_group_ids) = 'array'),
    CONSTRAINT native_checkout_offers_validity_days_check
        CHECK (redeem_validity_days >= 0)
);

CREATE INDEX IF NOT EXISTS idx_native_checkout_offers_enabled_sort
    ON native_checkout_offers (enabled, sort_order, code);

CREATE TABLE IF NOT EXISTS native_checkout_orders (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(64) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    offer_code VARCHAR(64) NOT NULL REFERENCES native_checkout_offers(code) ON DELETE RESTRICT,
    provider VARCHAR(32) NOT NULL,
    provider_goods_key VARCHAR(64) NOT NULL,
    provider_trade_no VARCHAR(128),
    payment_url TEXT,
    contact_hash CHAR(64) NOT NULL,
    product_kind VARCHAR(32) NOT NULL,
    pay_amount_cny_fen BIGINT NOT NULL,
    benefit_amount_cny_fen BIGINT NOT NULL,
    redeem_type VARCHAR(20) NOT NULL,
    redeem_value NUMERIC(20,8) NOT NULL,
    redeem_paid_value NUMERIC(20,8) NOT NULL DEFAULT 0,
    redeem_purpose VARCHAR(32) NOT NULL,
    redeem_sales_status VARCHAR(32) NOT NULL,
    redeem_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    redeem_validity_days INTEGER NOT NULL DEFAULT 30,
    enforce_once BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(24) NOT NULL,
    redeem_code_id BIGINT REFERENCES redeem_codes(id) ON DELETE RESTRICT,
    failure_code VARCHAR(64) NOT NULL DEFAULT '',
    check_count INTEGER NOT NULL DEFAULT 0,
    next_check_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    fulfillment_started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT native_checkout_orders_provider_check
        CHECK (provider IN ('ldxp')),
    CONSTRAINT native_checkout_orders_product_kind_check
        CHECK (product_kind IN ('balance', 'subscription')),
    CONSTRAINT native_checkout_orders_amount_check
        CHECK (pay_amount_cny_fen > 0 AND benefit_amount_cny_fen > 0),
    CONSTRAINT native_checkout_orders_redeem_value_check
        CHECK (redeem_value > 0 AND redeem_paid_value >= 0 AND redeem_paid_value <= redeem_value),
    CONSTRAINT native_checkout_orders_group_ids_check
        CHECK (jsonb_typeof(redeem_group_ids) = 'array'),
    CONSTRAINT native_checkout_orders_status_check
        CHECK (status IN ('creating', 'pending', 'fulfilling', 'completed', 'failed', 'manual_review')),
    CONSTRAINT native_checkout_orders_trade_no_check
        CHECK (provider_trade_no IS NULL OR LENGTH(BTRIM(provider_trade_no)) > 0),
    CONSTRAINT native_checkout_orders_payment_url_check
        CHECK (payment_url IS NULL OR LENGTH(BTRIM(payment_url)) > 0),
    CONSTRAINT native_checkout_orders_contact_hash_check
        CHECK (contact_hash ~ '^[0-9a-f]{64}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_native_checkout_orders_provider_trade_no
    ON native_checkout_orders (provider, provider_trade_no)
    WHERE provider_trade_no IS NOT NULL;

-- A once-only offer owns one durable row for the user's lifetime.  Definitive
-- creation failures retry by resetting that same row; ambiguous failures remain
-- held for manual review so a network timeout cannot create a double charge.
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_checkout_orders_once_per_user
    ON native_checkout_orders (user_id, offer_code)
    WHERE enforce_once;

-- Repeatable offers still allow only one active checkout at a time.
CREATE UNIQUE INDEX IF NOT EXISTS uq_native_checkout_orders_active_per_user
    ON native_checkout_orders (user_id, offer_code)
    WHERE status IN ('creating', 'pending', 'fulfilling', 'manual_review');

CREATE INDEX IF NOT EXISTS idx_native_checkout_orders_reconcile
    ON native_checkout_orders (next_check_at, updated_at)
    WHERE status IN ('creating', 'pending', 'fulfilling');

CREATE INDEX IF NOT EXISTS idx_native_checkout_orders_user_created
    ON native_checkout_orders (user_id, created_at DESC);

COMMENT ON COLUMN native_checkout_orders.contact_hash IS
    'Keyed HMAC-SHA-256 of the normalized registered email used as provider contact; the email itself is not duplicated here.';

-- Codes stocked in an external shop must not remain usable through the public
-- manual-redeem endpoint.  Assignment is claimed atomically with fulfillment;
-- only the native-checkout service receives the in-process authorization to
-- redeem a restricted code.
CREATE TABLE IF NOT EXISTS native_checkout_redeem_inventory (
    redeem_code_id BIGINT PRIMARY KEY REFERENCES redeem_codes(id) ON DELETE RESTRICT,
    offer_code VARCHAR(64) NOT NULL REFERENCES native_checkout_offers(code) ON DELETE RESTRICT,
    assigned_order_id BIGINT UNIQUE REFERENCES native_checkout_orders(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_native_checkout_redeem_inventory_offer_unassigned
    ON native_checkout_redeem_inventory (offer_code, redeem_code_id)
    WHERE assigned_order_id IS NULL;

-- First pilot: the full ¥5 entitlement is a gift card (paid_value=0,
-- purpose=gift, sales_status=gifted).  LDXP product oc3w4r is hidden from the
-- public card-shop catalog and requires email contact.  The application
-- database unique index is the authoritative per-account limit; provider-side
-- IP limits are unsuitable because all buyer API calls originate server-side.
INSERT INTO native_checkout_offers (
    code, provider, provider_goods_key, name, description, product_kind,
    pay_amount_cny_fen, benefit_amount_cny_fen, redeem_type, redeem_value,
    redeem_paid_value, redeem_purpose, redeem_sales_status, redeem_group_ids,
    redeem_validity_days, once_per_user, enabled, sort_order
) VALUES (
    'trial-balance-1-to-5', 'ldxp', 'oc3w4r',
    '1 元体验，到账 5 元赠送额度',
    '5 元全部作为体验赠送额度发放，每个账号仅可购买一次。',
    'balance', 100, 500, 'balance', 5, 0, 'gift', 'gifted', '[]'::jsonb,
    0, TRUE, TRUE, 10
)
ON CONFLICT (code) DO UPDATE SET
    provider = EXCLUDED.provider,
    provider_goods_key = EXCLUDED.provider_goods_key,
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    product_kind = EXCLUDED.product_kind,
    pay_amount_cny_fen = EXCLUDED.pay_amount_cny_fen,
    benefit_amount_cny_fen = EXCLUDED.benefit_amount_cny_fen,
    redeem_type = EXCLUDED.redeem_type,
    redeem_value = EXCLUDED.redeem_value,
    redeem_paid_value = EXCLUDED.redeem_paid_value,
    redeem_purpose = EXCLUDED.redeem_purpose,
    redeem_sales_status = EXCLUDED.redeem_sales_status,
    redeem_group_ids = EXCLUDED.redeem_group_ids,
    redeem_validity_days = EXCLUDED.redeem_validity_days,
    once_per_user = EXCLUDED.once_per_user,
    enabled = EXCLUDED.enabled,
    sort_order = EXCLUDED.sort_order,
    updated_at = NOW();

-- Production bootstrap batch. Other environments legitimately insert zero
-- rows. Replenishment must create the same pure-gift semantics and register
-- each new code in this inventory table before uploading it to LDXP.
INSERT INTO native_checkout_redeem_inventory (redeem_code_id, offer_code)
SELECT rc.id, 'trial-balance-1-to-5'
FROM redeem_codes rc
JOIN redeem_code_batches batch ON batch.id = rc.batch_id
WHERE batch.name = 'native-checkout-trial-1-to-5-20260813'
  AND rc.type = 'balance'
  AND rc.value = 5
  AND COALESCE(rc.paid_value, 0) = 0
  AND rc.purpose = 'gift'
  AND rc.sales_status = 'gifted'
ON CONFLICT (redeem_code_id) DO NOTHING;

RESET statement_timeout;
RESET lock_timeout;
