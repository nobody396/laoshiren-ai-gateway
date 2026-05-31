-- 129_add_redeem_code_billing_metadata.sql
-- Add structured card-code billing metadata for redeem-code based recharge reconciliation.

CREATE TABLE IF NOT EXISTS redeem_code_batches (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    purpose VARCHAR(32) NOT NULL DEFAULT 'sale_recharge',
    face_value DECIMAL(20,8) NOT NULL DEFAULT 0,
    currency VARCHAR(16) NOT NULL DEFAULT 'balance_unit',
    sales_channel VARCHAR(32) NOT NULL DEFAULT 'manual',
    external_url TEXT,
    notes TEXT,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS redeem_code_batches_purpose_idx
    ON redeem_code_batches(purpose);

CREATE INDEX IF NOT EXISTS redeem_code_batches_created_at_idx
    ON redeem_code_batches(created_at DESC);

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS batch_id BIGINT REFERENCES redeem_code_batches(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS purpose VARCHAR(32) NOT NULL DEFAULT 'sale_recharge',
    ADD COLUMN IF NOT EXISTS sales_status VARCHAR(32) NOT NULL DEFAULT 'inventory',
    ADD COLUMN IF NOT EXISTS sold_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS sold_to_note TEXT,
    ADD COLUMN IF NOT EXISTS external_order_no VARCHAR(128),
    ADD COLUMN IF NOT EXISTS external_order_url TEXT,
    ADD COLUMN IF NOT EXISTS internal_notes TEXT,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE redeem_codes
SET purpose = 'migration'
WHERE type <> 'balance'
  AND purpose = 'sale_recharge';

UPDATE redeem_codes
SET sales_status = 'gifted'
WHERE purpose IN ('gift', 'compensation')
  AND sales_status = 'inventory';

CREATE INDEX IF NOT EXISTS redeem_codes_batch_id_idx
    ON redeem_codes(batch_id);

CREATE INDEX IF NOT EXISTS redeem_codes_purpose_idx
    ON redeem_codes(purpose);

CREATE INDEX IF NOT EXISTS redeem_codes_sales_status_idx
    ON redeem_codes(sales_status);

CREATE INDEX IF NOT EXISTS redeem_codes_purpose_sales_status_idx
    ON redeem_codes(purpose, sales_status);

CREATE INDEX IF NOT EXISTS redeem_codes_used_at_idx
    ON redeem_codes(used_at DESC);

CREATE INDEX IF NOT EXISTS redeem_codes_created_at_desc_idx
    ON redeem_codes(created_at DESC);

COMMENT ON TABLE redeem_code_batches IS '兑换码批次，用于卡密销售/赠送/补偿对账';
COMMENT ON COLUMN redeem_codes.purpose IS '卡密用途: sale_recharge/gift/compensation/internal_test/migration';
COMMENT ON COLUMN redeem_codes.sales_status IS '销售状态: inventory/sold/gifted/void';
COMMENT ON COLUMN redeem_codes.sold_to_note IS '买家或领取人备注';
COMMENT ON COLUMN redeem_codes.external_order_url IS '外部购买或订单链接';
COMMENT ON COLUMN redeem_codes.internal_notes IS '管理员内部备注';
