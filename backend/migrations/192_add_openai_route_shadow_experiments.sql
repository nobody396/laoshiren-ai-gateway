-- Separate durable facts from experiment identity. Existing Shadow rows are
-- backfilled into one backward-compatible default variant, so a code deploy
-- does not erase or restart their evidence window.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE openai_route_shadow_decisions
  ADD COLUMN IF NOT EXISTS experiment_id VARCHAR(128) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS variant_id VARCHAR(64) NOT NULL DEFAULT 'default',
  ADD COLUMN IF NOT EXISTS treatment_fingerprint VARCHAR(32) NOT NULL DEFAULT '';

UPDATE openai_route_shadow_decisions
SET
  experiment_id = activation_id,
  variant_id = 'default',
  treatment_fingerprint = md5(COALESCE((snapshot->'policy')::text, '{}'))
WHERE activation_id <> ''
  AND (
    experiment_id = ''
    OR variant_id = ''
    OR treatment_fingerprint = ''
  );

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_experiment_scope
  ON openai_route_shadow_decisions (
    experiment_id,
    variant_id,
    activation_id,
    group_id,
    model,
    request_class,
    created_at DESC
  );

COMMENT ON COLUMN openai_route_shadow_decisions.experiment_id IS
  'Stable logical experiment identity; legacy single-variant policies inherit activation_id.';

COMMENT ON COLUMN openai_route_shadow_decisions.variant_id IS
  'Shadow-only treatment name within an experiment; legacy policies use default.';

COMMENT ON COLUMN openai_route_shadow_decisions.treatment_fingerprint IS
  'MD5 identity of the normalized JSONB policy snapshot, used only for equality and contamination gates.';
