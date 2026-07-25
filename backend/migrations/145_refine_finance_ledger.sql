-- Refine the manual finance ledger:
-- - distinguish hosting (fixed) from CDN (variable) cost;
-- - record the collection channel for screenshot-backed manual income.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

ALTER TABLE finance_transactions
    ADD COLUMN IF NOT EXISTS payment_channel VARCHAR(20);

-- Existing manual income predates channel capture. Mark it explicitly as
-- unknown/other so every historical income remains editable under the new
-- application invariant without inventing WeChat or Alipay attribution.
UPDATE finance_transactions
SET payment_channel = 'other'
WHERE type = 'income'
  AND payment_channel IS NULL;

ALTER TABLE finance_transactions
    DROP CONSTRAINT IF EXISTS chk_finance_transactions_category;

ALTER TABLE finance_transactions
    ADD CONSTRAINT chk_finance_transactions_category CHECK (category IN (
        'sale_revenue', 'other_income',
        'upstream_topup', 'server_cost', 'hosting_cost', 'cdn_cost',
        'domain_cost', 'early_cost', 'other_expense'
    ));

ALTER TABLE finance_transactions
    DROP CONSTRAINT IF EXISTS chk_finance_transactions_payment_channel;

ALTER TABLE finance_transactions
    ADD CONSTRAINT chk_finance_transactions_payment_channel CHECK (
        payment_channel IS NULL OR
        payment_channel IN ('wechat', 'alipay', 'bank_transfer', 'other')
    );

CREATE INDEX IF NOT EXISTS idx_finance_transactions_payment_channel
    ON finance_transactions (payment_channel);
