-- Control whether monthly-card runtime status is shown to regular users.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

INSERT INTO settings (key, value, updated_at)
VALUES ('monthly_card_public_status_enabled', 'false', NOW())
ON CONFLICT (key) DO NOTHING;
