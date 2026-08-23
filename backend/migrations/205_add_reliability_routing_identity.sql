-- Add the exact, non-sensitive route dimensions required to safely reuse
-- Reliability Evidence in adaptive-routing Shadow treatments. Existing rows
-- remain valid but deliberately cannot match until a producer supplies the
-- new identity.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

ALTER TABLE reliability_observations
    ADD COLUMN IF NOT EXISTS access_group_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS transport VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS routing_fingerprint VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE reliability_observations
    DROP CONSTRAINT IF EXISTS reliability_observation_access_group_check,
    ADD CONSTRAINT reliability_observation_access_group_check CHECK (access_group_id >= 0),
    DROP CONSTRAINT IF EXISTS reliability_observation_routing_fingerprint_check,
    ADD CONSTRAINT reliability_observation_routing_fingerprint_check CHECK (
        routing_fingerprint = '' OR routing_fingerprint ~ '^[0-9a-f]{32}$'
    );

CREATE INDEX IF NOT EXISTS idx_reliability_observations_routing_scope_observed
    ON reliability_observations (
        routing_fingerprint, access_group_id, protocol, observed_at DESC, id DESC
    )
    WHERE routing_fingerprint <> '';

CREATE INDEX IF NOT EXISTS idx_reliability_observations_probe_route_observed
    ON reliability_observations (
        account_id, endpoint_hash, transport, protocol, observed_at DESC, id DESC
    )
    WHERE fact_type = 'active_probe' AND group_id IS NULL;
