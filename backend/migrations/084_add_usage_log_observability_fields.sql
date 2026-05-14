-- Add usage log observability/account-cost fields aligned with upstream usage tracking.

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS channel_id BIGINT;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS model_mapping_chain VARCHAR(500);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS billing_tier VARCHAR(50);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS billing_mode VARCHAR(20);
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS image_output_tokens INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS image_output_cost DECIMAL(20, 10) NOT NULL DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS account_stats_cost DECIMAL(20, 10);
