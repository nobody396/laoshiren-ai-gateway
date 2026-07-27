-- Affiliate V2 risk review, immutable reversal audit, and withdrawal events.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS affiliate_risk_actions (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    action_type VARCHAR(32) NOT NULL,
    previous_risk_status VARCHAR(24) NOT NULL,
    next_risk_status VARCHAR(24) NOT NULL,
    reason TEXT NOT NULL,
    released_reward_count INTEGER NOT NULL DEFAULT 0,
    released_reward_micros BIGINT NOT NULL DEFAULT 0,
    released_cash_count INTEGER NOT NULL DEFAULT 0,
    released_cash_micros BIGINT NOT NULL DEFAULT 0,
    operator_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_risk_action_type CHECK (
        action_type IN ('review', 'block', 'clear')
    ),
    CONSTRAINT chk_affiliate_risk_action_statuses CHECK (
        previous_risk_status IN ('clear', 'review', 'blocked') AND
        next_risk_status IN ('clear', 'review', 'blocked')
    ),
    CONSTRAINT chk_affiliate_risk_action_reason CHECK (BTRIM(reason) <> ''),
    CONSTRAINT chk_affiliate_risk_release_totals CHECK (
        released_reward_count >= 0 AND released_reward_micros >= 0 AND
        released_cash_count >= 0 AND released_cash_micros >= 0
    ),
    CONSTRAINT chk_affiliate_risk_action_metadata CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_affiliate_risk_actions_agent_time
    ON affiliate_risk_actions (agent_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS affiliate_performance_reversals (
    id BIGSERIAL PRIMARY KEY,
    original_event_id BIGINT NOT NULL UNIQUE
        REFERENCES affiliate_performance_events(id) ON DELETE RESTRICT,
    reversal_event_id BIGINT NOT NULL UNIQUE
        REFERENCES affiliate_performance_events(id) ON DELETE RESTRICT,
    consumer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    direct_agent_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    amount_micros BIGINT NOT NULL,
    reversed_reward_micros BIGINT NOT NULL DEFAULT 0,
    reversed_cash_micros BIGINT NOT NULL DEFAULT 0,
    reason TEXT NOT NULL,
    operator_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_performance_reversal_amounts CHECK (
        amount_micros > 0 AND
        reversed_reward_micros >= 0 AND
        reversed_cash_micros >= 0
    ),
    CONSTRAINT chk_affiliate_performance_reversal_reason CHECK (BTRIM(reason) <> '')
);

CREATE INDEX IF NOT EXISTS idx_affiliate_performance_reversals_agent_time
    ON affiliate_performance_reversals (direct_agent_id, created_at DESC, id DESC)
    WHERE direct_agent_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_reward_reversal
    ON affiliate_reward_entries (reversal_of_id)
    WHERE reversal_of_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_cash_reversal
    ON agent_cash_commission_entries (related_entry_id)
    WHERE entry_type = 'reversal' AND related_entry_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS agent_withdrawal_events (
    id BIGSERIAL PRIMARY KEY,
    withdrawal_id BIGINT NOT NULL
        REFERENCES agent_withdrawal_requests(id) ON DELETE RESTRICT,
    agent_id BIGINT NOT NULL REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    event_type VARCHAR(24) NOT NULL,
    previous_status VARCHAR(24),
    next_status VARCHAR(24) NOT NULL,
    operator_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    note TEXT NOT NULL DEFAULT '',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_withdrawal_event_type CHECK (
        event_type IN ('requested', 'paid', 'failed')
    ),
    CONSTRAINT chk_agent_withdrawal_event_status CHECK (
        (previous_status IS NULL OR previous_status IN ('processing', 'paid', 'failed')) AND
        next_status IN ('processing', 'paid', 'failed')
    ),
    CONSTRAINT chk_agent_withdrawal_event_metadata CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_withdrawal_event_once
    ON agent_withdrawal_events (withdrawal_id, event_type);
CREATE INDEX IF NOT EXISTS idx_agent_withdrawal_events_agent_time
    ON agent_withdrawal_events (agent_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION guard_agent_payment_profile_during_withdrawal()
RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM agent_withdrawal_requests
        WHERE agent_id = OLD.agent_id
          AND status = 'processing'
    ) THEN
        RAISE EXCEPTION 'agent payment profile is locked by a processing withdrawal'
            USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_guard_agent_payment_profile_during_withdrawal
    ON agent_payment_profiles;
CREATE TRIGGER trg_guard_agent_payment_profile_during_withdrawal
    BEFORE UPDATE ON agent_payment_profiles
    FOR EACH ROW
    EXECUTE FUNCTION guard_agent_payment_profile_during_withdrawal();

CREATE TABLE IF NOT EXISTS agent_payment_qr_access_events (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    withdrawal_id BIGINT REFERENCES agent_withdrawal_requests(id) ON DELETE RESTRICT,
    accessor_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    access_context VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_payment_qr_access_context CHECK (
        access_context IN ('agent_self', 'admin_profile', 'withdrawal_snapshot')
    ),
    CONSTRAINT chk_agent_payment_qr_access_shape CHECK (
        (access_context = 'withdrawal_snapshot' AND withdrawal_id IS NOT NULL) OR
        (access_context <> 'withdrawal_snapshot' AND withdrawal_id IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_agent_payment_qr_access_agent_time
    ON agent_payment_qr_access_events (agent_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_agent_payment_qr_access_withdrawal
    ON agent_payment_qr_access_events (withdrawal_id, created_at DESC, id DESC)
    WHERE withdrawal_id IS NOT NULL;

ALTER TABLE affiliate_agent_notices
    DROP CONSTRAINT IF EXISTS chk_affiliate_agent_notice_type;
ALTER TABLE affiliate_agent_notices
    ADD CONSTRAINT chk_affiliate_agent_notice_type CHECK (
        notice_type IN (
            'community_invite',
            'withdrawal_paid',
            'withdrawal_failed',
            'risk_review',
            'risk_blocked',
            'risk_cleared',
            'commission_reversed'
        )
    );

COMMENT ON TABLE affiliate_risk_actions IS
    'Operator audit for agent risk state changes and held-fund releases';
COMMENT ON TABLE affiliate_performance_reversals IS
    'Exactly-once full reversals of confirmed Affiliate V2 consumption events';
COMMENT ON TABLE agent_withdrawal_events IS
    'Immutable withdrawal state-transition audit; user UI remains processing to paid';
COMMENT ON TABLE agent_payment_qr_access_events IS
    'Private Alipay QR access audit without storing QR contents in logs';
