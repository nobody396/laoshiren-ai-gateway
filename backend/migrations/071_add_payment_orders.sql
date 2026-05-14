-- 071: Add payment_orders table for Stripe Checkout payments

CREATE TABLE IF NOT EXISTS payment_orders (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(64) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    amount_cents INTEGER NOT NULL,
    credits DECIMAL(20,8) NOT NULL DEFAULT 0,
    currency VARCHAR(3) NOT NULL DEFAULT 'usd',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    stripe_session_id VARCHAR(255),
    stripe_payment_intent_id VARCHAR(255),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_payment_orders_user_id ON payment_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_payment_orders_stripe_session_id ON payment_orders(stripe_session_id);
CREATE INDEX IF NOT EXISTS idx_payment_orders_status ON payment_orders(status);
