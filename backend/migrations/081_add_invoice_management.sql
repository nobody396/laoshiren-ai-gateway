-- 081_add_invoice_management.sql
-- 新增充值订单开票管理相关表

ALTER TABLE topup_orders
    ADD COLUMN IF NOT EXISTS invoice_status VARCHAR(20) NOT NULL DEFAULT 'none';

CREATE INDEX IF NOT EXISTS topup_orders_invoice_status_idx
    ON topup_orders(invoice_status);

CREATE INDEX IF NOT EXISTS topup_orders_user_invoice_status_created_at_idx
    ON topup_orders(user_id, invoice_status, created_at DESC);

CREATE INDEX IF NOT EXISTS topup_orders_invoice_status_created_at_idx
    ON topup_orders(invoice_status, created_at DESC);

CREATE TABLE IF NOT EXISTS invoice_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    title VARCHAR(100) NOT NULL,
    tax_number VARCHAR(20) NOT NULL,
    email VARCHAR(100) NOT NULL,
    address VARCHAR(100),
    phone VARCHAR(40),
    bank_name VARCHAR(100),
    bank_account VARCHAR(100),
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS invoice_profiles_user_id_idx
    ON invoice_profiles(user_id);

CREATE UNIQUE INDEX IF NOT EXISTS invoice_profiles_user_default_idx
    ON invoice_profiles(user_id)
    WHERE is_default = TRUE;

CREATE TABLE IF NOT EXISTS invoice_requests (
    id BIGSERIAL PRIMARY KEY,
    serial_no VARCHAR(20) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL REFERENCES users(id),
    profile_id BIGINT REFERENCES invoice_profiles(id) ON DELETE SET NULL,
    profile_snapshot_json JSONB NOT NULL,
    total_amount_fen BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    export_batch_no VARCHAR(40),
    exported_at TIMESTAMPTZ,
    exported_by BIGINT,
    remark TEXT,
    reject_reason VARCHAR(255),
    completed_at TIMESTAMPTZ,
    completed_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS invoice_requests_user_id_idx
    ON invoice_requests(user_id);

CREATE INDEX IF NOT EXISTS invoice_requests_status_idx
    ON invoice_requests(status);

CREATE INDEX IF NOT EXISTS invoice_requests_user_status_created_at_idx
    ON invoice_requests(user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS invoice_requests_status_created_at_idx
    ON invoice_requests(status, created_at DESC);

CREATE TABLE IF NOT EXISTS invoice_request_orders (
    id BIGSERIAL PRIMARY KEY,
    invoice_request_id BIGINT NOT NULL REFERENCES invoice_requests(id) ON DELETE CASCADE,
    topup_order_id BIGINT NOT NULL REFERENCES topup_orders(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS invoice_request_orders_request_order_idx
    ON invoice_request_orders(invoice_request_id, topup_order_id);

CREATE UNIQUE INDEX IF NOT EXISTS invoice_request_orders_topup_order_id_idx
    ON invoice_request_orders(topup_order_id);
