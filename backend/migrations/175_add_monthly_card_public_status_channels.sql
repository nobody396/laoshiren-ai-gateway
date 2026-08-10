-- Allow operators to choose which monthly-card status channels are public.
-- Keep all channels visible by default so existing installations preserve the
-- behavior of the original global public-status switch.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

INSERT INTO settings (key, value, updated_at)
VALUES ('monthly_card_public_status_channels', '["codex","claude","grok"]', NOW())
ON CONFLICT (key) DO NOTHING;
