# Security Fix Record: B1 RBAC Menu Wildcard

Date: 2026-05-16
Repository: `/Users/fujunhao/laoshirenai/DragonCode-sub2api`
Status: Fixed locally, not deployed

## Fixed Issue

P0 B1 from `SECURITY_AUDIT_HANDOFF_2026-05-16.md`:

RBAC menu `permission_key="*"` could be merged into the same permission list as API permissions. `RequireAPIPermission` then treated `"*"` as all API access. That meant a non-super admin who could create and assign a menu with `permission_key="*"` could gain backend API access beyond their intended role.

## What Changed

- `GetUserPermissionKeys` no longer adds menu `permission_key="*"` to non-super-admin permission keys.
- Cached permission lists containing `"*"` are rechecked against the user's current super-admin status. If the user is not super admin, `"*"` is removed and the cache is rewritten.
- `CheckPermission` no longer treats `"*"` as sufficient unless the user is currently super admin.
- `RequireAPIPermission` no longer treats `"*"` in `ContextKeyUserPermissions` as an API bypass. Super admin still bypasses through `ContextKeyIsSuperAdmin`.
- Generic `RequirePermission` was aligned with the same rule: wildcard bypass comes from the super-admin context, not from a permission-list string.
- Menu create/update now rejects `permission_key="*"` with a 400 application error from the RBAC service.

## Changed Files

- `backend/internal/service/rbac_service.go`
- `backend/internal/service/rbac_service_wildcard_test.go`
- `backend/internal/server/middleware/rbac_permission.go`
- `backend/internal/server/middleware/rbac_permission_test.go`
- `SECURITY_FIX_B1_RBAC_MENU_WILDCARD_2026-05-16.md`

## Tests Added Or Updated

- Added service-level regression tests:
  - non-super admin menu `permission_key="*"` is not returned as a permission key
  - cached non-super-admin `"*"` is removed
  - super admin still receives `"*"`
  - menu create/update rejects `permission_key="*"`
- Added middleware regression tests:
  - non-super admin permission list containing `"*"` does not satisfy API permission checks
  - super admin context still bypasses API permission checks
  - explicit API permission still allows non-super admin access

## Validation Run Locally

Before the fix, the new targeted test command failed and reproduced the issue:

```bash
go test -tags=unit ./internal/service ./internal/server/middleware -run 'Test(RBACService(GetUserPermissionKeys|CreateUpdateMenu)|RequireAPIPermission)' -count=1
```

Observed failures before the fix:

- non-super-admin permission keys contained `"*"` from a menu
- menu create/update accepted `permission_key="*"`
- `RequireAPIPermission` allowed a non-super admin whose permission list contained `"*"`

Passed after the fix:

```bash
go test -tags=unit ./internal/service ./internal/server/middleware -run 'Test(RBACService(GetUserPermissionKeys|CreateUpdateMenu)|RequireAPIPermission)' -count=1
go test -tags=unit ./internal/server/middleware -count=1
go test -tags=unit ./internal/service -run 'TestRBACService' -count=1
go build -o /tmp/sub2api-server-check ./cmd/server
```

Handler integration menu tests were compile-checked but skipped because no local test database was configured:

```bash
go test -tags=integration -v ./internal/handler/admin -run 'TestRBACIntegration_(CreateMenu|UpdateMenu)' -count=1
```

Result: PASS with each selected test skipped due `TEST_DATABASE_URL` not being set.

## Deployment And Migration Notes

No production deployment was performed.

No production database was accessed.

No production migration was run.

No database migration is required for this fix.

## Before Going Online

1. Review the code and tests.
2. Deploy through the normal controlled release process only after owner approval.
3. After deployment, verify a non-super admin with menu-only RBAC access cannot call unrelated admin APIs.
4. If any existing database row has `admin_menus.permission_key='*'`, clean it up through the normal admin/data maintenance process. The fixed code prevents API authorization from that row, but leaving it in the menu table may still confuse UI permissions.

## Remaining Risk

- This fix does not address other queued findings.
- Existing stale Redis permission caches containing `"*"` are sanitized on access, and the middleware no longer accepts `"*"` for non-super API access.
- Existing menu rows with `permission_key="*"` are not deleted by this code change because no production database work was allowed.

## Next Security Queue Item

Do not continue automatically. Wait for owner confirmation.

Next pending P0 from the handoff:

`B2. non-super admin can create or assign super-admin roles`

`A2. refresh token rotation is not atomic` remains pending, but it is P1 in the handoff and was intentionally not changed in this B1 repair.
