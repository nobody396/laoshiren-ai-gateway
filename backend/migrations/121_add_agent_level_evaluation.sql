-- 121_add_agent_level_evaluation.sql
-- 代理等级规则与月度自动评档状态。

CREATE TABLE IF NOT EXISTS agent_level_rules (
    level_key VARCHAR(32) PRIMARY KEY,
    level_name VARCHAR(64) NOT NULL,
    rate DECIMAL(10,6) NOT NULL CHECK (rate >= 0 AND rate <= 0.200000),
    monthly_consumption_threshold DECIMAL(20,8),
    cumulative_consumption_threshold DECIMAL(20,8),
    sort_order INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (monthly_consumption_threshold IS NULL OR monthly_consumption_threshold >= 0),
    CHECK (cumulative_consumption_threshold IS NULL OR cumulative_consumption_threshold >= 0)
);

INSERT INTO agent_level_rules (
    level_key,
    level_name,
    rate,
    monthly_consumption_threshold,
    cumulative_consumption_threshold,
    sort_order,
    enabled
) VALUES
    ('light', '轻代理', 0.050000, NULL, NULL, 1, true),
    ('standard', '标准代理', 0.100000, NULL, 1000.00000000, 2, true),
    ('core', '核心代理', 0.150000, 1500.00000000, 10000.00000000, 3, true),
    ('super', '超级代理', 0.200000, 3000.00000000, 50000.00000000, 4, true)
ON CONFLICT (level_key) DO NOTHING;

CREATE TABLE IF NOT EXISTS agent_level_states (
    agent_id BIGINT PRIMARY KEY,
    base_level_key VARCHAR(32) NOT NULL DEFAULT 'light',
    base_rate DECIMAL(10,6) NOT NULL DEFAULT 0.050000,
    permanent_level_key VARCHAR(32) NOT NULL DEFAULT 'light',
    temporary_level_key VARCHAR(32),
    current_level_key VARCHAR(32) NOT NULL DEFAULT 'light',
    current_rate DECIMAL(10,6) NOT NULL DEFAULT 0.050000,
    rate_source VARCHAR(32) NOT NULL DEFAULT 'agent_level',
    last_evaluated_period VARCHAR(7) NOT NULL DEFAULT '',
    last_month_consumption DECIMAL(20,8) NOT NULL DEFAULT 0,
    total_consumption DECIMAL(20,8) NOT NULL DEFAULT 0,
    next_level_key VARCHAR(32),
    next_level_gap DECIMAL(20,8) NOT NULL DEFAULT 0,
    evaluated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (base_rate >= 0 AND base_rate <= 0.200000),
    CHECK (current_rate >= 0 AND current_rate <= 0.200000),
    CHECK (last_month_consumption >= 0),
    CHECK (total_consumption >= 0),
    CHECK (next_level_gap >= 0)
);

CREATE INDEX IF NOT EXISTS agent_level_states_period_idx
    ON agent_level_states(last_evaluated_period);

CREATE INDEX IF NOT EXISTS agent_level_states_current_level_idx
    ON agent_level_states(current_level_key);
