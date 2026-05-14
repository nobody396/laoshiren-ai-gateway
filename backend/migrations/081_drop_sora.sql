-- Migration: 081_drop_sora
-- 移除已废弃的 Sora 相关表和字段

DROP TABLE IF EXISTS sora_generations;
DROP TABLE IF EXISTS sora_accounts;

ALTER TABLE groups
    DROP COLUMN IF EXISTS sora_image_price_360,
    DROP COLUMN IF EXISTS sora_image_price_540,
    DROP COLUMN IF EXISTS sora_video_price_per_request,
    DROP COLUMN IF EXISTS sora_video_price_per_request_hd,
    DROP COLUMN IF EXISTS sora_storage_quota_bytes;

ALTER TABLE users
    DROP COLUMN IF EXISTS sora_storage_quota_bytes,
    DROP COLUMN IF EXISTS sora_storage_used_bytes;
