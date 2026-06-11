-- Distinguish user-experience gateway probes from direct upstream diagnostics.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE monthly_upstream_probe_results
    ADD COLUMN IF NOT EXISTS probe_path VARCHAR(40) NOT NULL DEFAULT 'direct_upstream';

ALTER TABLE monthly_upstream_probe_results
    DROP CONSTRAINT IF EXISTS monthly_upstream_probe_path_check;

ALTER TABLE monthly_upstream_probe_results
    ADD CONSTRAINT monthly_upstream_probe_path_check CHECK (
        probe_path IN ('gateway', 'direct_upstream')
    );

CREATE INDEX IF NOT EXISTS idx_monthly_upstream_probe_path_account_checked_at
    ON monthly_upstream_probe_results (probe_path, account_name, checked_at DESC);
