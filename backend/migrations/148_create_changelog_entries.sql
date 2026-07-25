-- Curated Build in Public changelog. Git metadata is internal traceability
-- only and is intentionally excluded from public API responses.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

CREATE TABLE IF NOT EXISTS changelog_entries (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(180) NOT NULL UNIQUE,
    title VARCHAR(200) NOT NULL,
    summary VARCHAR(500) NOT NULL,
    rationale VARCHAR(500) NOT NULL,
    content TEXT NOT NULL,
    category VARCHAR(30) NOT NULL,
    related_products JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    published_at TIMESTAMPTZ,
    commit_sha VARCHAR(64),
    pull_request_url VARCHAR(500),
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_changelog_status
        CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT chk_changelog_category
        CHECK (category IN ('feature', 'model_config', 'improvement', 'fix')),
    CONSTRAINT chk_changelog_published_at
        CHECK (status <> 'published' OR published_at IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_changelog_entries_status_published_at
    ON changelog_entries (status, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_changelog_entries_category
    ON changelog_entries (category);
CREATE INDEX IF NOT EXISTS idx_changelog_entries_created_at
    ON changelog_entries (created_at DESC);

COMMENT ON TABLE changelog_entries IS 'Curated public Build in Public changelog';
COMMENT ON COLUMN changelog_entries.commit_sha IS 'Internal traceability only; never exposed publicly';
COMMENT ON COLUMN changelog_entries.pull_request_url IS 'Internal traceability only; never exposed publicly';

INSERT INTO admin_menus (
    name, name_en, type, path, component, icon, permission_key, sort_order, status
)
VALUES (
    '更新日志管理', 'Changelog', 'menu', '/admin/changelog', '', 'edit',
    'admin:changelog', 95, 'active'
)
ON CONFLICT (permission_key) DO UPDATE SET
    name = EXCLUDED.name,
    name_en = EXCLUDED.name_en,
    path = EXCLUDED.path,
    icon = EXCLUDED.icon,
    sort_order = EXCLUDED.sort_order,
    status = EXCLUDED.status,
    updated_at = NOW();

INSERT INTO admin_apis ("group", path, method, description, sort_order, status)
VALUES
    ('更新日志管理', '/admin/changelog', 'GET', '分页查询更新日志', 1, 'active'),
    ('更新日志管理', '/admin/changelog', 'POST', '创建更新日志', 2, 'active'),
    ('更新日志管理', '/admin/changelog/:id', 'GET', '获取更新日志详情', 3, 'active'),
    ('更新日志管理', '/admin/changelog/:id', 'PUT', '编辑更新日志', 4, 'active'),
    ('更新日志管理', '/admin/changelog/:id', 'DELETE', '删除未发布更新日志', 5, 'active')
ON CONFLICT (method, path) DO UPDATE SET
    "group" = EXCLUDED."group",
    description = EXCLUDED.description,
    sort_order = EXCLUDED.sort_order,
    status = EXCLUDED.status,
    updated_at = NOW();
