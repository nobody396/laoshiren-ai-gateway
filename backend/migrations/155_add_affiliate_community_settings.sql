-- Affiliate V2 private agent-community onboarding card.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS affiliate_community_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    title VARCHAR(100) NOT NULL DEFAULT '代理商社群',
    message TEXT NOT NULL DEFAULT '扫码加入代理商社群，获取运营支持与最新通知。',
    qr_object_key TEXT NOT NULL DEFAULT '',
    qr_content_type VARCHAR(64) NOT NULL DEFAULT '',
    qr_original_filename TEXT NOT NULL DEFAULT '',
    qr_size BIGINT NOT NULL DEFAULT 0,
    revision BIGINT NOT NULL DEFAULT 1,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_affiliate_community_title CHECK (BTRIM(title) <> ''),
    CONSTRAINT chk_affiliate_community_qr_size CHECK (qr_size >= 0),
    CONSTRAINT chk_affiliate_community_revision CHECK (revision > 0)
);

INSERT INTO affiliate_community_settings (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;
