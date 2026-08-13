-- Distinguish "awaiting payment" from "payment confirmed, checking delivery".
-- Once LDXP reports paid, the payable QR must disappear immediately and the
-- order must remain recoverable by the background reconciler.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE native_checkout_orders
    DROP CONSTRAINT IF EXISTS native_checkout_orders_status_check;

ALTER TABLE native_checkout_orders
    ADD CONSTRAINT native_checkout_orders_status_check
    CHECK (status IN ('creating', 'pending', 'checking', 'fulfilling', 'completed', 'failed', 'manual_review'));

DROP INDEX IF EXISTS uq_native_checkout_orders_active_per_user;
CREATE UNIQUE INDEX uq_native_checkout_orders_active_per_user
    ON native_checkout_orders (user_id, offer_code)
    WHERE status IN ('creating', 'pending', 'checking', 'fulfilling', 'manual_review');

DROP INDEX IF EXISTS idx_native_checkout_orders_reconcile;
CREATE INDEX idx_native_checkout_orders_reconcile
    ON native_checkout_orders (next_check_at, updated_at)
    WHERE status IN ('creating', 'pending', 'checking', 'fulfilling');

RESET statement_timeout;
RESET lock_timeout;
