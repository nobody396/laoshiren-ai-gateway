-- 119_seed_rbac_initial_data.sql
-- Seed final RBAC menus and preserve access for existing administrators.

INSERT INTO admin_roles (name, description, is_super_admin, status)
VALUES ('super_admin', '超级管理员, 拥有所有权限', true, 'active')
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    is_super_admin = true,
    status = 'active',
    updated_at = now();

INSERT INTO admin_menus (name, name_en, type, path, component, icon, permission_key, sort_order, status)
VALUES
    ('管理仪表盘', 'Dashboard', 'menu', '/admin/dashboard', '', 'dashboard', 'admin:dashboard', 10, 'active'),
    ('运维监控', 'Operations', 'menu', '/admin/ops', '', 'chart', 'admin:ops', 20, 'active'),
    ('用户管理', 'Users', 'menu', '/admin/users', '', 'users', 'admin:users', 30, 'active'),
    ('代理商管理', 'Agents', 'menu', '/admin/agents', '', 'agent', 'admin:agents', 40, 'active'),
    ('分组管理', 'Groups', 'menu', '/admin/groups', '', 'folder', 'admin:groups', 50, 'active'),
    ('渠道管理', 'Channels', 'menu', '/admin/channels', '', 'channel', 'admin:channels', 60, 'active'),
    ('订阅管理', 'Subscriptions', 'menu', '/admin/subscriptions', '', 'credit-card', 'admin:subscriptions', 70, 'active'),
    ('账号管理', 'Accounts', 'menu', '/admin/accounts', '', 'globe', 'admin:accounts', 80, 'active'),
    ('公告管理', 'Announcements', 'menu', '/admin/announcements', '', 'bell', 'admin:announcements', 90, 'active'),
    ('反馈管理', 'Feedback', 'menu', '/admin/feedbacks', '', 'feedback', 'admin:feedbacks', 100, 'active'),
    ('代理管理', 'Proxies', 'menu', '/admin/proxies', '', 'server', 'admin:proxies', 110, 'active'),
    ('卡密管理', 'Redeem Codes', 'menu', '/admin/redeem', '', 'ticket', 'admin:redeem', 120, 'active'),
    ('优惠码管理', 'Promo Codes', 'menu', '/admin/promo-codes', '', 'gift', 'admin:promo-codes', 130, 'active'),
    ('使用记录', 'Usage', 'menu', '/admin/usage', '', 'chart', 'admin:usage', 140, 'active'),
    ('充值订单', 'Top-up Orders', 'menu', '/admin/topup-orders', '', 'credit-card', 'admin:topup-orders', 150, 'active'),
    ('开票管理', 'Invoice Requests', 'menu', '/admin/invoice-requests', '', 'ticket', 'admin:invoice-requests', 160, 'active'),
    ('角色管理', 'Roles', 'menu', '/admin/roles', '', 'shield', 'admin:roles', 900, 'active'),
    ('菜单管理', 'Menus', 'menu', '/admin/menus', '', 'menu', 'admin:menus', 910, 'active'),
    ('API 管理', 'APIs', 'menu', '/admin/apis', '', 'code', 'admin:apis', 920, 'active'),
    ('系统设置', 'Settings', 'menu', '/admin/settings', '', 'cog', 'admin:settings', 1000, 'active')
ON CONFLICT (permission_key) DO UPDATE SET
    name = EXCLUDED.name,
    name_en = EXCLUDED.name_en,
    type = EXCLUDED.type,
    path = EXCLUDED.path,
    component = EXCLUDED.component,
    icon = EXCLUDED.icon,
    sort_order = EXCLUDED.sort_order,
    status = EXCLUDED.status,
    updated_at = now();

INSERT INTO admin_user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
CROSS JOIN admin_roles r
WHERE u.role = 'admin'
  AND u.deleted_at IS NULL
  AND r.name = 'super_admin'
ON CONFLICT (user_id, role_id) DO NOTHING;

SELECT setval('admin_roles_id_seq', COALESCE((SELECT MAX(id) FROM admin_roles), 1), true);
SELECT setval('admin_menus_id_seq', COALESCE((SELECT MAX(id) FROM admin_menus), 1), true);
SELECT setval('admin_user_roles_id_seq', COALESCE((SELECT MAX(id) FROM admin_user_roles), 1), true);
