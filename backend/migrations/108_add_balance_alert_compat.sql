-- 108_add_balance_alert_compat.sql
-- Restore feat/blance_alert compatibility on top of current schema.
-- All operations are idempotent.

INSERT INTO user_attribute_definitions (key, name, description, type, options, required, validation, placeholder, display_order, enabled, created_at, updated_at)
VALUES ('balance_alert_enabled', '余额预警开关', '开启后余额不足时自动发送邮件通知', 'select',
  '[{"value":"true","label":"开启"},{"value":"false","label":"关闭"}]'::jsonb,
  false, '{}'::jsonb, '', 100, true, NOW(), NOW())
ON CONFLICT (key) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO user_attribute_definitions (key, name, description, type, options, required, validation, placeholder, display_order, enabled, created_at, updated_at)
VALUES ('balance_alert_threshold', '预警阈值 (USD)', '余额低于此值时触发预警, 留空使用系统默认值', 'number',
  '[]'::jsonb,
  false, '{"min": 0.1}'::jsonb, '使用系统默认值', 101, true, NOW(), NOW())
ON CONFLICT (key) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO user_attribute_definitions (key, name, description, type, options, required, validation, placeholder, display_order, enabled, created_at, updated_at)
VALUES ('balance_alert_email', '预警接收邮箱', '留空则使用登录账号邮箱', 'email',
  '[]'::jsonb,
  false, '{}'::jsonb, '使用登录邮箱', 102, true, NOW(), NOW())
ON CONFLICT (key) WHERE deleted_at IS NULL DO NOTHING;

INSERT INTO settings (key, value, updated_at)
VALUES ('balance_alert_enabled', 'true', NOW())
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value, updated_at)
VALUES ('balance_alert_default_threshold', '5.00', NOW())
ON CONFLICT (key) DO NOTHING;
