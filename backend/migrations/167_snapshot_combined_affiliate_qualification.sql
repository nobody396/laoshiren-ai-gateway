-- Preserve the complete qualification calculation on every partner
-- application. Route B uses self + direct confirmed consumption, so storing
-- only the direct amount is insufficient for review and audit.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

ALTER TABLE affiliate_agent_applications
    ADD COLUMN IF NOT EXISTS self_consumption_micros BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS combined_consumption_micros BIGINT NOT NULL DEFAULT 0;

-- Historical applications predate the combined snapshot. Preserve their known
-- direct amount and use zero self amount rather than inventing history.
UPDATE affiliate_agent_applications
SET
    self_consumption_micros = 0,
    combined_consumption_micros = direct_team_consumption_micros
WHERE combined_consumption_micros = 0
  AND direct_team_consumption_micros > 0;

ALTER TABLE affiliate_agent_applications
    DROP CONSTRAINT IF EXISTS chk_affiliate_application_totals;

ALTER TABLE affiliate_agent_applications
    ADD CONSTRAINT chk_affiliate_application_totals CHECK (
        direct_valid_consumer_count >= 0
        AND direct_team_consumption_micros >= 0
        AND self_consumption_micros >= 0
        AND combined_consumption_micros =
            self_consumption_micros + direct_team_consumption_micros
    );

COMMENT ON COLUMN affiliate_agent_applications.combined_consumption_micros IS
    'Immutable application snapshot: self plus direct confirmed consumption';
