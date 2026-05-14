-- 072: Migrate payment_orders from Stripe to Alipay

-- Remove Stripe-specific columns
ALTER TABLE payment_orders DROP COLUMN IF EXISTS stripe_session_id;
ALTER TABLE payment_orders DROP COLUMN IF EXISTS stripe_payment_intent_id;
ALTER TABLE payment_orders DROP COLUMN IF EXISTS credits;
ALTER TABLE payment_orders DROP COLUMN IF EXISTS currency;

-- Add Alipay-specific columns
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS alipay_trade_no VARCHAR(64);
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS qr_code_url TEXT;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS plan_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS group_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE payment_orders ADD COLUMN IF NOT EXISTS validity_days INTEGER NOT NULL DEFAULT 30;

-- Update indexes
DROP INDEX IF EXISTS idx_payment_orders_stripe_session_id;
CREATE INDEX IF NOT EXISTS idx_payment_orders_alipay_trade_no ON payment_orders(alipay_trade_no);
CREATE INDEX IF NOT EXISTS idx_payment_orders_plan_id ON payment_orders(plan_id);
