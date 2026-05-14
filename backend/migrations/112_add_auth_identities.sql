-- 112_add_auth_identities.sql
-- Third-party identity bindings for user profile login/bind management.

CREATE TABLE IF NOT EXISTS auth_identities (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         VARCHAR(50) NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    email            VARCHAR(255) NOT NULL DEFAULT '',
    email_verified   BOOLEAN NOT NULL DEFAULT false,
    display_name     VARCHAR(255) NOT NULL DEFAULT '',
    avatar_url       TEXT NOT NULL DEFAULT '',
    raw_profile      JSONB NOT NULL DEFAULT '{}'::jsonb,
    last_login_at    TIMESTAMPTZ NULL,
    bound_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_auth_identities_provider_subject UNIQUE (provider, provider_user_id)
);

CREATE INDEX IF NOT EXISTS idx_auth_identities_user_provider
    ON auth_identities (user_id, provider);

CREATE INDEX IF NOT EXISTS idx_auth_identities_provider_email
    ON auth_identities (provider, email);
