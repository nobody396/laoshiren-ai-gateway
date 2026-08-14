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
    CONSTRAINT native_checkout_offers_product_redeem_type_check
        CHECK (product_kind = redeem_type),
    CONSTRAINT native_checkout_offers_amount_check
        CHECK (pay_amount_cny_fen > 0 AND benefit_amount_cny_fen > 0),
    CONSTRAINT native_checkout_offers_balance_value_check
        CHECK (product_kind <> 'balance' OR benefit_amount_cny_fen = redeem_value * 100),
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

-- Disabled offers may be exposed to explicitly selected, owned test accounts
-- without making them visible or purchasable by the rest of the user base.
-- Production launch still uses native_checkout_offers.enabled; this table is
-- only a narrow pre-launch acceptance gate.
CREATE TABLE IF NOT EXISTS native_checkout_offer_testers (
    offer_code VARCHAR(64) NOT NULL REFERENCES native_checkout_offers(code) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (offer_code, user_id)
);

CREATE INDEX IF NOT EXISTS idx_native_checkout_offer_testers_user
    ON native_checkout_offer_testers (user_id, offer_code);

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

-- The offer row is the single source of truth for both customer-facing terms
-- and the exact redeem-code semantics.  Reject stock that does not match it;
-- this prevents the runbook, frontend, and inventory upload from silently
-- becoming separate commercial configurations.
CREATE OR REPLACE FUNCTION enforce_native_checkout_inventory_offer_match()
RETURNS TRIGGER AS $$
DECLARE
    checkout_offer native_checkout_offers%ROWTYPE;
    checkout_code redeem_codes%ROWTYPE;
BEGIN
    SELECT * INTO STRICT checkout_offer
    FROM native_checkout_offers
    WHERE code = NEW.offer_code;

    SELECT * INTO STRICT checkout_code
    FROM redeem_codes
    WHERE id = NEW.redeem_code_id;

    IF checkout_code.status <> 'unused'
       OR checkout_code.type <> checkout_offer.redeem_type
       OR checkout_code.value <> checkout_offer.redeem_value
       OR COALESCE(checkout_code.paid_value, 0) <> checkout_offer.redeem_paid_value
       OR checkout_code.purpose <> checkout_offer.redeem_purpose
       OR checkout_code.sales_status <> checkout_offer.redeem_sales_status
       OR checkout_code.group_ids <> checkout_offer.redeem_group_ids
       OR checkout_code.validity_days <> checkout_offer.redeem_validity_days THEN
        RAISE EXCEPTION 'native checkout inventory does not match offer %', NEW.offer_code
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_native_checkout_inventory_offer_match
    ON native_checkout_redeem_inventory;
CREATE TRIGGER trg_native_checkout_inventory_offer_match
    BEFORE INSERT OR UPDATE OF redeem_code_id, offer_code
    ON native_checkout_redeem_inventory
    FOR EACH ROW
    EXECUTE FUNCTION enforce_native_checkout_inventory_offer_match();

-- Once inventory is registered, changing its entitlement semantics would make
-- the canonical offer disagree with cards already uploaded to the provider.
-- Require a new offer instead of rewriting an in-flight inventory contract.
CREATE OR REPLACE FUNCTION guard_native_checkout_stocked_offer_semantics()
RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM native_checkout_redeem_inventory
        WHERE offer_code = OLD.code
    ) AND (
        NEW.redeem_type IS DISTINCT FROM OLD.redeem_type
        OR NEW.redeem_value IS DISTINCT FROM OLD.redeem_value
        OR NEW.redeem_paid_value IS DISTINCT FROM OLD.redeem_paid_value
        OR NEW.redeem_purpose IS DISTINCT FROM OLD.redeem_purpose
        OR NEW.redeem_sales_status IS DISTINCT FROM OLD.redeem_sales_status
        OR NEW.redeem_group_ids IS DISTINCT FROM OLD.redeem_group_ids
        OR NEW.redeem_validity_days IS DISTINCT FROM OLD.redeem_validity_days
    ) THEN
        RAISE EXCEPTION 'cannot change stocked native checkout offer semantics: %', OLD.code
            USING ERRCODE = '23514';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_native_checkout_stocked_offer_semantics
    ON native_checkout_offers;
CREATE TRIGGER trg_native_checkout_stocked_offer_semantics
    BEFORE UPDATE OF redeem_type, redeem_value, redeem_paid_value,
        redeem_purpose, redeem_sales_status, redeem_group_ids, redeem_validity_days
    ON native_checkout_offers
    FOR EACH ROW
    EXECUTE FUNCTION guard_native_checkout_stocked_offer_semantics();

-- Intentionally seed no offer and no inventory. Commercial terms, provider
-- goods key, stock semantics, and activation are created together in a later
-- explicitly reviewed operation. With zero rows, deployment cannot open sales.

RESET statement_timeout;
RESET lock_timeout;
