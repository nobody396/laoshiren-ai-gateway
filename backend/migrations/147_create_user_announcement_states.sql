-- Separate popup delivery from read state so a backlog can interrupt a user once
-- without being marked as read or replayed one announcement at a time.
CREATE TABLE IF NOT EXISTS user_announcement_states (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_prompted_announcement_id BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_announcement_states_last_prompted_nonnegative
        CHECK (last_prompted_announcement_id >= 0)
);

COMMENT ON TABLE user_announcement_states IS 'Per-user announcement delivery cursor, separate from read receipts';
COMMENT ON COLUMN user_announcement_states.last_prompted_announcement_id IS 'Highest visible popup announcement included in a batch reminder';
