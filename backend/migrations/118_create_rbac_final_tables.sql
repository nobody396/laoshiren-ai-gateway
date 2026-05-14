-- 118_create_rbac_final_tables.sql
-- RBAC final schema. This intentionally skips the historical intermediate
-- admin_permissions/admin_role_permissions tables from the RBAC feature branch.

CREATE TABLE IF NOT EXISTS admin_roles (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description VARCHAR(500) NOT NULL DEFAULT '',
    is_super_admin BOOLEAN NOT NULL DEFAULT false,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_menus (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    name_en VARCHAR(100) NOT NULL DEFAULT '',
    type VARCHAR(20) NOT NULL DEFAULT 'menu',
    path VARCHAR(255) NOT NULL DEFAULT '',
    component VARCHAR(255) NOT NULL DEFAULT '',
    icon VARCHAR(100) NOT NULL DEFAULT '',
    permission_key VARCHAR(100) NOT NULL UNIQUE,
    sort_order INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT admin_menus_type_check CHECK (type = 'menu')
);

CREATE INDEX IF NOT EXISTS idx_admin_menus_type ON admin_menus(type);
CREATE INDEX IF NOT EXISTS idx_admin_menus_permission_key ON admin_menus(permission_key);

CREATE TABLE IF NOT EXISTS admin_apis (
    id BIGSERIAL PRIMARY KEY,
    "group" VARCHAR(100) NOT NULL DEFAULT '',
    path VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT admin_apis_method_path_key UNIQUE (method, path)
);

CREATE INDEX IF NOT EXISTS idx_admin_apis_group ON admin_apis("group");

CREATE TABLE IF NOT EXISTS admin_role_menus (
    id BIGSERIAL PRIMARY KEY,
    role_id BIGINT NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    menu_id BIGINT NOT NULL REFERENCES admin_menus(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (role_id, menu_id)
);

CREATE INDEX IF NOT EXISTS idx_admin_role_menus_role_id ON admin_role_menus(role_id);
CREATE INDEX IF NOT EXISTS idx_admin_role_menus_menu_id ON admin_role_menus(menu_id);

CREATE TABLE IF NOT EXISTS admin_role_apis (
    id BIGSERIAL PRIMARY KEY,
    role_id BIGINT NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    api_id BIGINT NOT NULL REFERENCES admin_apis(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (role_id, api_id)
);

CREATE INDEX IF NOT EXISTS idx_admin_role_apis_role_id ON admin_role_apis(role_id);
CREATE INDEX IF NOT EXISTS idx_admin_role_apis_api_id ON admin_role_apis(api_id);

CREATE TABLE IF NOT EXISTS admin_user_roles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES admin_roles(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_admin_user_roles_user_id ON admin_user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_admin_user_roles_role_id ON admin_user_roles(role_id);
