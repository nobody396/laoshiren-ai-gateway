-- Affiliate V2 first-paid purchase rewards and maturity support.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS affiliate_first_paid_purchases (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    purchase_type VARCHAR(32) NOT NULL,
    source_id BIGINT,
    purchase_key VARCHAR(180) NOT NULL UNIQUE,
    amount_micros BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_first_paid_type CHECK (
        purchase_type IN ('balance_redeem', 'balance_topup', 'monthly_redeem', 'monthly_payment')
    ),
    CONSTRAINT chk_affiliate_first_paid_amount CHECK (amount_micros > 0)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_first_paid_occurred
    ON affiliate_first_paid_purchases (occurred_at DESC);

ALTER TABLE affiliate_reward_entries
    ADD COLUMN IF NOT EXISTS posted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_affiliate_rewards_maturity
    ON affiliate_reward_entries (available_at, id)
    WHERE status = 'pending';

-- Every account has an invite code. Existing custom codes are preserved.
-- Retry a salted deterministic candidate if a historical custom code happens
-- to collide with the generated value.
DO $$
DECLARE
    target RECORD;
    salt INTEGER;
    candidate VARCHAR(20);
BEGIN
    FOR target IN
        SELECT id
        FROM users
        WHERE invite_code IS NULL OR BTRIM(invite_code) = ''
        ORDER BY id
    LOOP
        salt := 0;
        LOOP
            candidate := 'U' || UPPER(SUBSTRING(MD5(target.id::text || ':' || salt::text) FROM 1 FOR 15));
            EXIT WHEN NOT EXISTS (
                SELECT 1
                FROM users
                WHERE invite_code = candidate
                  AND id <> target.id
            );
            salt := salt + 1;
        END LOOP;
        UPDATE users
        SET invite_code = candidate,
            updated_at = NOW()
        WHERE id = target.id;
    END LOOP;
END
$$;

COMMENT ON TABLE affiliate_first_paid_purchases IS 'One live-program first real paid purchase per invited user';
COMMENT ON COLUMN affiliate_reward_entries.posted_at IS 'Time platform credit became spendable';
