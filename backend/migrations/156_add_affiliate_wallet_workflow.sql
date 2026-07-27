-- Affiliate V2 on-demand withdrawals, cash-to-credit conversion, and notices.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS agent_withdrawal_requests (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    amount_micros BIGINT NOT NULL,
    status VARCHAR(24) NOT NULL DEFAULT 'processing',
    idempotency_key VARCHAR(180) NOT NULL,
    payment_alipay_real_name TEXT NOT NULL,
    payment_alipay_account TEXT NOT NULL,
    payment_contact_phone TEXT NOT NULL DEFAULT '',
    payment_note TEXT NOT NULL DEFAULT '',
    payment_qr_object_key TEXT NOT NULL,
    payment_qr_content_type VARCHAR(64) NOT NULL DEFAULT '',
    payment_qr_original_filename TEXT NOT NULL DEFAULT '',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    due_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    handled_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    payment_reference TEXT NOT NULL DEFAULT '',
    failure_reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_withdrawal_idempotency UNIQUE (agent_id, idempotency_key),
    CONSTRAINT chk_agent_withdrawal_amount CHECK (amount_micros > 0),
    CONSTRAINT chk_agent_withdrawal_status CHECK (status IN ('processing', 'paid', 'failed')),
    CONSTRAINT chk_agent_withdrawal_due CHECK (due_at >= requested_at),
    CONSTRAINT chk_agent_withdrawal_paid_time CHECK (
        (status = 'paid' AND paid_at IS NOT NULL) OR status <> 'paid'
    ),
    CONSTRAINT chk_agent_withdrawal_failed_time CHECK (
        (status = 'failed' AND failed_at IS NOT NULL) OR status <> 'failed'
    )
);

CREATE INDEX IF NOT EXISTS idx_agent_withdrawals_agent_time
    ON agent_withdrawal_requests (agent_id, requested_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_agent_withdrawals_processing_due
    ON agent_withdrawal_requests (due_at, id)
    WHERE status = 'processing';

CREATE TABLE IF NOT EXISTS agent_commission_conversions (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    cash_amount_micros BIGINT NOT NULL,
    credit_amount_micros BIGINT NOT NULL,
    multiplier_millis INTEGER NOT NULL,
    idempotency_key VARCHAR(180) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_agent_conversion_idempotency UNIQUE (agent_id, idempotency_key),
    CONSTRAINT chk_agent_conversion_amounts CHECK (
        cash_amount_micros > 0 AND credit_amount_micros > 0
    ),
    CONSTRAINT chk_agent_conversion_multiplier CHECK (multiplier_millis >= 1000)
);

CREATE INDEX IF NOT EXISTS idx_agent_conversions_agent_time
    ON agent_commission_conversions (agent_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS affiliate_agent_notices (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL REFERENCES agent_principals(agent_id) ON DELETE CASCADE,
    notice_type VARCHAR(32) NOT NULL,
    title VARCHAR(120) NOT NULL,
    message TEXT NOT NULL,
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT,
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    read_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_agent_notice_type CHECK (
        notice_type IN ('community_invite', 'withdrawal_paid', 'withdrawal_failed')
    ),
    CONSTRAINT chk_affiliate_agent_notice_metadata CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_affiliate_agent_notices_unread
    ON affiliate_agent_notices (agent_id, created_at DESC, id DESC)
    WHERE read_at IS NULL;
