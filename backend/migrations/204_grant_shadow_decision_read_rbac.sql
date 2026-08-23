-- Channel Monitoring embeds recent Shadow decisions through the authenticated
-- API client. Grant that existing read-only route to roles already carrying
-- the admin:ops menu permission.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

INSERT INTO admin_apis ("group", path, method, description, sort_order, status, created_at, updated_at)
VALUES ('运维监控', '/admin/ops/openai-route-shadow/decisions', 'GET', 'OpenAI Shadow 路由决策列表', 57, 'active', NOW(), NOW())
ON CONFLICT (method, path) DO UPDATE SET description=EXCLUDED.description, "group"=EXCLUDED."group", sort_order=EXCLUDED.sort_order, status='active', updated_at=NOW();

INSERT INTO admin_role_apis (role_id, api_id, created_at)
SELECT rm.role_id, api.id, NOW()
FROM admin_role_menus rm
JOIN admin_menus menu ON menu.id=rm.menu_id AND menu.permission_key='admin:ops'
JOIN admin_apis api ON api.method='GET' AND api.path='/admin/ops/openai-route-shadow/decisions'
ON CONFLICT (role_id, api_id) DO NOTHING;
