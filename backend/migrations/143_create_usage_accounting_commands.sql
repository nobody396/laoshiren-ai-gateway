-- Durable, replayable accounting command queue. This migration is
-- forward-only, additive, and compatible with the previous application.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS usage_accounting_commands (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT NOT NULL,
    api_key_id BIGINT NOT NULL,
    usage_log_id BIGINT NOT NULL REFERENCES usage_logs(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL DEFAULT 1,
    payload JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_owner VARCHAR(128),
    lease_expires_at TIMESTAMPTZ,
    last_error_code VARCHAR(64),
    last_error_message VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    CONSTRAINT uq_usage_accounting_request_key UNIQUE (request_id, api_key_id),
    CONSTRAINT uq_usage_accounting_usage_log UNIQUE (usage_log_id),
    CONSTRAINT chk_usage_accounting_status CHECK (status IN ('pending', 'processing', 'completed', 'dead')),
    CONSTRAINT chk_usage_accounting_attempts CHECK (attempts >= 0),
    CONSTRAINT chk_usage_accounting_version CHECK (version > 0),
    CONSTRAINT chk_usage_accounting_payload_object CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_usage_accounting_due
    ON usage_accounting_commands (available_at, id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_usage_accounting_expired_lease
    ON usage_accounting_commands (lease_expires_at, id)
    WHERE status = 'processing';

CREATE INDEX IF NOT EXISTS idx_usage_accounting_dead
    ON usage_accounting_commands (updated_at, id)
    WHERE status = 'dead';
