-- Extend supplier probes to RelayPulse-style status classes and diagnostics.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE suppliers DROP CONSTRAINT IF EXISTS suppliers_probe_status_check;

ALTER TABLE suppliers
    ADD COLUMN IF NOT EXISTS last_probe_sub_status VARCHAR(40) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS last_probe_http_code INT,
    ALTER COLUMN probe_model SET DEFAULT 'claude-haiku-4-5-20251001',
    ADD CONSTRAINT suppliers_probe_status_check CHECK (last_probe_status IN ('unknown', 'success', 'degraded', 'failed')),
    ADD CONSTRAINT suppliers_probe_http_code_check CHECK (last_probe_http_code IS NULL OR last_probe_http_code >= 0);

ALTER TABLE supplier_probe_results DROP CONSTRAINT IF EXISTS supplier_probe_results_status_check;

ALTER TABLE supplier_probe_results
    ADD COLUMN IF NOT EXISTS sub_status VARCHAR(40) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS http_code INT NOT NULL DEFAULT 0,
    ADD CONSTRAINT supplier_probe_results_status_check CHECK (status IN ('success', 'degraded', 'failed')),
    ADD CONSTRAINT supplier_probe_results_http_code_check CHECK (http_code >= 0);
