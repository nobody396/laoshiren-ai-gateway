-- Incident Control: candidates, lifecycle, affected products, customer-impact
-- segments, evidence links, operator updates, public timeline, and audit.
-- All switches remain off by default and no routing or compensation state is
-- mutated by these tables.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS reliability_incident_candidates (
    id BIGSERIAL PRIMARY KEY,
    candidate_key VARCHAR(120) NOT NULL UNIQUE,
    state VARCHAR(16) NOT NULL DEFAULT 'open',
    first_observed_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    last_reconciled_at TIMESTAMPTZ,
    confirmed_incident_id BIGINT,
    dismissed_reason VARCHAR(500) NOT NULL DEFAULT '',
    dismissed_by_user_id BIGINT,
    dismissed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reliability_incident_candidate_state_check CHECK (
        state IN ('open','confirmed','dismissed','recovered')
    ),
    CONSTRAINT reliability_incident_candidate_window_check CHECK (
        last_observed_at >= first_observed_at
    )
);

CREATE INDEX IF NOT EXISTS idx_reliability_incident_candidates_state_time
    ON reliability_incident_candidates (state, last_observed_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS reliability_incident_candidate_products (
    candidate_id BIGINT NOT NULL REFERENCES reliability_incident_candidates(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES service_status_products(id) ON DELETE RESTRICT,
    first_observed_at TIMESTAMPTZ NOT NULL,
    last_observed_at TIMESTAMPTZ NOT NULL,
    latest_status VARCHAR(32) NOT NULL,
    first_customer_failure_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (candidate_id, product_id),
    CONSTRAINT reliability_incident_candidate_product_status_check CHECK (
        latest_status IN ('operational','degraded_performance','partial_outage','major_outage','maintenance','monitoring')
    )
);

CREATE TABLE IF NOT EXISTS reliability_incidents (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(36) NOT NULL UNIQUE,
    candidate_id BIGINT UNIQUE REFERENCES reliability_incident_candidates(id) ON DELETE RESTRICT,
    phase VARCHAR(24) NOT NULL DEFAULT 'investigating',
    title VARCHAR(200) NOT NULL,
    internal_summary TEXT NOT NULL DEFAULT '',
    observation_started_at TIMESTAMPTZ NOT NULL,
    observation_ended_at TIMESTAMPTZ,
    customer_impact_started_at TIMESTAMPTZ,
    customer_impact_ended_at TIMESTAMPTZ,
    monitoring_since TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ,
	last_reconciled_at TIMESTAMPTZ,
	evidence_gap BOOLEAN NOT NULL DEFAULT FALSE,
    created_by_user_id BIGINT,
    updated_by_user_id BIGINT,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reliability_incident_phase_check CHECK (
        phase IN ('investigating','identified','mitigating','monitoring','resolved')
    ),
    CONSTRAINT reliability_incident_observation_window_check CHECK (
        observation_ended_at IS NULL OR observation_ended_at >= observation_started_at
    ),
    CONSTRAINT reliability_incident_impact_window_check CHECK (
        customer_impact_ended_at IS NULL OR customer_impact_started_at IS NOT NULL
    )
);

ALTER TABLE reliability_incident_candidates
    DROP CONSTRAINT IF EXISTS reliability_incident_candidate_confirmed_incident_fkey,
    ADD CONSTRAINT reliability_incident_candidate_confirmed_incident_fkey
        FOREIGN KEY (confirmed_incident_id) REFERENCES reliability_incidents(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_reliability_incidents_phase_updated
    ON reliability_incidents (phase, updated_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS reliability_incident_products (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES service_status_products(id) ON DELETE RESTRICT,
    affected_at TIMESTAMPTZ NOT NULL,
    current_status VARCHAR(32) NOT NULL,
    monitoring_since TIMESTAMPTZ,
    recovered_at TIMESTAMPTZ,
    last_customer_failure_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (incident_id, product_id),
    CONSTRAINT reliability_incident_product_status_check CHECK (
        current_status IN ('operational','degraded_performance','partial_outage','major_outage','maintenance','monitoring')
    )
);

CREATE INDEX IF NOT EXISTS idx_reliability_incident_products_product
    ON reliability_incident_products (product_id, incident_id DESC);

CREATE TABLE IF NOT EXISTS reliability_incident_impact_segments (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE CASCADE,
    incident_product_id BIGINT NOT NULL REFERENCES reliability_incident_products(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ,
    start_observation_id BIGINT REFERENCES reliability_observations(id) ON DELETE RESTRICT,
    end_observation_id BIGINT REFERENCES reliability_observations(id) ON DELETE RESTRICT,
    close_reason VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reliability_incident_segment_window_check CHECK (
        ended_at IS NULL OR ended_at >= started_at
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_reliability_incident_segments_one_open
    ON reliability_incident_impact_segments (incident_product_id)
    WHERE ended_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_reliability_incident_segments_incident_time
    ON reliability_incident_impact_segments (incident_id, started_at, id);

CREATE TABLE IF NOT EXISTS reliability_incident_updates (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE CASCADE,
    phase VARCHAR(24) NOT NULL,
    kind VARCHAR(24) NOT NULL DEFAULT 'operator',
    internal_message TEXT NOT NULL,
    created_by_user_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reliability_incident_update_phase_check CHECK (
        phase IN ('investigating','identified','mitigating','monitoring','resolved')
    ),
    CONSTRAINT reliability_incident_update_kind_check CHECK (
        kind IN ('system','operator','transition')
    )
);

CREATE INDEX IF NOT EXISTS idx_reliability_incident_updates_timeline
    ON reliability_incident_updates (incident_id, created_at, id);

CREATE TABLE IF NOT EXISTS reliability_incident_observation_links (
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE CASCADE,
    observation_id BIGINT NOT NULL REFERENCES reliability_observations(id) ON DELETE RESTRICT,
    product_id BIGINT REFERENCES service_status_products(id) ON DELETE RESTRICT,
    relation VARCHAR(32) NOT NULL,
    linked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (incident_id, observation_id, relation),
    CONSTRAINT reliability_incident_observation_relation_check CHECK (
        relation IN ('customer_impact','status_evidence','recovery')
    )
);

CREATE INDEX IF NOT EXISTS idx_reliability_incident_observation_product
    ON reliability_incident_observation_links (product_id, observation_id)
    WHERE product_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS reliability_incident_public_timeline (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT NOT NULL REFERENCES reliability_incidents(id) ON DELETE CASCADE,
    phase VARCHAR(24) NOT NULL,
    message VARCHAR(1000) NOT NULL,
    published_by_user_id BIGINT,
    published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reliability_incident_public_phase_check CHECK (
        phase IN ('investigating','identified','mitigating','monitoring','resolved')
    )
);

CREATE INDEX IF NOT EXISTS idx_reliability_incident_public_timeline
    ON reliability_incident_public_timeline (incident_id, published_at, id);

CREATE TABLE IF NOT EXISTS reliability_incident_audit_log (
    id BIGSERIAL PRIMARY KEY,
    incident_id BIGINT REFERENCES reliability_incidents(id) ON DELETE CASCADE,
    candidate_id BIGINT REFERENCES reliability_incident_candidates(id) ON DELETE CASCADE,
    action VARCHAR(64) NOT NULL,
    actor_user_id BIGINT,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    before_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    after_state JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT reliability_incident_audit_target_check CHECK (
        incident_id IS NOT NULL OR candidate_id IS NOT NULL
    )
);

CREATE INDEX IF NOT EXISTS idx_reliability_incident_audit_incident
    ON reliability_incident_audit_log (incident_id, created_at, id)
    WHERE incident_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS reliability_incident_settings_audit (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT,
    before_state JSONB NOT NULL,
    after_state JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO settings (key, value, updated_at) VALUES
    ('reliability_incidents_enabled', 'false', NOW()),
    ('reliability_incidents_public_enabled', 'false', NOW())
ON CONFLICT (key) DO NOTHING;
