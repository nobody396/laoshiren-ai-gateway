-- Refine the finance ledger's business dimensions without changing any amount:
-- - split recurring domain email from the one-off annual domain purchase;
-- - promote 链动小铺 from historical/other into a first-class income channel.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

ALTER TABLE finance_transactions
    DROP CONSTRAINT IF EXISTS chk_finance_transactions_category;

ALTER TABLE finance_transactions
    ADD CONSTRAINT chk_finance_transactions_category CHECK (category IN (
        'sale_revenue', 'other_income',
        'upstream_topup', 'server_cost', 'hosting_cost', 'cdn_cost',
        'domain_cost', 'domain_email_cost', 'early_cost', 'other_expense'
    ));

ALTER TABLE finance_transactions
    DROP CONSTRAINT IF EXISTS chk_finance_transactions_payment_channel;

ALTER TABLE finance_transactions
    ADD CONSTRAINT chk_finance_transactions_payment_channel CHECK (
        payment_channel IS NULL OR
        payment_channel IN (
            'wechat', 'alipay', 'liandong_shop', 'bank_transfer', 'other'
        )
    );

UPDATE finance_transactions
SET payment_channel = 'liandong_shop'
WHERE type = 'income'
  AND note ILIKE '%链动小铺%';

UPDATE finance_transactions
SET category = 'domain_email_cost'
WHERE type = 'expense'
  AND category = 'domain_cost'
  AND note ILIKE '%Spacemail%';
