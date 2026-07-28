-- Affiliate Program V2 foundation. Additive and disabled by default.
-- Monetary values use integer micro-units (1 CNY/credit = 1,000,000 micros)
-- and rates use basis points (10% = 1,000 bps) to avoid floating-point drift.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS affiliate_program_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    program_version VARCHAR(16) NOT NULL DEFAULT 'v2',
    mode VARCHAR(16) NOT NULL DEFAULT 'off',
    started_at TIMESTAMPTZ,
    ordinary_referral_rate_bps INTEGER NOT NULL DEFAULT 500,
    first_paid_bonus_threshold_micros BIGINT NOT NULL DEFAULT 50000000,
    first_paid_bonus_micros BIGINT NOT NULL DEFAULT 5000000,
    agent_pool_rate_bps INTEGER NOT NULL DEFAULT 1000,
    qualification_direct_user_count INTEGER NOT NULL DEFAULT 10,
    qualification_min_user_consumption_micros BIGINT NOT NULL DEFAULT 20000000,
    qualification_direct_team_consumption_micros BIGINT NOT NULL DEFAULT 1000000000,
    qualification_combined_consumption_micros BIGINT NOT NULL DEFAULT 2000000000,
    max_campaign_links INTEGER NOT NULL DEFAULT 5,
    commission_conversion_multiplier_millis INTEGER NOT NULL DEFAULT 1200,
    withdrawal_min_micros BIGINT NOT NULL DEFAULT 100000000,
    withdrawal_sla_hours INTEGER NOT NULL DEFAULT 24,
    margin_floor_bps INTEGER NOT NULL DEFAULT 3500,
    revision BIGINT NOT NULL DEFAULT 1,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_program_version CHECK (program_version = 'v2'),
    CONSTRAINT chk_affiliate_program_mode CHECK (mode IN ('off', 'shadow', 'live')),
    CONSTRAINT chk_affiliate_program_started CHECK (mode <> 'live' OR started_at IS NOT NULL),
    CONSTRAINT chk_affiliate_ordinary_referral_rate CHECK (ordinary_referral_rate_bps BETWEEN 0 AND 10000),
    CONSTRAINT chk_affiliate_agent_pool_rate CHECK (agent_pool_rate_bps BETWEEN 0 AND 10000),
    CONSTRAINT chk_affiliate_bonus_values CHECK (
        first_paid_bonus_threshold_micros >= 0 AND first_paid_bonus_micros >= 0
    ),
    CONSTRAINT chk_affiliate_qualification_values CHECK (
        qualification_direct_user_count > 0 AND
        qualification_min_user_consumption_micros > 0 AND
        qualification_direct_team_consumption_micros > 0 AND
        qualification_combined_consumption_micros > 0
    ),
    CONSTRAINT chk_affiliate_link_limit CHECK (max_campaign_links BETWEEN 0 AND 100),
    CONSTRAINT chk_affiliate_conversion_multiplier CHECK (commission_conversion_multiplier_millis >= 1000),
    CONSTRAINT chk_affiliate_withdrawal_values CHECK (withdrawal_min_micros > 0 AND withdrawal_sla_hours > 0),
    CONSTRAINT chk_affiliate_margin_floor CHECK (margin_floor_bps BETWEEN 0 AND 10000),
    CONSTRAINT chk_affiliate_settings_revision CHECK (revision > 0)
);

INSERT INTO affiliate_program_settings (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS agent_principals (
    agent_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    status VARCHAR(24) NOT NULL DEFAULT 'candidate',
    principal_key_hash VARCHAR(128),
    risk_status VARCHAR(24) NOT NULL DEFAULT 'clear',
    risk_note TEXT NOT NULL DEFAULT '',
    qualified_at TIMESTAMPTZ,
    activated_at TIMESTAMPTZ,
    reviewed_at TIMESTAMPTZ,
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_principal_status CHECK (
        status IN ('candidate', 'pending_review', 'active', 'rejected', 'suspended')
    ),
    CONSTRAINT chk_agent_principal_risk_status CHECK (
        risk_status IN ('clear', 'review', 'blocked')
    ),
    CONSTRAINT chk_agent_principal_activation CHECK (
        status <> 'active' OR activated_at IS NOT NULL
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_principals_principal_key
    ON agent_principals (principal_key_hash)
    WHERE principal_key_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agent_principals_status
    ON agent_principals (status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_principals_risk_status
    ON agent_principals (risk_status, updated_at DESC);

CREATE TABLE IF NOT EXISTS affiliate_links (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    code VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    channel VARCHAR(100) NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    current_rate_version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_link_code CHECK (BTRIM(code) <> ''),
    CONSTRAINT chk_affiliate_link_name CHECK (BTRIM(name) <> ''),
    CONSTRAINT chk_affiliate_link_status CHECK (status IN ('active', 'paused')),
    CONSTRAINT chk_affiliate_link_rate_version CHECK (current_rate_version > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_affiliate_links_default_agent
    ON affiliate_links (agent_id)
    WHERE is_default = TRUE;
CREATE INDEX IF NOT EXISTS idx_affiliate_links_agent_status
    ON affiliate_links (agent_id, status, id);

CREATE TABLE IF NOT EXISTS affiliate_link_rate_versions (
    id BIGSERIAL PRIMARY KEY,
    link_id BIGINT NOT NULL REFERENCES affiliate_links(id) ON DELETE RESTRICT,
    version INTEGER NOT NULL,
    customer_rebate_rate_bps INTEGER NOT NULL,
    agent_commission_rate_bps INTEGER NOT NULL,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    effective_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_affiliate_link_rate_version UNIQUE (link_id, version),
    CONSTRAINT chk_affiliate_link_rate_version_positive CHECK (version > 0),
    CONSTRAINT chk_affiliate_customer_rebate_rate CHECK (
        customer_rebate_rate_bps BETWEEN 0 AND 1000 AND customer_rebate_rate_bps % 100 = 0
    ),
    CONSTRAINT chk_affiliate_agent_commission_rate CHECK (
        agent_commission_rate_bps BETWEEN 0 AND 1000
    ),
    CONSTRAINT chk_affiliate_link_pool_fixed CHECK (
        customer_rebate_rate_bps + agent_commission_rate_bps = 1000
    )
);

CREATE INDEX IF NOT EXISTS idx_affiliate_link_rate_effective
    ON affiliate_link_rate_versions (link_id, effective_at DESC, version DESC);

CREATE TABLE IF NOT EXISTS affiliate_bindings (
    id BIGSERIAL PRIMARY KEY,
    customer_user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE RESTRICT,
    inviter_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    binding_kind VARCHAR(16) NOT NULL,
    agent_id BIGINT REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    affiliate_link_id BIGINT REFERENCES affiliate_links(id) ON DELETE RESTRICT,
    link_rate_version INTEGER,
    customer_rebate_rate_snapshot_bps INTEGER NOT NULL DEFAULT 0,
    agent_commission_rate_snapshot_bps INTEGER NOT NULL DEFAULT 0,
    bound_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_binding_not_self CHECK (customer_user_id <> inviter_user_id),
    CONSTRAINT chk_affiliate_binding_kind CHECK (binding_kind IN ('ordinary', 'agent')),
    CONSTRAINT chk_affiliate_binding_rate CHECK (
        customer_rebate_rate_snapshot_bps BETWEEN 0 AND 1000 AND
        agent_commission_rate_snapshot_bps BETWEEN 0 AND 1000
    ),
    CONSTRAINT chk_affiliate_agent_binding_shape CHECK (
        (binding_kind = 'ordinary' AND agent_id IS NULL AND affiliate_link_id IS NULL AND link_rate_version IS NULL) OR
        (binding_kind = 'agent' AND agent_id IS NOT NULL AND affiliate_link_id IS NOT NULL AND link_rate_version IS NOT NULL)
    ),
    CONSTRAINT chk_affiliate_agent_binding_pool CHECK (
        binding_kind <> 'agent' OR
        customer_rebate_rate_snapshot_bps + agent_commission_rate_snapshot_bps = 1000
    )
);

CREATE INDEX IF NOT EXISTS idx_affiliate_bindings_inviter
    ON affiliate_bindings (inviter_user_id, bound_at DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_bindings_agent
    ON affiliate_bindings (agent_id, bound_at DESC)
    WHERE agent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_affiliate_bindings_link
    ON affiliate_bindings (affiliate_link_id, bound_at DESC)
    WHERE affiliate_link_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS affiliate_reward_entries (
    id BIGSERIAL PRIMARY KEY,
    beneficiary_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    consumer_user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reward_type VARCHAR(32) NOT NULL,
    asset_type VARCHAR(24) NOT NULL DEFAULT 'platform_credit',
    amount_micros BIGINT NOT NULL,
    source_amount_micros BIGINT NOT NULL,
    rate_bps INTEGER,
    status VARCHAR(16) NOT NULL DEFAULT 'posted',
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT,
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    reversal_of_id BIGINT REFERENCES affiliate_reward_entries(id) ON DELETE RESTRICT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_reward_type CHECK (
        reward_type IN ('ordinary_referral', 'first_paid_bonus', 'customer_rebate', 'reversal')
    ),
    CONSTRAINT chk_affiliate_reward_asset CHECK (asset_type = 'platform_credit'),
    CONSTRAINT chk_affiliate_reward_amount CHECK (amount_micros > 0),
    CONSTRAINT chk_affiliate_reward_source_amount CHECK (source_amount_micros >= 0),
    CONSTRAINT chk_affiliate_reward_rate CHECK (rate_bps IS NULL OR rate_bps BETWEEN 0 AND 10000),
    CONSTRAINT chk_affiliate_reward_status CHECK (status IN ('pending', 'posted', 'risk_hold', 'reversed')),
    CONSTRAINT chk_affiliate_reward_metadata CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_affiliate_rewards_beneficiary
    ON affiliate_reward_entries (beneficiary_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_rewards_pending
    ON affiliate_reward_entries (available_at, id)
    WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_affiliate_rewards_consumer
    ON affiliate_reward_entries (consumer_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS affiliate_performance_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    direct_agent_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    event_type VARCHAR(32) NOT NULL,
    amount_micros BIGINT NOT NULL DEFAULT 0,
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT,
    event_key VARCHAR(180) NOT NULL UNIQUE,
    occurred_at TIMESTAMPTZ NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_performance_event_type CHECK (
        event_type IN ('confirmed_consumption', 'consumption_reversal', 'binding_created', 'agent_activated')
    ),
    CONSTRAINT chk_affiliate_performance_amount CHECK (amount_micros >= 0),
    CONSTRAINT chk_affiliate_performance_metadata CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_affiliate_performance_user_time
    ON affiliate_performance_events (user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_affiliate_performance_agent_time
    ON affiliate_performance_events (direct_agent_id, occurred_at DESC)
    WHERE direct_agent_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS affiliate_qualification_states (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE RESTRICT,
    direct_valid_consumer_count INTEGER NOT NULL DEFAULT 0,
    direct_team_consumption_micros BIGINT NOT NULL DEFAULT 0,
    self_consumption_micros BIGINT NOT NULL DEFAULT 0,
    combined_consumption_micros BIGINT NOT NULL DEFAULT 0,
    qualifying_route VARCHAR(16),
    status VARCHAR(24) NOT NULL DEFAULT 'tracking',
    qualified_at TIMESTAMPTZ,
    evaluated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_qualification_counts CHECK (
        direct_valid_consumer_count >= 0 AND
        direct_team_consumption_micros >= 0 AND
        self_consumption_micros >= 0 AND
        combined_consumption_micros >= 0
    ),
    CONSTRAINT chk_affiliate_qualification_route CHECK (
        qualifying_route IS NULL OR qualifying_route IN ('direct_team', 'combined')
    ),
    CONSTRAINT chk_affiliate_qualification_status CHECK (
        status IN ('tracking', 'qualified', 'candidate', 'active', 'rejected', 'suspended')
    ),
    CONSTRAINT chk_affiliate_qualification_time CHECK (
        status = 'tracking' OR qualified_at IS NOT NULL
    )
);

CREATE INDEX IF NOT EXISTS idx_affiliate_qualification_status
    ON affiliate_qualification_states (status, evaluated_at DESC);

CREATE TABLE IF NOT EXISTS agent_cash_commission_entries (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL REFERENCES agent_principals(agent_id) ON DELETE RESTRICT,
    consumer_user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    entry_type VARCHAR(32) NOT NULL,
    amount_micros BIGINT NOT NULL,
    source_amount_micros BIGINT,
    customer_rebate_rate_bps INTEGER,
    agent_commission_rate_bps INTEGER,
    posting_status VARCHAR(16) NOT NULL DEFAULT 'posted',
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT,
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    related_entry_id BIGINT REFERENCES agent_cash_commission_entries(id) ON DELETE RESTRICT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_cash_entry_type CHECK (
        entry_type IN ('earned', 'withdrawal_hold', 'withdrawal_release', 'conversion', 'reversal', 'risk_release')
    ),
    CONSTRAINT chk_agent_cash_entry_amount CHECK (amount_micros <> 0),
    CONSTRAINT chk_agent_cash_entry_direction CHECK (
        (entry_type IN ('earned', 'withdrawal_release', 'risk_release') AND amount_micros > 0) OR
        (entry_type IN ('withdrawal_hold', 'conversion', 'reversal') AND amount_micros < 0)
    ),
    CONSTRAINT chk_agent_cash_source_amount CHECK (source_amount_micros IS NULL OR source_amount_micros >= 0),
    CONSTRAINT chk_agent_cash_rates CHECK (
        (customer_rebate_rate_bps IS NULL OR customer_rebate_rate_bps BETWEEN 0 AND 1000) AND
        (agent_commission_rate_bps IS NULL OR agent_commission_rate_bps BETWEEN 0 AND 1000)
    ),
    CONSTRAINT chk_agent_cash_pool CHECK (
        customer_rebate_rate_bps IS NULL OR agent_commission_rate_bps IS NULL OR
        customer_rebate_rate_bps + agent_commission_rate_bps = 1000
    ),
    CONSTRAINT chk_agent_cash_posting_status CHECK (posting_status IN ('posted', 'risk_hold', 'reversed')),
    CONSTRAINT chk_agent_cash_metadata CHECK (jsonb_typeof(metadata) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_agent_cash_commission_agent_time
    ON agent_cash_commission_entries (agent_id, occurred_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_agent_cash_commission_consumer
    ON agent_cash_commission_entries (consumer_user_id, occurred_at DESC)
    WHERE consumer_user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agent_cash_commission_risk
    ON agent_cash_commission_entries (agent_id, occurred_at DESC)
    WHERE posting_status = 'risk_hold';

COMMENT ON TABLE affiliate_program_settings IS 'Affiliate V2 singleton settings; defaults to off';
COMMENT ON TABLE affiliate_reward_entries IS 'Affiliate V2 platform-credit reward ledger, separate from legacy commission_records';
COMMENT ON TABLE agent_cash_commission_entries IS 'Affiliate V2 fixed-point cash commission ledger, separate from legacy agent_settlements';
