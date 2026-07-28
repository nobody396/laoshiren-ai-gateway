-- Rename upstream proxy menu to avoid confusion with affiliate partners.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '1min';

UPDATE admin_menus
SET name = '代理节点',
    updated_at = NOW()
WHERE path = '/admin/proxies'
  AND name IN ('代理管理', 'IP管理');

UPDATE admin_apis
SET "group" = '代理节点',
    updated_at = NOW()
WHERE "group" = '代理管理'
  AND path LIKE '/admin/proxies%';
