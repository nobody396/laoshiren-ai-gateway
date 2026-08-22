ALTER TABLE channel_model_pricing
    ADD COLUMN IF NOT EXISTS fast_multiplier NUMERIC(12,6),
    ADD COLUMN IF NOT EXISTS flex_multiplier NUMERIC(12,6);

ALTER TABLE channel_account_stats_model_pricing
    ADD COLUMN IF NOT EXISTS fast_multiplier NUMERIC(12,6),
    ADD COLUMN IF NOT EXISTS flex_multiplier NUMERIC(12,6);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'channel_model_pricing_fast_multiplier_positive'
          AND conrelid = 'channel_model_pricing'::regclass
    ) THEN
        ALTER TABLE channel_model_pricing
            ADD CONSTRAINT channel_model_pricing_fast_multiplier_positive
            CHECK (fast_multiplier IS NULL OR (fast_multiplier > 0 AND fast_multiplier::text NOT IN ('NaN', 'Infinity', '-Infinity')));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'channel_model_pricing_flex_multiplier_positive'
          AND conrelid = 'channel_model_pricing'::regclass
    ) THEN
        ALTER TABLE channel_model_pricing
            ADD CONSTRAINT channel_model_pricing_flex_multiplier_positive
            CHECK (flex_multiplier IS NULL OR (flex_multiplier > 0 AND flex_multiplier::text NOT IN ('NaN', 'Infinity', '-Infinity')));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'channel_account_stats_model_pricing_fast_multiplier_positive'
          AND conrelid = 'channel_account_stats_model_pricing'::regclass
    ) THEN
        ALTER TABLE channel_account_stats_model_pricing
            ADD CONSTRAINT channel_account_stats_model_pricing_fast_multiplier_positive
            CHECK (fast_multiplier IS NULL OR (fast_multiplier > 0 AND fast_multiplier::text NOT IN ('NaN', 'Infinity', '-Infinity')));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'channel_account_stats_model_pricing_flex_multiplier_positive'
          AND conrelid = 'channel_account_stats_model_pricing'::regclass
    ) THEN
        ALTER TABLE channel_account_stats_model_pricing
            ADD CONSTRAINT channel_account_stats_model_pricing_flex_multiplier_positive
            CHECK (flex_multiplier IS NULL OR (flex_multiplier > 0 AND flex_multiplier::text NOT IN ('NaN', 'Infinity', '-Infinity')));
    END IF;
END $$;

COMMENT ON COLUMN channel_model_pricing.fast_multiplier IS
    'Fast/Priority service-tier multiplier applied to the selected standard channel price';
COMMENT ON COLUMN channel_model_pricing.flex_multiplier IS
    'Flex service-tier multiplier applied to the selected standard channel price';
COMMENT ON COLUMN channel_account_stats_model_pricing.fast_multiplier IS
    'Fast/Priority multiplier used when calculating supplier account statistics';
COMMENT ON COLUMN channel_account_stats_model_pricing.flex_multiplier IS
    'Flex multiplier used when calculating supplier account statistics';
