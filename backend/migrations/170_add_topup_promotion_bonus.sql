-- Persist the promotional bonus selected when an order is created.
-- Historical orders remain zero so new promotion rules never apply retroactively.
ALTER TABLE topup_orders
    ADD COLUMN IF NOT EXISTS bonus_amount_cny_fen INTEGER NOT NULL DEFAULT 0;

ALTER TABLE topup_orders
    DROP CONSTRAINT IF EXISTS topup_orders_bonus_amount_cny_fen_nonnegative;

ALTER TABLE topup_orders
    ADD CONSTRAINT topup_orders_bonus_amount_cny_fen_nonnegative
    CHECK (bonus_amount_cny_fen >= 0);
