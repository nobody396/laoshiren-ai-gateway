-- Keep the new read-only Channel Monitoring API aligned with the existing
-- admin:ops menu role. Route discovery alone creates admin_apis rows but does
-- not grant them to non-super roles, so preserve that relation explicitly.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

INSERT INTO admin_apis ("group", path, method, description, sort_order, status, created_at, updated_at)
VALUES
 ('运维监控', '/admin/ops/channel-monitoring', 'GET', 'GET /admin/ops/channel-monitoring', 54, 'active', NOW(), NOW()),
 ('运维监控', '/admin/ops/openai-route-shadow/stats', 'GET', 'GET /admin/ops/openai-route-shadow/stats', 55, 'active', NOW(), NOW()),
 ('运维监控', '/admin/ops/openai-route-shadow/health', 'GET', 'GET /admin/ops/openai-route-shadow/health', 56, 'active', NOW(), NOW())
ON CONFLICT (method, path) DO UPDATE SET
  "group"=EXCLUDED."group", description=EXCLUDED.description,
  sort_order=EXCLUDED.sort_order, status='active', updated_at=NOW();

INSERT INTO admin_role_apis (role_id, api_id, created_at)
SELECT role_menu.role_id, api.id, NOW()
FROM admin_role_menus role_menu
JOIN admin_menus menu ON menu.id=role_menu.menu_id AND menu.permission_key='admin:ops'
JOIN admin_apis api ON api.method='GET' AND api.path IN (
 '/admin/ops/channel-monitoring',
 '/admin/ops/openai-route-shadow/stats',
 '/admin/ops/openai-route-shadow/health'
)
ON CONFLICT (role_id, api_id) DO NOTHING;
