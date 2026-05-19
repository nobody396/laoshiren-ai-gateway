INSERT INTO settings (`key`, value, updated_at)
VALUES ('group_cache_hit_rate_enabled', 'false', NOW())
ON DUPLICATE KEY UPDATE `key` = `key`;
