-- 111_add_users_last_active_at_notx.sql
-- Concurrent index for last_active_at to support admin sorting/filtering without long table locks.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_last_active_at
    ON users (last_active_at DESC NULLS LAST)
    WHERE deleted_at IS NULL;
