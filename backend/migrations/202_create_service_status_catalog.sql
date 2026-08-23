-- Explicit Service Status catalog and current-state storage. All public and
-- evaluation switches remain disabled until operator verification.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE INDEX IF NOT EXISTS idx_reliability_observations_platform_model_observed
    ON reliability_observations (platform, model, observed_at DESC);

CREATE TABLE IF NOT EXISTS service_status_families (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(64) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS service_status_products (
    id BIGSERIAL PRIMARY KEY,
    family_id BIGINT NOT NULL REFERENCES service_status_families(id) ON DELETE RESTRICT,
    code VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(120) NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    public BOOLEAN NOT NULL DEFAULT TRUE,
    critical BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_service_status_products_family_sort
    ON service_status_products (family_id, sort_order, id);

CREATE TABLE IF NOT EXISTS service_status_components (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES service_status_products(id) ON DELETE CASCADE,
    code VARCHAR(140) NOT NULL UNIQUE,
    display_name VARCHAR(140) NOT NULL,
    model_pattern VARCHAR(160) NOT NULL DEFAULT '',
    access_mode VARCHAR(16) NOT NULL DEFAULT 'http',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT service_status_component_access_mode_check CHECK (access_mode = 'http')
);

CREATE INDEX IF NOT EXISTS idx_service_status_components_product
    ON service_status_components (product_id, id);

CREATE TABLE IF NOT EXISTS service_status_bindings (
    id BIGSERIAL PRIMARY KEY,
    binding_key VARCHAR(180) NOT NULL UNIQUE,
    component_id BIGINT NOT NULL REFERENCES service_status_components(id) ON DELETE CASCADE,
    group_id BIGINT,
    group_name VARCHAR(100) NOT NULL DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT '',
    model_pattern VARCHAR(160) NOT NULL DEFAULT '',
    route_fingerprint VARCHAR(32) NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT service_status_binding_selector_check CHECK (
        group_id IS NOT NULL OR group_name <> '' OR platform <> '' OR
        model_pattern <> '' OR route_fingerprint <> ''
    ),
    CONSTRAINT service_status_binding_route_fingerprint_check CHECK (
        route_fingerprint = '' OR route_fingerprint ~ '^[0-9a-f]{32}$'
    )
);

CREATE INDEX IF NOT EXISTS idx_service_status_bindings_component
    ON service_status_bindings (component_id, id);
CREATE INDEX IF NOT EXISTS idx_service_status_bindings_group
    ON service_status_bindings (group_id) WHERE group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_service_status_bindings_route
    ON service_status_bindings (route_fingerprint) WHERE route_fingerprint <> '';

CREATE TABLE IF NOT EXISTS service_status_component_current (
    component_id BIGINT PRIMARY KEY REFERENCES service_status_components(id) ON DELETE CASCADE,
    computed_status VARCHAR(32) NOT NULL DEFAULT 'monitoring',
    reason VARCHAR(64) NOT NULL DEFAULT 'no_evidence',
    evidence_at TIMESTAMPTZ,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    monitoring_since TIMESTAMPTZ,
    recovery_confirmed_at TIMESTAMPTZ,
    customer_request_count INTEGER NOT NULL DEFAULT 0,
    customer_success_count INTEGER NOT NULL DEFAULT 0,
    customer_failure_count INTEGER NOT NULL DEFAULT 0,
    probe_count INTEGER NOT NULL DEFAULT 0,
    probe_success_count INTEGER NOT NULL DEFAULT 0,
    probe_failure_count INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT service_status_component_current_status_check CHECK (
        computed_status IN ('operational','degraded_performance','partial_outage','major_outage','maintenance','monitoring')
    )
);

CREATE TABLE IF NOT EXISTS service_status_current (
    product_id BIGINT PRIMARY KEY REFERENCES service_status_products(id) ON DELETE CASCADE,
    computed_status VARCHAR(32) NOT NULL DEFAULT 'monitoring',
    effective_status VARCHAR(32) NOT NULL DEFAULT 'monitoring',
    computed_reason VARCHAR(64) NOT NULL DEFAULT 'no_evidence',
    effective_reason VARCHAR(64) NOT NULL DEFAULT 'no_evidence',
    evidence_at TIMESTAMPTZ,
    computed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    monitoring_since TIMESTAMPTZ,
    recovery_confirmed_at TIMESTAMPTZ,
    customer_request_count INTEGER NOT NULL DEFAULT 0,
    customer_success_count INTEGER NOT NULL DEFAULT 0,
    customer_failure_count INTEGER NOT NULL DEFAULT 0,
    probe_count INTEGER NOT NULL DEFAULT 0,
    probe_success_count INTEGER NOT NULL DEFAULT 0,
    probe_failure_count INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT service_status_current_computed_check CHECK (
        computed_status IN ('operational','degraded_performance','partial_outage','major_outage','maintenance','monitoring')
    ),
    CONSTRAINT service_status_current_effective_check CHECK (
        effective_status IN ('operational','degraded_performance','partial_outage','major_outage','maintenance','monitoring')
    )
);

CREATE TABLE IF NOT EXISTS service_status_overrides (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES service_status_products(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL,
    reason VARCHAR(500) NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_by_user_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT service_status_override_status_check CHECK (
        status IN ('operational','degraded_performance','partial_outage','major_outage','maintenance','monitoring')
    ),
    CONSTRAINT service_status_override_window_check CHECK (expires_at > starts_at)
);

CREATE INDEX IF NOT EXISTS idx_service_status_overrides_active
    ON service_status_overrides (product_id, starts_at, expires_at DESC);

INSERT INTO settings (key, value, updated_at) VALUES
    ('service_status_enabled', 'false', NOW()),
    ('service_status_public_enabled', 'false', NOW())
ON CONFLICT (key) DO NOTHING;

INSERT INTO service_status_families (code, display_name, sort_order) VALUES
    ('openai-codex', 'OpenAI / Codex', 10),
    ('claude', 'Claude', 20),
    ('grok', 'Grok', 30),
    ('gemini', 'Gemini', 40),
    ('builder-pass', 'Builder Pass', 50),
    ('other', '其他服务', 100)
ON CONFLICT (code) DO NOTHING;

WITH products(family_code, code, display_name, sort_order, critical) AS (VALUES
    ('openai-codex', 'openai-codex-api', 'OpenAI / Codex API', 10, TRUE),
    ('claude', 'claude-api', 'Claude API', 10, TRUE),
    ('grok', 'grok-api', 'Grok API', 10, FALSE),
    ('gemini', 'gemini-api', 'Gemini API', 10, FALSE),
    ('builder-pass', 'builder-pass-gpt', 'Builder Pass GPT', 10, TRUE),
    ('builder-pass', 'builder-pass-claude', 'Builder Pass Claude', 20, TRUE),
    ('builder-pass', 'builder-pass-grok', 'Builder Pass Grok', 30, FALSE)
)
INSERT INTO service_status_products (family_id, code, display_name, sort_order, critical)
SELECT family.id, products.code, products.display_name, products.sort_order, products.critical
FROM products JOIN service_status_families family ON family.code = products.family_code
ON CONFLICT (code) DO NOTHING;

INSERT INTO service_status_components (product_id, code, display_name, access_mode)
SELECT id, code || '-http', display_name || ' · HTTP', 'http'
FROM service_status_products
ON CONFLICT (code) DO NOTHING;

WITH platform_bindings(product_code, platform) AS (VALUES
    ('openai-codex-api', 'openai'),
    ('claude-api', 'anthropic'),
    ('grok-api', 'grok'),
    ('gemini-api', 'gemini')
)
INSERT INTO service_status_bindings (binding_key, component_id, platform)
SELECT component.code || ':platform:' || binding.platform, component.id, binding.platform
FROM platform_bindings binding
JOIN service_status_products product ON product.code = binding.product_code
JOIN service_status_components component ON component.product_id = product.id
ON CONFLICT (binding_key) DO NOTHING;

WITH monthly_bindings(product_code, group_name) AS (VALUES
    ('builder-pass-gpt', 'GPT Plus 月卡组'),
    ('builder-pass-gpt', 'GPT Pro V3 月卡组'),
    ('builder-pass-gpt', 'GPT Max V3 月卡组'),
    ('builder-pass-claude', 'Claude Plus 月卡组'),
    ('builder-pass-claude', 'Claude Pro V3 月卡组'),
    ('builder-pass-claude', 'Claude Max V3 月卡组'),
    ('builder-pass-grok', 'Grok Plus 月卡组'),
    ('builder-pass-grok', 'Grok Pro V3 月卡组'),
    ('builder-pass-grok', 'Grok Max V3 月卡组')
)
INSERT INTO service_status_bindings (binding_key, component_id, group_name)
SELECT component.code || ':group-name:' || binding.group_name, component.id, binding.group_name
FROM monthly_bindings binding
JOIN service_status_products product ON product.code = binding.product_code
JOIN service_status_components component ON component.product_id = product.id
ON CONFLICT (binding_key) DO NOTHING;

INSERT INTO service_status_component_current (component_id)
SELECT id FROM service_status_components
ON CONFLICT (component_id) DO NOTHING;

INSERT INTO service_status_current (product_id)
SELECT id FROM service_status_products
ON CONFLICT (product_id) DO NOTHING;
