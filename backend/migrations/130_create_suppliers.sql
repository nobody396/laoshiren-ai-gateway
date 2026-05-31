-- Create supplier evaluation tables for upstream procurement candidates.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE TABLE IF NOT EXISTS suppliers (
    id               BIGSERIAL PRIMARY KEY,
    name             VARCHAR(100) NOT NULL,
    website_url      VARCHAR(500) NOT NULL DEFAULT '',
    console_url      VARCHAR(500) NOT NULL DEFAULT '',
    docs_url         VARCHAR(500) NOT NULL DEFAULT '',
    contact_name     VARCHAR(100) NOT NULL DEFAULT '',
    contact_method   VARCHAR(200) NOT NULL DEFAULT '',
    status           VARCHAR(20) NOT NULL DEFAULT 'candidate',
    risk_level       VARCHAR(20) NOT NULL DEFAULT 'unknown',
    stability_level  VARCHAR(20) NOT NULL DEFAULT 'unknown',
    cost_amount      NUMERIC(20, 8),
    cost_unit        VARCHAR(100) NOT NULL DEFAULT '',
    payment_terms    TEXT NOT NULL DEFAULT '',
    currency         VARCHAR(16) NOT NULL DEFAULT 'USD',
    notes            TEXT NOT NULL DEFAULT '',
    last_reviewed_at TIMESTAMPTZ,
    deleted_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT suppliers_status_check CHECK (status IN ('candidate', 'trial', 'active', 'paused', 'rejected')),
    CONSTRAINT suppliers_risk_level_check CHECK (risk_level IN ('unknown', 'low', 'medium', 'high')),
    CONSTRAINT suppliers_stability_level_check CHECK (stability_level IN ('unknown', 'stable', 'watch', 'unstable')),
    CONSTRAINT suppliers_cost_amount_check CHECK (cost_amount IS NULL OR cost_amount >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_suppliers_name_active_unique
    ON suppliers (LOWER(name))
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_suppliers_status ON suppliers(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_suppliers_risk_level ON suppliers(risk_level) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_suppliers_stability_level ON suppliers(stability_level) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_suppliers_deleted_at ON suppliers(deleted_at);

CREATE TABLE IF NOT EXISTS supplier_groups (
    id          BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES suppliers(id) ON DELETE CASCADE,
    group_id    BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    relationship VARCHAR(20) NOT NULL DEFAULT 'candidate',
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (supplier_id, group_id)
);

CREATE INDEX IF NOT EXISTS idx_supplier_groups_supplier_id ON supplier_groups(supplier_id);
CREATE INDEX IF NOT EXISTS idx_supplier_groups_group_id ON supplier_groups(group_id);

CREATE TABLE IF NOT EXISTS supplier_accounts (
    id          BIGSERIAL PRIMARY KEY,
    supplier_id BIGINT NOT NULL REFERENCES suppliers(id) ON DELETE CASCADE,
    account_id  BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    relationship VARCHAR(20) NOT NULL DEFAULT 'trial',
    notes       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (supplier_id, account_id)
);

CREATE INDEX IF NOT EXISTS idx_supplier_accounts_supplier_id ON supplier_accounts(supplier_id);
CREATE INDEX IF NOT EXISTS idx_supplier_accounts_account_id ON supplier_accounts(account_id);

INSERT INTO admin_menus (name, name_en, type, path, component, icon, permission_key, sort_order, status)
VALUES ('供应商考察', 'Suppliers', 'menu', '/admin/suppliers', '', 'server', 'admin:suppliers', 65, 'active')
ON CONFLICT (permission_key) DO UPDATE SET
    name = EXCLUDED.name,
    name_en = EXCLUDED.name_en,
    type = EXCLUDED.type,
    path = EXCLUDED.path,
    component = EXCLUDED.component,
    icon = EXCLUDED.icon,
    sort_order = EXCLUDED.sort_order,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO admin_apis ("group", path, method, description, sort_order, status)
VALUES
    ('供应商考察', '/admin/suppliers', 'GET', '分页查询供应商列表', 1, 'active'),
    ('供应商考察', '/admin/suppliers/:id', 'GET', '供应商详情', 2, 'active'),
    ('供应商考察', '/admin/suppliers', 'POST', '新建供应商', 3, 'active'),
    ('供应商考察', '/admin/suppliers/:id', 'PUT', '编辑供应商', 4, 'active'),
    ('供应商考察', '/admin/suppliers/:id', 'DELETE', '删除供应商', 5, 'active')
ON CONFLICT (method, path) DO UPDATE SET
    "group" = EXCLUDED."group",
    description = EXCLUDED.description,
    sort_order = EXCLUDED.sort_order,
    status = EXCLUDED.status,
    updated_at = NOW();

COMMENT ON TABLE suppliers IS '供应商考察池：记录上游采购候选、成本、风险与稳定性结论';
COMMENT ON TABLE supplier_groups IS '供应商可覆盖的业务分组';
COMMENT ON TABLE supplier_accounts IS '供应商与已纳入账号的关联，便于从考察池晋升到账号管理后追踪来源';
