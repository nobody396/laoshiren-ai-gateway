-- Affiliate Program V3: manual partner review, source-locked reward policy,
-- conservative margin guard, and immutable application/status history.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

ALTER TABLE affiliate_program_settings
    DROP CONSTRAINT IF EXISTS chk_affiliate_program_version;

ALTER TABLE affiliate_program_settings
    ADD COLUMN IF NOT EXISTS ordinary_invitee_rate_bps INTEGER NOT NULL DEFAULT 500,
    ADD COLUMN IF NOT EXISTS operational_reserve_bps INTEGER NOT NULL DEFAULT 200,
    ADD COLUMN IF NOT EXISTS stress_cost_per_raw_credit_micros BIGINT NOT NULL DEFAULT 530000,
    ADD COLUMN IF NOT EXISTS stress_cost_snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS cost_snapshot_max_age_hours INTEGER NOT NULL DEFAULT 24;

UPDATE affiliate_program_settings
SET
    program_version = 'v3',
    ordinary_referral_rate_bps = 500,
    ordinary_invitee_rate_bps = 500,
    first_paid_bonus_threshold_micros = 0,
    first_paid_bonus_micros = 0,
    agent_pool_rate_bps = 1000,
    qualification_direct_user_count = 10,
    qualification_min_user_consumption_micros = 20000000,
    qualification_direct_team_consumption_micros = 1000000000,
    qualification_combined_consumption_micros = 2000000000,
    commission_conversion_multiplier_millis = 1200,
    operational_reserve_bps = 200,
    stress_cost_per_raw_credit_micros = 530000,
    cost_snapshot_max_age_hours = 24,
    updated_at = NOW()
WHERE id = 1;

UPDATE agent_commission_settings
SET
    first_recharge_invitee_rate = 0.050000,
    first_recharge_referral_rate = 0.050000,
    updated_at = NOW()
WHERE id = 1;

ALTER TABLE affiliate_program_settings
    ADD CONSTRAINT chk_affiliate_program_version CHECK (program_version = 'v3'),
    ADD CONSTRAINT chk_affiliate_v3_ordinary_rates CHECK (
        ordinary_referral_rate_bps = 500
        AND ordinary_invitee_rate_bps = 500
    ),
    ADD CONSTRAINT chk_affiliate_v3_conversion_multiplier CHECK (
        commission_conversion_multiplier_millis = 1200
    ),
    ADD CONSTRAINT chk_affiliate_operational_reserve CHECK (
        operational_reserve_bps BETWEEN 200 AND 3000
    ),
    ADD CONSTRAINT chk_affiliate_stress_cost CHECK (
        stress_cost_per_raw_credit_micros > 0
    ),
    ADD CONSTRAINT chk_affiliate_cost_snapshot_age CHECK (
        cost_snapshot_max_age_hours BETWEEN 1 AND 720
    );

ALTER TABLE agent_principals
    DROP CONSTRAINT IF EXISTS chk_agent_principal_status;

ALTER TABLE agent_principals
    ADD COLUMN IF NOT EXISTS applied_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS application_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS decision_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS terminated_at TIMESTAMPTZ,
    ADD CONSTRAINT chk_agent_principal_status CHECK (
        status IN (
            'candidate', 'pending_review', 'active', 'rejected',
            'suspended', 'terminated'
        )
    ),
    ADD CONSTRAINT chk_agent_principal_termination CHECK (
        status <> 'terminated' OR terminated_at IS NOT NULL
    );

CREATE TABLE IF NOT EXISTS affiliate_agent_applications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(24) NOT NULL DEFAULT 'pending_review',
    qualifying_route VARCHAR(24) NOT NULL,
    direct_valid_consumer_count INTEGER NOT NULL,
    direct_team_consumption_micros BIGINT NOT NULL,
    application_note TEXT NOT NULL DEFAULT '',
    decision_note TEXT NOT NULL DEFAULT '',
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_application_status CHECK (
        status IN ('pending_review', 'approved', 'rejected', 'cancelled')
    ),
    CONSTRAINT chk_affiliate_application_route CHECK (
        qualifying_route IN ('direct_team', 'direct_volume')
    ),
    CONSTRAINT chk_affiliate_application_totals CHECK (
        direct_valid_consumer_count >= 0 AND
        direct_team_consumption_micros >= 0
    ),
    CONSTRAINT chk_affiliate_application_review CHECK (
        (status = 'pending_review' AND reviewed_at IS NULL AND reviewed_by IS NULL)
        OR
        (status <> 'pending_review' AND reviewed_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_application_pending
    ON affiliate_agent_applications (user_id)
    WHERE status = 'pending_review';
CREATE INDEX IF NOT EXISTS idx_affiliate_applications_review_queue
    ON affiliate_agent_applications (status, submitted_at, id);

CREATE TABLE IF NOT EXISTS affiliate_agent_status_events (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    previous_status VARCHAR(24),
    next_status VARCHAR(24) NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    application_id BIGINT REFERENCES affiliate_agent_applications(id) ON DELETE RESTRICT,
    operator_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_status_event_previous CHECK (
        previous_status IS NULL OR previous_status IN (
            'candidate', 'pending_review', 'active', 'rejected',
            'suspended', 'terminated'
        )
    ),
    CONSTRAINT chk_affiliate_status_event_next CHECK (
        next_status IN (
            'candidate', 'pending_review', 'active', 'rejected',
            'suspended', 'terminated'
        )
    ),
    CONSTRAINT chk_affiliate_status_event_metadata CHECK (
        jsonb_typeof(metadata) = 'object'
    )
);

CREATE INDEX IF NOT EXISTS idx_affiliate_status_events_agent_time
    ON affiliate_agent_status_events (agent_id, effective_at DESC, id DESC);

ALTER TABLE balance_lots
    ADD COLUMN IF NOT EXISTS affiliate_policy VARCHAR(32) NOT NULL DEFAULT 'NONE',
    ADD COLUMN IF NOT EXISTS direct_partner_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS customer_rebate_rate_bps INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS partner_commission_rate_bps INTEGER NOT NULL DEFAULT 0;

UPDATE balance_lots
SET
    affiliate_policy = 'NONE',
    affiliate_eligible = FALSE,
    direct_partner_id = NULL,
    customer_rebate_rate_bps = 0,
    partner_commission_rate_bps = 0;

ALTER TABLE balance_lots
    ADD CONSTRAINT chk_balance_lot_affiliate_policy CHECK (
        affiliate_policy IN ('NONE', 'ORDINARY_FIRST_PAID', 'PARTNER_USAGE')
    ),
    ADD CONSTRAINT chk_balance_lot_affiliate_rates CHECK (
        customer_rebate_rate_bps BETWEEN 0 AND 1000 AND
        partner_commission_rate_bps BETWEEN 0 AND 1000
    ),
    ADD CONSTRAINT chk_balance_lot_affiliate_shape CHECK (
        (
            affiliate_policy = 'PARTNER_USAGE'
            AND affiliate_eligible = TRUE
            AND direct_partner_id IS NOT NULL
            AND customer_rebate_rate_bps + partner_commission_rate_bps = 1000
        )
        OR
        (
            affiliate_policy <> 'PARTNER_USAGE'
            AND affiliate_eligible = (affiliate_policy = 'ORDINARY_FIRST_PAID')
            AND customer_rebate_rate_bps = 0
            AND partner_commission_rate_bps = 0
        )
    );

CREATE INDEX IF NOT EXISTS idx_balance_lots_affiliate_policy
    ON balance_lots (user_id, affiliate_policy, occurred_at, id)
    WHERE remaining_amount_micros > 0;

ALTER TABLE monthly_entitlement_cycles
    ADD COLUMN IF NOT EXISTS affiliate_policy VARCHAR(32) NOT NULL DEFAULT 'NONE',
    ADD COLUMN IF NOT EXISTS direct_partner_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN IF NOT EXISTS customer_rebate_rate_bps INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS partner_commission_rate_bps INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pricing_table_version VARCHAR(32) NOT NULL DEFAULT 'legacy';

UPDATE monthly_entitlement_cycles
SET
    affiliate_policy = 'NONE',
    affiliate_eligible = FALSE,
    direct_partner_id = NULL,
    customer_rebate_rate_bps = 0,
    partner_commission_rate_bps = 0,
    pricing_table_version = CASE
        WHEN pricing_table_version = '' THEN 'legacy'
        ELSE pricing_table_version
    END;

ALTER TABLE monthly_entitlement_cycles
    ADD CONSTRAINT chk_monthly_entitlement_affiliate_policy CHECK (
        affiliate_policy IN ('NONE', 'ORDINARY_FIRST_PAID', 'PARTNER_USAGE')
    ),
    ADD CONSTRAINT chk_monthly_entitlement_affiliate_rates CHECK (
        customer_rebate_rate_bps BETWEEN 0 AND 1000 AND
        partner_commission_rate_bps BETWEEN 0 AND 1000
    ),
    ADD CONSTRAINT chk_monthly_entitlement_affiliate_shape CHECK (
        (
            affiliate_policy = 'PARTNER_USAGE'
            AND affiliate_eligible = TRUE
            AND direct_partner_id IS NOT NULL
            AND customer_rebate_rate_bps + partner_commission_rate_bps = 1000
        )
        OR
        (
            affiliate_policy <> 'PARTNER_USAGE'
            AND affiliate_eligible = (affiliate_policy = 'ORDINARY_FIRST_PAID')
            AND customer_rebate_rate_bps = 0
            AND partner_commission_rate_bps = 0
        )
    ),
    ADD CONSTRAINT chk_monthly_pricing_table_version CHECK (
        BTRIM(pricing_table_version) <> ''
    );

ALTER TABLE affiliate_performance_events
    ADD COLUMN IF NOT EXISTS affiliate_policy VARCHAR(32) NOT NULL DEFAULT 'NONE',
    ADD COLUMN IF NOT EXISTS customer_rebate_rate_bps INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS partner_commission_rate_bps INTEGER NOT NULL DEFAULT 0;

ALTER TABLE affiliate_performance_events
    ADD CONSTRAINT chk_affiliate_performance_policy CHECK (
        affiliate_policy IN ('NONE', 'ORDINARY_FIRST_PAID', 'PARTNER_USAGE')
    ),
    ADD CONSTRAINT chk_affiliate_performance_rates CHECK (
        customer_rebate_rate_bps BETWEEN 0 AND 1000 AND
        partner_commission_rate_bps BETWEEN 0 AND 1000 AND
        (
            affiliate_policy <> 'PARTNER_USAGE'
            OR customer_rebate_rate_bps + partner_commission_rate_bps = 1000
        )
    );

ALTER TABLE affiliate_reward_entries
    DROP CONSTRAINT IF EXISTS chk_affiliate_reward_type;
ALTER TABLE affiliate_reward_entries
    ADD CONSTRAINT chk_affiliate_reward_type CHECK (
        reward_type IN (
            'ordinary_referral', 'ordinary_invitee', 'first_paid_bonus',
            'customer_rebate', 'reversal'
        )
    );

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_withdrawal_payment_reference
    ON agent_withdrawal_requests (payment_reference)
    WHERE status = 'paid' AND BTRIM(payment_reference) <> '';

COMMENT ON COLUMN balance_lots.affiliate_policy IS
    'Immutable V3 source policy; purchase/activation changes never rewrite old lots';
COMMENT ON COLUMN monthly_entitlement_cycles.affiliate_policy IS
    'Immutable V3 source policy for one purchased 31-day entitlement';
COMMENT ON TABLE affiliate_agent_applications IS
    'Manual partner application snapshots; reaching thresholds never auto-activates';
COMMENT ON TABLE affiliate_agent_status_events IS
    'Immutable partner state transition audit';
