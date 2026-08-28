SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE channel_account_stats_model_pricing
    ADD COLUMN IF NOT EXISTS cost_multiplier NUMERIC(20,12) NULL;

ALTER TABLE channel_account_stats_model_pricing
    DROP CONSTRAINT IF EXISTS chk_account_stats_model_cost_multiplier;

ALTER TABLE channel_account_stats_model_pricing
    ADD CONSTRAINT chk_account_stats_model_cost_multiplier
    CHECK (cost_multiplier IS NULL OR cost_multiplier > 0);

COMMENT ON COLUMN channel_account_stats_model_pricing.cost_multiplier IS
    'Optional supplier cost multiplier applied to the settled pre-group customer base cost; reuses identical context and request-start time pricing selection';
