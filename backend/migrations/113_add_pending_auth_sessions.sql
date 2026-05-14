-- 113_add_pending_auth_sessions.sql
-- Pending OAuth state and post-callback continuation records.

CREATE TABLE IF NOT EXISTS pending_auth_sessions (
    id              BIGSERIAL PRIMARY KEY,
    state           VARCHAR(255) NOT NULL,
    provider        VARCHAR(50) NOT NULL,
    provider_user_id VARCHAR(255) NULL,
    intended_action VARCHAR(50) NOT NULL,
    claims_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    redirect_uri    TEXT NOT NULL DEFAULT '',
    user_id         BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    consumed_at     TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ip              TEXT NOT NULL DEFAULT '',
    user_agent      TEXT NOT NULL DEFAULT '',
    CONSTRAINT uq_pending_auth_sessions_state UNIQUE (state)
);

CREATE INDEX IF NOT EXISTS idx_pending_auth_sessions_expires_at
    ON pending_auth_sessions (expires_at);

CREATE INDEX IF NOT EXISTS idx_pending_auth_sessions_provider_subject
    ON pending_auth_sessions (provider, provider_user_id);

CREATE INDEX IF NOT EXISTS idx_pending_auth_sessions_user_provider
    ON pending_auth_sessions (user_id, provider);
