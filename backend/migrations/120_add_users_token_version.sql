-- Persist user token version so password changes and session revocation invalidate old tokens.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS token_version BIGINT NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_token_version_nonnegative'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_token_version_nonnegative
            CHECK (token_version >= 0)
            NOT VALID;
    END IF;
END $$;

ALTER TABLE users
    VALIDATE CONSTRAINT users_token_version_nonnegative;
