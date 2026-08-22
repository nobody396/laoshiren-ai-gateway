-- Preserve the universal access path as durable, non-sensitive Shadow
-- provenance while the concrete target group remains the actual routing and
-- billing group. This migration does not enable Enforce or create policies.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE openai_route_shadow_decisions
  ADD COLUMN IF NOT EXISTS access_group_id BIGINT,
  ADD COLUMN IF NOT EXISTS inbound_protocol VARCHAR(32) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS requested_service_tier VARCHAR(16) NOT NULL DEFAULT '';

ALTER TABLE openai_route_shadow_decisions
  DROP CONSTRAINT IF EXISTS openai_route_shadow_access_group_positive;

ALTER TABLE openai_route_shadow_decisions
  ADD CONSTRAINT openai_route_shadow_access_group_positive
  CHECK (access_group_id IS NULL OR access_group_id > 0);

CREATE INDEX IF NOT EXISTS idx_openai_route_shadow_decisions_access_scope
  ON openai_route_shadow_decisions (
    access_group_id,
    model,
    inbound_protocol,
    requested_service_tier,
    created_at DESC
  )
  WHERE access_group_id IS NOT NULL;

COMMENT ON COLUMN openai_route_shadow_decisions.access_group_id IS
  'Universal API-key access group before rebinding to the concrete routing and billing group; NULL for ordinary keys.';

COMMENT ON COLUMN openai_route_shadow_decisions.inbound_protocol IS
  'Customer-facing universal protocol; empty for ordinary keys.';

COMMENT ON COLUMN openai_route_shadow_decisions.requested_service_tier IS
  'Normalized tier requested by the client; a later policy may still filter it before forwarding and billing.';
