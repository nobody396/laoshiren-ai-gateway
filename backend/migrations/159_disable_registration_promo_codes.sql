-- Disable registration promo codes for the affiliate rollout.
-- This keeps the historical promo-code tables and admin page intact, but hides
-- and ignores the public registration promo-code field unless an operator
-- explicitly re-enables the feature later.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '1min';

INSERT INTO settings (key, value, updated_at)
VALUES ('promo_code_enabled', 'false', NOW())
ON CONFLICT (key) DO UPDATE SET
    value = 'false',
    updated_at = NOW();
