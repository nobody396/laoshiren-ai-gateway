-- Store server-side monthly upstream health probes.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS monthly_upstream_probe_results (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    account_name VARCHAR(100) NOT NULL,
    platform VARCHAR(50) NOT NULL,
    model VARCHAR(120) NOT NULL,
    status VARCHAR(30) NOT NULL,
    http_status INT,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    error_code VARCHAR(120) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT monthly_upstream_probe_status_check CHECK (
        status IN ('ok', 'slow', 'rate_limited', 'failed', 'not_schedulable')
    ),
    CONSTRAINT monthly_upstream_probe_http_status_check CHECK (
        http_status IS NULL OR http_status >= 0
    )
);

CREATE INDEX IF NOT EXISTS idx_monthly_upstream_probe_checked_at
    ON monthly_upstream_probe_results (checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_monthly_upstream_probe_account_checked_at
    ON monthly_upstream_probe_results (account_name, checked_at DESC);

INSERT INTO settings (key, value, created_at, updated_at)
VALUES ('monthly_upstream_probe_enabled', 'false', NOW(), NOW())
ON CONFLICT (key) DO NOTHING;
