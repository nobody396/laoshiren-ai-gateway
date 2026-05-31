-- Move suppliers created with the old default probe model to the cheaper Haiku probe.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

UPDATE suppliers
SET probe_model = 'claude-haiku-4-5-20251001'
WHERE probe_model = 'gpt-5.1-codex-mini';
