-- 122_add_agent_payment_profiles.sql
-- 代理收款资料与结算快照。

ALTER TABLE agent_commission_settings
    ADD COLUMN IF NOT EXISTS settlement_min_amount DECIMAL(20,8) NOT NULL DEFAULT 50.00000000;

CREATE TABLE IF NOT EXISTS agent_payment_profiles (
    agent_id BIGINT PRIMARY KEY,
    alipay_real_name TEXT NOT NULL DEFAULT '',
    alipay_account TEXT NOT NULL DEFAULT '',
    contact_phone TEXT NOT NULL DEFAULT '',
    payment_note TEXT NOT NULL DEFAULT '',
    alipay_qr_object_key TEXT NOT NULL DEFAULT '',
    alipay_qr_content_type VARCHAR(64) NOT NULL DEFAULT '',
    alipay_qr_original_filename TEXT NOT NULL DEFAULT '',
    alipay_qr_size BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS agent_payment_profiles_updated_at_idx
    ON agent_payment_profiles(updated_at);

ALTER TABLE agent_settlements
    ADD COLUMN IF NOT EXISTS payment_alipay_real_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_alipay_account TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_contact_phone TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_qr_object_key TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS payment_reference TEXT NOT NULL DEFAULT '';
