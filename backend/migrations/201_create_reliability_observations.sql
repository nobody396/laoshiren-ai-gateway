-- Normalized, append-only reliability evidence. The collector is disabled by
-- default and stores no prompt, response body, credential, account name, or
-- raw upstream URL.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS reliability_observations (
    id BIGSERIAL PRIMARY KEY,
    idempotency_key VARCHAR(180) NOT NULL UNIQUE,
    fact_type VARCHAR(32) NOT NULL,
    source VARCHAR(64) NOT NULL,
    source_id VARCHAR(180) NOT NULL,

    request_id VARCHAR(128) NOT NULL DEFAULT '',
    client_request_id VARCHAR(128) NOT NULL DEFAULT '',
    user_id BIGINT,
    group_id BIGINT,
    account_id BIGINT,

    platform VARCHAR(32) NOT NULL DEFAULT '',
    model VARCHAR(128) NOT NULL DEFAULT '',
    request_class VARCHAR(16) NOT NULL DEFAULT 'other',
    protocol VARCHAR(32) NOT NULL DEFAULT '',
    endpoint_hash VARCHAR(16) NOT NULL DEFAULT '',
    route_fingerprint VARCHAR(32) NOT NULL DEFAULT '',

    outcome VARCHAR(16) NOT NULL,
    status_code INTEGER,
    error_owner VARCHAR(32) NOT NULL DEFAULT '',
    exclusion_reason VARCHAR(64) NOT NULL DEFAULT '',
    customer_impact BOOLEAN NOT NULL DEFAULT FALSE,
    latency_ms BIGINT NOT NULL DEFAULT 0,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT reliability_observation_fact_type_check CHECK (
        fact_type IN ('customer_request', 'upstream_attempt', 'active_probe')
    ),
    CONSTRAINT reliability_observation_outcome_check CHECK (
        outcome IN ('success', 'failure', 'recovered', 'excluded')
    ),
    CONSTRAINT reliability_observation_request_class_check CHECK (
        request_class IN ('text', 'image', 'video', 'other')
    ),
    CONSTRAINT reliability_observation_status_code_check CHECK (
        status_code IS NULL OR status_code BETWEEN 100 AND 599
    ),
    CONSTRAINT reliability_observation_route_fingerprint_check CHECK (
        route_fingerprint = '' OR route_fingerprint ~ '^[0-9a-f]{32}$'
    ),
    CONSTRAINT reliability_observation_endpoint_hash_check CHECK (
        endpoint_hash = '' OR endpoint_hash ~ '^[0-9a-f]{16}$'
    ),
    CONSTRAINT reliability_observation_latency_check CHECK (latency_ms >= 0),
    CONSTRAINT reliability_observation_customer_impact_check CHECK (
        NOT customer_impact OR (
            fact_type = 'customer_request'
            AND outcome = 'failure'
            AND error_owner IN ('provider', 'platform')
        )
    )
);

CREATE INDEX IF NOT EXISTS idx_reliability_observations_observed
    ON reliability_observations (observed_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_reliability_observations_fact_observed
    ON reliability_observations (fact_type, observed_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_reliability_observations_group_model_observed
    ON reliability_observations (group_id, model, observed_at DESC, id DESC)
    WHERE group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_reliability_observations_route_observed
    ON reliability_observations (route_fingerprint, observed_at DESC, id DESC)
    WHERE route_fingerprint <> '';
CREATE INDEX IF NOT EXISTS idx_reliability_observations_customer_impact
    ON reliability_observations (user_id, observed_at DESC, id DESC)
    WHERE customer_impact = TRUE;

CREATE TABLE IF NOT EXISTS reliability_probe_claims (
    claim_key VARCHAR(180) PRIMARY KEY,
    route_fingerprint VARCHAR(32) NOT NULL,
    interval_start TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reliability_probe_claim_route_check CHECK (route_fingerprint ~ '^[0-9a-f]{32}$'),
    CONSTRAINT reliability_probe_claim_window_check CHECK (expires_at > interval_start),
    CONSTRAINT reliability_probe_claim_route_interval_key UNIQUE (route_fingerprint, interval_start)
);

CREATE INDEX IF NOT EXISTS idx_reliability_probe_claims_expires
    ON reliability_probe_claims (expires_at);

INSERT INTO settings (key, value, updated_at)
VALUES ('reliability_observation_enabled', 'false', NOW())
ON CONFLICT (key) DO NOTHING;

COMMENT ON TABLE reliability_observations IS
    'Append-only normalized facts for Service Status, Channel Monitoring, routing Shadow evidence, Incidents, and compensation.';
