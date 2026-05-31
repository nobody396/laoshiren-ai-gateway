-- Refine supplier evaluation into upstream probes and target group planning.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE suppliers DROP CONSTRAINT IF EXISTS suppliers_status_check;
ALTER TABLE suppliers DROP CONSTRAINT IF EXISTS suppliers_risk_level_check;
ALTER TABLE suppliers DROP CONSTRAINT IF EXISTS suppliers_stability_level_check;
ALTER TABLE suppliers DROP CONSTRAINT IF EXISTS suppliers_cost_amount_check;

ALTER TABLE suppliers
    ADD COLUMN IF NOT EXISTS base_url VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS api_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS upstream_group VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS contact_platform VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS contact_value VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cost_rmb_per_usd NUMERIC(20, 8),
    ADD COLUMN IF NOT EXISTS probe_enabled BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS probe_model VARCHAR(100) NOT NULL DEFAULT 'gpt-5.1-codex-mini',
    ADD COLUMN IF NOT EXISTS probe_interval_minutes INT NOT NULL DEFAULT 30,
    ADD COLUMN IF NOT EXISTS last_probe_status VARCHAR(20) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS last_probe_latency_ms BIGINT,
    ADD COLUMN IF NOT EXISTS last_probe_error TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_probe_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS next_probe_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS probe_success_rate NUMERIC(5, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS probe_success_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS probe_total_count INT NOT NULL DEFAULT 0;

UPDATE suppliers
SET status = CASE
    WHEN status = 'active' THEN 'active'
    ELSE 'evaluating'
END;

ALTER TABLE suppliers
    ALTER COLUMN status SET DEFAULT 'evaluating',
    ADD CONSTRAINT suppliers_status_check CHECK (status IN ('evaluating', 'active')),
    ADD CONSTRAINT suppliers_cost_rmb_per_usd_check CHECK (cost_rmb_per_usd IS NULL OR cost_rmb_per_usd >= 0),
    ADD CONSTRAINT suppliers_probe_interval_check CHECK (probe_interval_minutes BETWEEN 1 AND 1440),
    ADD CONSTRAINT suppliers_probe_status_check CHECK (last_probe_status IN ('unknown', 'success', 'failed')),
    ADD CONSTRAINT suppliers_probe_counts_check CHECK (
        probe_success_count >= 0 AND probe_total_count >= 0 AND probe_success_count <= probe_total_count
    );

ALTER TABLE suppliers
    DROP COLUMN IF EXISTS console_url,
    DROP COLUMN IF EXISTS docs_url,
    DROP COLUMN IF EXISTS contact_name,
    DROP COLUMN IF EXISTS contact_method,
    DROP COLUMN IF EXISTS risk_level,
    DROP COLUMN IF EXISTS stability_level,
    DROP COLUMN IF EXISTS cost_amount,
    DROP COLUMN IF EXISTS cost_unit,
    DROP COLUMN IF EXISTS payment_terms,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS last_reviewed_at;

DROP INDEX IF EXISTS idx_suppliers_risk_level;
DROP INDEX IF EXISTS idx_suppliers_stability_level;
CREATE INDEX IF NOT EXISTS idx_suppliers_probe_status ON suppliers(last_probe_status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_suppliers_next_probe_at ON suppliers(next_probe_at) WHERE deleted_at IS NULL AND probe_enabled = true;

DROP TABLE IF EXISTS supplier_accounts;

CREATE TABLE IF NOT EXISTS supplier_probe_results (
    id             BIGSERIAL PRIMARY KEY,
    supplier_id    BIGINT NOT NULL REFERENCES suppliers(id) ON DELETE CASCADE,
    status         VARCHAR(20) NOT NULL DEFAULT 'success',
    model          VARCHAR(100) NOT NULL DEFAULT '',
    latency_ms     BIGINT NOT NULL DEFAULT 0,
    accuracy_ok    BOOLEAN NOT NULL DEFAULT false,
    response_text  TEXT NOT NULL DEFAULT '',
    error_message  TEXT NOT NULL DEFAULT '',
    checked_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT supplier_probe_results_status_check CHECK (status IN ('success', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_supplier_probe_results_supplier_created
    ON supplier_probe_results(supplier_id, created_at DESC);

INSERT INTO admin_apis ("group", path, method, description, sort_order, status)
VALUES ('供应商考察', '/admin/suppliers/:id/probe', 'POST', '立即执行供应商探针', 5, 'active')
ON CONFLICT (method, path) DO UPDATE SET
    "group" = EXCLUDED."group",
    description = EXCLUDED.description,
    sort_order = EXCLUDED.sort_order,
    status = EXCLUDED.status,
    updated_at = NOW();

UPDATE admin_apis
SET sort_order = 6, updated_at = NOW()
WHERE method = 'DELETE' AND path = '/admin/suppliers/:id';

COMMENT ON TABLE suppliers IS '供应商考察池：记录上游官网、Key/Base URL、目标分组与探针稳定性';
COMMENT ON TABLE supplier_groups IS '供应商计划服务的本地业务分组';
COMMENT ON TABLE supplier_probe_results IS '供应商上游 Key/Base URL 的探针轮询结果';
