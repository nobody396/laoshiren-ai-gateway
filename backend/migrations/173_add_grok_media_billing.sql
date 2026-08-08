-- Add Grok media permission/pricing controls and per-request billing metadata.
-- Additive only: the currently deployed application safely ignores these columns.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS allow_image_generation BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS image_rate_independent BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS image_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS video_rate_independent BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS video_rate_multiplier DECIMAL(10,4) NOT NULL DEFAULT 1.0,
    ADD COLUMN IF NOT EXISTS video_price_480p DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS video_price_720p DECIMAL(20,8),
    ADD COLUMN IF NOT EXISTS video_price_1080p DECIMAL(20,8);

-- Existing image-capable groups must keep working after the new permission gate is introduced.
UPDATE groups
SET allow_image_generation = true
WHERE platform IN ('openai', 'gemini', 'antigravity', 'gpt-image', 'grok');

COMMENT ON COLUMN groups.allow_image_generation IS '是否允许该分组使用图片/媒体生成能力；Grok 图片和视频共用';
COMMENT ON COLUMN groups.image_rate_independent IS '图片生成是否使用独立倍率；false 表示共享分组有效倍率';
COMMENT ON COLUMN groups.image_rate_multiplier IS '图片生成独立倍率，仅 image_rate_independent=true 时生效';
COMMENT ON COLUMN groups.video_rate_independent IS '视频生成是否使用独立倍率；false 表示共享分组有效倍率';
COMMENT ON COLUMN groups.video_rate_multiplier IS '视频生成独立倍率，仅 video_rate_independent=true 时生效';
COMMENT ON COLUMN groups.video_price_480p IS '480p 视频生成每秒单价 (USD/s)，Grok 平台使用';
COMMENT ON COLUMN groups.video_price_720p IS '720p 视频生成每秒单价 (USD/s)，Grok 平台使用';
COMMENT ON COLUMN groups.video_price_1080p IS '1080p 视频生成每秒单价 (USD/s)，Grok 平台使用';

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS video_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS video_resolution VARCHAR(10),
    ADD COLUMN IF NOT EXISTS video_duration_seconds INTEGER;

COMMENT ON COLUMN usage_logs.video_count IS '视频生成数量；大于 0 表示本行是视频生成用量';
COMMENT ON COLUMN usage_logs.video_resolution IS '计费用视频分辨率 480p/720p/1080p';
COMMENT ON COLUMN usage_logs.video_duration_seconds IS '提交时请求的视频时长（秒），按秒计费的乘数';
