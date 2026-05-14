ALTER TABLE users DROP COLUMN IF EXISTS balance_notify_enabled;
ALTER TABLE users DROP COLUMN IF EXISTS balance_notify_threshold_type;
ALTER TABLE users DROP COLUMN IF EXISTS balance_notify_threshold;
ALTER TABLE users DROP COLUMN IF EXISTS balance_notify_extra_emails;

DELETE FROM settings
WHERE key IN (
  'balance_low_notify_enabled',
  'balance_low_notify_threshold',
  'balance_low_notify_recharge_url'
);
