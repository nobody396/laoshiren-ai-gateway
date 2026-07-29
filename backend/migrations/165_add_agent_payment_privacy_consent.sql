-- Record explicit consent for sensitive Alipay payout information.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

ALTER TABLE agent_payment_profiles
    ADD COLUMN IF NOT EXISTS privacy_consent_version VARCHAR(80) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS privacy_consented_at TIMESTAMPTZ;

-- A legacy pending profile has not accepted the new notice and must be
-- resubmitted before it can enter the review queue again.
UPDATE agent_payment_profiles
SET verification_status = 'incomplete',
    verification_note = '',
    verified_at = NULL,
    verified_by = NULL,
    updated_at = NOW()
WHERE verification_status = 'pending_review'
  AND (
      BTRIM(privacy_consent_version) = ''
      OR privacy_consented_at IS NULL
  );

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_agent_payment_privacy_consent'
          AND conrelid = 'agent_payment_profiles'::regclass
    ) THEN
        ALTER TABLE agent_payment_profiles
            ADD CONSTRAINT chk_agent_payment_privacy_consent
            CHECK (
                (
                    BTRIM(privacy_consent_version) = ''
                    AND privacy_consented_at IS NULL
                )
                OR (
                    BTRIM(privacy_consent_version) <> ''
                    AND privacy_consented_at IS NOT NULL
                )
            );
    END IF;
END
$$;

COMMENT ON COLUMN agent_payment_profiles.privacy_consent_version IS
    'Version of the separate payout-profile privacy notice accepted by the agent';
COMMENT ON COLUMN agent_payment_profiles.privacy_consented_at IS
    'Server-recorded timestamp of the agent payout-profile privacy consent';
