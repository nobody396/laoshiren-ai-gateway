-- Use a denser cadence for account-sourced supplier probes.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

ALTER TABLE suppliers
    ALTER COLUMN probe_interval_minutes SET DEFAULT 2;

UPDATE suppliers
SET probe_interval_minutes = 2,
    next_probe_at = CASE
        WHEN probe_enabled = TRUE THEN LEAST(COALESCE(next_probe_at, NOW()), NOW())
        ELSE next_probe_at
    END,
    updated_at = NOW()
WHERE deleted_at IS NULL
    AND source_account_id IS NOT NULL;
