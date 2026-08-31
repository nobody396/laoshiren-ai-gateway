-- Add a gated commission-wallet checkout channel. The commercial cutover
-- flips these settings only after the application version is deployed and
-- verified; migration defaults preserve the current 1.2x conversion behavior.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE affiliate_program_settings
    ADD COLUMN IF NOT EXISTS commission_wallet_checkout_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS commission_wallet_purchase_rate_bps INTEGER NOT NULL DEFAULT 8500,
    ADD COLUMN IF NOT EXISTS commission_conversion_enabled BOOLEAN NOT NULL DEFAULT TRUE;

ALTER TABLE affiliate_program_settings
    DROP CONSTRAINT IF EXISTS affiliate_program_settings_wallet_purchase_rate_check;
ALTER TABLE affiliate_program_settings
    ADD CONSTRAINT affiliate_program_settings_wallet_purchase_rate_check
    CHECK (commission_wallet_purchase_rate_bps BETWEEN 1 AND 10000);

ALTER TABLE native_checkout_orders
    DROP CONSTRAINT IF EXISTS native_checkout_orders_provider_check;
ALTER TABLE native_checkout_orders
    ADD CONSTRAINT native_checkout_orders_provider_check
    CHECK (provider IN ('ldxp', 'easypay', 'affiliate_wallet'));

ALTER TABLE native_checkout_orders
    ALTER COLUMN payment_method TYPE VARCHAR(32);
ALTER TABLE native_checkout_orders
    DROP CONSTRAINT IF EXISTS native_checkout_orders_payment_method_check;
ALTER TABLE native_checkout_orders
    ADD CONSTRAINT native_checkout_orders_payment_method_check
    CHECK (payment_method IS NULL OR payment_method IN ('wechat', 'alipay', 'commission_wallet'));

ALTER TABLE balance_lots
    DROP CONSTRAINT IF EXISTS chk_balance_lot_source_type;
ALTER TABLE balance_lots
    ADD CONSTRAINT chk_balance_lot_source_type CHECK (
        source_type IN (
            'paid_redeem', 'paid_topup', 'gift', 'referral_bonus',
            'customer_rebate', 'commission_conversion', 'commission_purchase',
            'compensation', 'erroneous_charge_refund',
            'internal_test', 'legacy_unattributed', 'admin_adjustment'
        )
    );

COMMENT ON COLUMN affiliate_program_settings.commission_wallet_checkout_enabled IS
    'Whether active clear-risk partners may pay eligible products directly from cash commission';
COMMENT ON COLUMN affiliate_program_settings.commission_wallet_purchase_rate_bps IS
    'Commission-wallet charge as basis points of the live product price; 8500 means 85%';
COMMENT ON COLUMN affiliate_program_settings.commission_conversion_enabled IS
    'Whether new cash-commission to platform-credit conversions are accepted';

CREATE TABLE IF NOT EXISTS monthly_commercial_cutovers (
    id BIGSERIAL PRIMARY KEY,
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    effective_at TIMESTAMPTZ NOT NULL,
    result JSONB NOT NULL,
    executed_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT monthly_commercial_cutovers_result_check CHECK (jsonb_typeof(result)='object'),
    CONSTRAINT monthly_commercial_cutovers_effective_at_unique UNIQUE(effective_at)
);

RESET statement_timeout;
RESET lock_timeout;
