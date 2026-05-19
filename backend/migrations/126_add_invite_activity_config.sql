-- 126_add_invite_activity_config.sql
-- 限时邀请活动：活动期通过邀请链接注册赠送余额。

ALTER TABLE agent_commission_settings
    ADD COLUMN IF NOT EXISTS invite_activity_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS invite_activity_name VARCHAR(128) NOT NULL DEFAULT '公测邀请活动',
    ADD COLUMN IF NOT EXISTS invite_activity_start_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS invite_activity_end_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS invite_activity_registration_bonus DECIMAL(20,8) NOT NULL DEFAULT 5.00000000,
    ADD COLUMN IF NOT EXISTS invite_activity_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

INSERT INTO agent_commission_settings (
    id,
    consumption_rate,
    first_recharge_invitee_rate,
    first_recharge_referral_rate,
    invite_activity_enabled,
    invite_activity_name,
    invite_activity_registration_bonus
) VALUES (
    1,
    0.060000,
    0.100000,
    0.050000,
    FALSE,
    '公测邀请活动',
    5.00000000
) ON CONFLICT (id) DO NOTHING;
