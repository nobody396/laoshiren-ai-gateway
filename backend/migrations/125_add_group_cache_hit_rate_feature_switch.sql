INSERT INTO settings (key, value, updated_at)
VALUES ('group_cache_hit_rate_enabled', 'false', NOW())
ON CONFLICT (key) DO NOTHING;
