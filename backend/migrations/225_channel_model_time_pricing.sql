SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS time_pricing JSONB NULL;

COMMENT ON COLUMN channel_model_pricing.time_pricing IS
    'Optional IANA timezone and non-overlapping recurring daily token-price multiplier periods; null means disabled';
