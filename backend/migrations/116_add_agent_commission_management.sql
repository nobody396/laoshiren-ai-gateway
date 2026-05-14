-- 116_add_agent_commission_management.sql
-- 代理商管理：分佣比例配置、佣金比例快照、结算账本。

ALTER TABLE commission_records ADD COLUMN IF NOT EXISTS rate DECIMAL(10,6);
ALTER TABLE commission_records ADD COLUMN IF NOT EXISTS rate_source VARCHAR(32);

CREATE TABLE IF NOT EXISTS agent_commission_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    consumption_rate DECIMAL(10,6) NOT NULL DEFAULT 0.060000,
    first_recharge_invitee_rate DECIMAL(10,6) NOT NULL DEFAULT 0.100000,
    first_recharge_referral_rate DECIMAL(10,6) NOT NULL DEFAULT 0.050000,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO agent_commission_settings (
    id,
    consumption_rate,
    first_recharge_invitee_rate,
    first_recharge_referral_rate
) VALUES (1, 0.060000, 0.100000, 0.050000)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS agent_rate_configs (
    agent_id BIGINT PRIMARY KEY,
    consumption_rate DECIMAL(10,6) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agent_settlements (
    id BIGSERIAL PRIMARY KEY,
    agent_id BIGINT NOT NULL,
    amount DECIMAL(20,8) NOT NULL CHECK (amount > 0),
    operator_id BIGINT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'completed',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS agent_settlements_agent_id_idx
    ON agent_settlements(agent_id);

CREATE INDEX IF NOT EXISTS agent_settlements_agent_status_idx
    ON agent_settlements(agent_id, status);

CREATE INDEX IF NOT EXISTS agent_settlements_created_at_idx
    ON agent_settlements(created_at);
