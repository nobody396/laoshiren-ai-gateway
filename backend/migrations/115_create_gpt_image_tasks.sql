CREATE TABLE IF NOT EXISTS gpt_image_tasks (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(128) NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    model VARCHAR(128) NOT NULL DEFAULT 'gpt-image-2',
    upstream_model VARCHAR(128) NOT NULL DEFAULT 'gpt-image-2',
    resolution VARCHAR(8) NOT NULL DEFAULT '1K',
    size VARCHAR(32) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'submitted',
    storage_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    billing_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    s3_object_keys JSONB NOT NULL DEFAULT '[]'::jsonb,
    media_token VARCHAR(96) NOT NULL DEFAULT '',
    image_count INTEGER NOT NULL DEFAULT 1,
    inbound_endpoint VARCHAR(255) NOT NULL DEFAULT '',
    upstream_endpoint VARCHAR(255) NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    ip_address VARCHAR(64) NOT NULL DEFAULT '',
    request_payload_hash VARCHAR(128) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    billed_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    failed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gpt_image_tasks_user_created ON gpt_image_tasks(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_gpt_image_tasks_api_key_created ON gpt_image_tasks(api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_gpt_image_tasks_account_created ON gpt_image_tasks(account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_gpt_image_tasks_storage_status ON gpt_image_tasks(storage_status);
CREATE INDEX IF NOT EXISTS idx_gpt_image_tasks_billing_status ON gpt_image_tasks(billing_status);
