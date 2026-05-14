-- 080_add_first_invited_topup_marker.sql
-- 为“被邀请用户首次虎皮椒充值”奖励增加独立幂等标记

ALTER TABLE users ADD COLUMN IF NOT EXISTS first_invited_topup_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS first_invited_topup_order_id BIGINT;

CREATE INDEX IF NOT EXISTS users_first_invited_topup_at_idx
    ON users(first_invited_topup_at);
