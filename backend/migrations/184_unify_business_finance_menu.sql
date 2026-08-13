-- Merge the separate finance-ledger navigation entry into one owner-facing
-- business finance center. Keep the existing permission key so current RBAC
-- role assignments continue to work without a migration of role grants.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

UPDATE admin_menus
SET name = '经营财务中心',
    name_en = 'Business Finance',
    path = '/admin/business-finance',
    icon = 'dollar',
    updated_at = now()
WHERE permission_key = 'admin:finance-transactions';
