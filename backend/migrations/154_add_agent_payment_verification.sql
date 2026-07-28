-- Affiliate V2 payment-profile review and one-principal identity guard.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

ALTER TABLE agent_payment_profiles
    ADD COLUMN IF NOT EXISTS identity_fingerprint_hash VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS verification_status VARCHAR(24) NOT NULL DEFAULT 'incomplete',
    ADD COLUMN IF NOT EXISTS verification_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS verified_by BIGINT REFERENCES users(id) ON DELETE SET NULL;

UPDATE agent_payment_profiles
SET verification_status = CASE
        WHEN BTRIM(alipay_real_name) <> ''
         AND BTRIM(alipay_account) <> ''
         AND BTRIM(alipay_qr_object_key) <> ''
        THEN 'pending_review'
        ELSE 'incomplete'
    END,
    verified_at = NULL,
    verified_by = NULL
WHERE verification_status NOT IN ('pending_review', 'verified', 'rejected');

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'chk_agent_payment_verification_status'
          AND conrelid = 'agent_payment_profiles'::regclass
    ) THEN
        ALTER TABLE agent_payment_profiles
            ADD CONSTRAINT chk_agent_payment_verification_status
            CHECK (verification_status IN ('incomplete', 'pending_review', 'verified', 'rejected'));
    END IF;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_agent_payment_verified_identity
    ON agent_payment_profiles (identity_fingerprint_hash)
    WHERE verification_status = 'verified'
      AND identity_fingerprint_hash <> '';

CREATE INDEX IF NOT EXISTS idx_agent_payment_verification_queue
    ON agent_payment_profiles (verification_status, updated_at DESC);
