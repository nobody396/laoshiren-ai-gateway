-- 111_add_users_last_active_at.sql
-- Add user-level last active timestamp for admin visibility and auth activity tracking.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS last_active_at TIMESTAMPTZ NULL;
