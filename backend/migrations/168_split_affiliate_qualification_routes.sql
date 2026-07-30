-- Align partner qualification with the approved launch rules:
--   Route A: at least 5 direct users, each with >= ¥20 confirmed consumption,
--            and those valid users contribute >= ¥1,000 in total.
--   Route B: the applicant alone contributes >= ¥500 confirmed consumption.
--
-- Keep the legacy combined threshold column for rollback compatibility, but
-- stop using it in the V3 application contract.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

ALTER TABLE affiliate_program_settings
    ADD COLUMN IF NOT EXISTS qualification_self_consumption_micros
        BIGINT NOT NULL DEFAULT 500000000;

ALTER TABLE affiliate_program_settings
    DROP CONSTRAINT IF EXISTS chk_affiliate_self_qualification_value;

ALTER TABLE affiliate_program_settings
    ADD CONSTRAINT chk_affiliate_self_qualification_value CHECK (
        qualification_self_consumption_micros > 0
    );

UPDATE affiliate_program_settings
SET
    qualification_direct_user_count = 5,
    qualification_min_user_consumption_micros = 20000000,
    qualification_direct_team_consumption_micros = 1000000000,
    qualification_self_consumption_micros = 500000000,
    revision = revision + 1,
    updated_at = NOW()
WHERE id = 1
  AND (
      qualification_direct_user_count IS DISTINCT FROM 5
      OR qualification_min_user_consumption_micros IS DISTINCT FROM 20000000
      OR qualification_direct_team_consumption_micros IS DISTINCT FROM 1000000000
      OR qualification_self_consumption_micros IS DISTINCT FROM 500000000
  );

ALTER TABLE affiliate_agent_applications
    DROP CONSTRAINT IF EXISTS chk_affiliate_application_route;

ALTER TABLE affiliate_agent_applications
    ADD CONSTRAINT chk_affiliate_application_route CHECK (
        qualifying_route IN (
            'direct_team',
            'self_consumption',
            'direct_volume'
        )
    );

ALTER TABLE affiliate_qualification_states
    DROP CONSTRAINT IF EXISTS chk_affiliate_qualification_route;

ALTER TABLE affiliate_qualification_states
    ADD CONSTRAINT chk_affiliate_qualification_route CHECK (
        qualifying_route IS NULL
        OR qualifying_route IN (
            'direct_team',
            'self_consumption',
            'direct_volume'
        )
    );

COMMENT ON COLUMN affiliate_program_settings.qualification_self_consumption_micros IS
    'Route B threshold: applicant self confirmed consumption only';
COMMENT ON COLUMN affiliate_program_settings.qualification_combined_consumption_micros IS
    'Deprecated rollback-compatibility field; Route B no longer uses combined consumption';
