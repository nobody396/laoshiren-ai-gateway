-- Normalize the external order identity and customer-paid gross amount already
-- present in finance ledger notes. Customer Tier can then use indexed exact
-- order linkage instead of rescanning and reparsing every 90-day ledger row for
-- every customer evaluation.

SET LOCAL lock_timeout='5s';
SET LOCAL statement_timeout='2min';

CREATE OR REPLACE FUNCTION extract_finance_external_order_no(value TEXT)
RETURNS TEXT
LANGUAGE SQL
IMMUTABLE
PARALLEL SAFE
AS $$
 SELECT COALESCE(
  substring(upper(COALESCE(value,'')) from '订单[[:space:]]+([0-9A-Z][0-9A-Z_-]{5,})'),
  substring(upper(COALESCE(value,'')) from '订单号[^0-9A-Z]*([0-9A-Z][0-9A-Z_-]{5,})'),
  substring(upper(COALESCE(value,'')) from '(LD[0-9A-Z]+)')
 )
$$;

CREATE OR REPLACE FUNCTION extract_finance_gross_amount_fen(value TEXT, fallback BIGINT)
RETURNS BIGINT
LANGUAGE SQL
IMMUTABLE
PARALLEL SAFE
AS $$
 SELECT COALESCE(
  ROUND((substring(COALESCE(value,'') from '毛额[^0-9]*([0-9]+[.][0-9]+)'))::numeric*100)::bigint,
  fallback
 )
$$;

ALTER TABLE finance_transactions
 ADD COLUMN IF NOT EXISTS external_order_no TEXT
  GENERATED ALWAYS AS (extract_finance_external_order_no(note)) STORED,
 ADD COLUMN IF NOT EXISTS gross_amount_fen BIGINT
  GENERATED ALWAYS AS (extract_finance_gross_amount_fen(note,amount_fen)) STORED;

ALTER TABLE finance_transactions ALTER COLUMN gross_amount_fen SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_finance_transactions_external_sale_order
 ON finance_transactions(upper(btrim(external_order_no)),occurred_at DESC,id DESC)
 WHERE type='income' AND category='sale_revenue' AND external_order_no IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_external_order_lookup
 ON redeem_codes(upper(btrim(external_order_no)),id)
 WHERE external_order_no IS NOT NULL;

COMMENT ON COLUMN finance_transactions.external_order_no IS
 'Generated exact external sale order identity parsed from the canonical finance note.';
COMMENT ON COLUMN finance_transactions.gross_amount_fen IS
 'Generated customer-paid gross amount; falls back to amount_fen when the note has no separate gross amount.';
