-- Manual bookkeeping ledger: real cash in/out (sale revenue, upstream top-ups,
-- server/domain costs, ...). Independent of the theoretical cost-accounting
-- margin calculator; the two are not meant to be reconciled row-for-row.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS finance_transactions (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(10) NOT NULL,
    category VARCHAR(30) NOT NULL,
    amount_fen BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    note TEXT,
    receipt_key VARCHAR(255),
    source VARCHAR(10) NOT NULL DEFAULT 'manual',
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_finance_transactions_type CHECK (type IN ('income', 'expense')),
    CONSTRAINT chk_finance_transactions_category CHECK (category IN (
        'sale_revenue', 'other_income',
        'upstream_topup', 'server_cost', 'domain_cost', 'early_cost', 'other_expense'
    )),
    CONSTRAINT chk_finance_transactions_amount_positive CHECK (amount_fen > 0),
    CONSTRAINT chk_finance_transactions_source CHECK (source IN ('manual', 'skill'))
);

CREATE INDEX IF NOT EXISTS idx_finance_transactions_type ON finance_transactions (type);
CREATE INDEX IF NOT EXISTS idx_finance_transactions_category ON finance_transactions (category);
CREATE INDEX IF NOT EXISTS idx_finance_transactions_occurred_at ON finance_transactions (occurred_at);
CREATE INDEX IF NOT EXISTS idx_finance_transactions_created_at ON finance_transactions (created_at);

-- Sidebar entry. Sits right after Feedback (100) and before Proxies (110).
INSERT INTO admin_menus (name, name_en, type, path, component, icon, permission_key, sort_order, status)
VALUES
    ('财务记账', 'Finance Ledger', 'menu', '/admin/finance-transactions', '', 'dollar', 'admin:finance-transactions', 105, 'active')
ON CONFLICT (permission_key) DO UPDATE SET
    name = EXCLUDED.name,
    name_en = EXCLUDED.name_en,
    type = EXCLUDED.type,
    path = EXCLUDED.path,
    component = EXCLUDED.component,
    icon = EXCLUDED.icon,
    sort_order = EXCLUDED.sort_order,
    status = EXCLUDED.status,
    updated_at = now();
