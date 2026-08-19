-- Add the EasyPay-compatible provider (彩虹易支付 MD5 protocol) alongside the
-- hardcoded xunhu topup channel. topup_orders records which gateway collected
-- the money; native checkout offers/orders may now point at an EasyPay-hosted
-- goods page instead of LDXP card stock.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE topup_orders
    ADD COLUMN IF NOT EXISTS provider VARCHAR(16) NOT NULL DEFAULT 'xunhu';

ALTER TABLE topup_orders
    DROP CONSTRAINT IF EXISTS topup_orders_provider_check;
ALTER TABLE topup_orders
    ADD CONSTRAINT topup_orders_provider_check
        CHECK (provider IN ('xunhu', 'easypay'));

ALTER TABLE native_checkout_offers
    DROP CONSTRAINT IF EXISTS native_checkout_offers_provider_check;
ALTER TABLE native_checkout_offers
    ADD CONSTRAINT native_checkout_offers_provider_check
        CHECK (provider IN ('ldxp', 'easypay'));

ALTER TABLE native_checkout_orders
    DROP CONSTRAINT IF EXISTS native_checkout_orders_provider_check;
ALTER TABLE native_checkout_orders
    ADD CONSTRAINT native_checkout_orders_provider_check
        CHECK (provider IN ('ldxp', 'easypay'));

-- The newcomer offer row is deliberately NOT repointed here. Flipping
-- provider 'ldxp' -> 'easypay' is a launch-time ops step (see
-- docs/ops/native-checkout.md): doing it in this migration would break the
-- live LDXP manual purchase path (manualCheckoutPurchaseURL requires
-- provider='ldxp') the moment this ships, before the native flow is enabled.

RESET statement_timeout;
RESET lock_timeout;
