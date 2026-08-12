-- Keep text and image observations in separate route identities. Historical
-- rows predate the dimension and remain explicitly unknown instead of being
-- mislabeled as text.

ALTER TABLE openai_route_shadow_decisions
  ADD COLUMN IF NOT EXISTS request_class VARCHAR(16) NOT NULL DEFAULT 'unknown';

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_group_model_class_created
  ON openai_route_shadow_decisions (group_id, model, request_class, created_at DESC);

COMMENT ON COLUMN openai_route_shadow_decisions.request_class IS
  'Semantic request class used by routing, currently text or image; unknown for historical rows.';
