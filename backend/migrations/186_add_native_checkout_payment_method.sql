-- Persist the exact LDXP QR channel selected when an order is created. The
-- customer prompt must remain tied to that order even if the merchant later
-- switches the product between WeChat Pay and Alipay.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE native_checkout_orders
    ADD COLUMN IF NOT EXISTS payment_method VARCHAR(16);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'native_checkout_orders'::regclass
          AND conname = 'native_checkout_orders_payment_method_check'
    ) THEN
        ALTER TABLE native_checkout_orders
            ADD CONSTRAINT native_checkout_orders_payment_method_check
            CHECK (payment_method IS NULL OR payment_method IN ('wechat', 'alipay'));
    END IF;
END
$$;
