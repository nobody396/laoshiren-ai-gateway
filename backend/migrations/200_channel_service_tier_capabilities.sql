-- A service-tier price is not proof that an upstream accepts the tier. Keep
-- capability admission explicitly false until an operator records provider
-- confirmation. Existing channels therefore remain behaviorally unchanged.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE channel_model_pricing
  ADD COLUMN IF NOT EXISTS fast_supported BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS flex_supported BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS fast_verified_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS flex_verified_at TIMESTAMPTZ;

ALTER TABLE channel_model_pricing
  DROP CONSTRAINT IF EXISTS channel_model_pricing_fast_capability_verified,
  DROP CONSTRAINT IF EXISTS channel_model_pricing_flex_capability_verified;

ALTER TABLE channel_model_pricing
  ADD CONSTRAINT channel_model_pricing_fast_capability_verified
    CHECK (NOT fast_supported OR (fast_verified_at IS NOT NULL AND fast_multiplier IS NOT NULL AND fast_multiplier > 0)),
  ADD CONSTRAINT channel_model_pricing_flex_capability_verified
    CHECK (NOT flex_supported OR (flex_verified_at IS NOT NULL AND flex_multiplier IS NOT NULL AND flex_multiplier > 0));

COMMENT ON COLUMN channel_model_pricing.fast_supported IS
  'True only after the channel provider confirms Fast/Priority support for this model rule; defaults false.';
COMMENT ON COLUMN channel_model_pricing.flex_supported IS
  'True only after the channel provider confirms Flex support for this model rule; defaults false.';
COMMENT ON COLUMN channel_model_pricing.fast_verified_at IS
  'Operator-recorded provider confirmation time for Fast/Priority capability.';
COMMENT ON COLUMN channel_model_pricing.flex_verified_at IS
  'Operator-recorded provider confirmation time for Flex capability.';
