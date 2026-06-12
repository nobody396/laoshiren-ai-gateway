-- Supplier probe monitoring enhancements.
-- Keeps synced supplier rows tied to their source account without storing secrets in migrations.

ALTER TABLE suppliers
  ADD COLUMN IF NOT EXISTS source_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS source_platform TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_suppliers_source_account_id
  ON suppliers(source_account_id)
  WHERE source_account_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_suppliers_probe_due
  ON suppliers(next_probe_at)
  WHERE probe_enabled = TRUE AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_supplier_probe_results_checked_at
  ON supplier_probe_results(checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_supplier_probe_results_supplier_checked_at
  ON supplier_probe_results(supplier_id, checked_at DESC);
