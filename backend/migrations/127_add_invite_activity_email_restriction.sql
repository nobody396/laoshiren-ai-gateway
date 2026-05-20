-- 127_add_invite_activity_email_restriction.sql
-- 限时邀请活动：活动期注册邮箱域名限制。

ALTER TABLE agent_commission_settings
    ADD COLUMN IF NOT EXISTS invite_activity_email_restriction_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS invite_activity_email_suffix_whitelist JSONB NOT NULL DEFAULT '[]'::jsonb;
