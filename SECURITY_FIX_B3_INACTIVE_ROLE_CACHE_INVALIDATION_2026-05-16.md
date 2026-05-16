# Security Fix Record: B3 Inactive Role Cache Invalidation

Date: 2026-05-16
Repository: `/Users/fujunhao/laoshirenai/DragonCode-sub2api`
Status: Fixed locally, not deployed

## 修复了什么

修复 B3: inactive RBAC roles no longer authorize, and successful role updates now clear RBAC permission/menu caches.

- Authorization path now uses only `admin_roles.status='active'` roles from `GetRolesByUserID`.
- `GetUserMenuTree`, `GetUserPermissionKeys`, and `IsSuperAdmin` also defensively ignore non-active roles returned by fakes or alternate repositories.
- Users with no active roles now get no menu permissions, no API permissions, and no super-admin bypass.
- `RBACService.UpdateRole` invalidates all RBAC caches after a successful role update, so status / `is_super_admin` / permission-relevant changes take effect immediately.
- `GetUserRoles` remains unfiltered so admin user-role screens and B2 super-role boundary checks can still see inactive assigned roles.

## changed files

- `backend/internal/repository/rbac_repository.go`
- `backend/internal/service/rbac_service.go`
- `backend/internal/service/rbac_service_inactive_role_test.go`
- `backend/internal/service/rbac_service_super_admin_boundary_test.go`
- `backend/internal/handler/admin/rbac_handler_super_admin_boundary_test.go`
- `SECURITY_FIX_B3_INACTIVE_ROLE_CACHE_INVALIDATION_2026-05-16.md`

## 新增/更新了哪些测试

- 新增 B3 service unit tests:
  - inactive super role does not make `IsSuperAdmin` true
  - inactive super role does not return wildcard permissions
  - inactive normal role does not grant API/menu permissions
  - active normal role permissions still work
  - active super-admin behavior still works
  - successful `UpdateRole` invalidates all RBAC caches
- Updated existing B2 handler/service test fakes so the new `UpdateRole` invalidation call is implemented in those unit tests.

## 已运行哪些测试及结果

修复过程中，新增窄测试先失败并复现 inactive roles still granting permissions:

```bash
go test -tags=unit ./internal/service -run 'TestRBACService(Inactive|ActiveRolesStillAuthorize|UpdateRoleInvalidatesAllPermissionCaches)' -count=1
```

Result: FAIL before the final service short-circuit; inactive roles returned menu/API permissions.

修复后通过:

```bash
go test -tags=unit ./internal/service -run 'TestRBACService(Inactive|ActiveRolesStillAuthorize|UpdateRoleInvalidatesAllPermissionCaches)' -count=1
```

Result: PASS.

```bash
go test -tags=unit ./internal/service -run 'TestRBACService' -count=1
```

Result: PASS.

```bash
go test -tags=unit ./internal/handler/admin -run 'TestRBACHandler' -count=1
```

Result: PASS.

```bash
go test -tags=unit ./internal/server/middleware -run 'Test(AdminAuth|RequireAPI|RequirePermission)' -count=1
```

Result: PASS.

```bash
go test ./internal/repository -run 'TestNonExistentRBACCompileCheck' -count=1
```

Result: PASS, no tests to run. Used as repository package compile coverage for the RBAC repository change.

```bash
go test -tags=integration -v ./internal/handler/admin -run 'TestRBACIntegration_(UpdateRole|GetUserRoles|SetUserRoles)' -count=1
```

Result: PASS with selected DB-backed tests skipped because `TEST_DATABASE_URL` is not set.

```bash
go build -o /tmp/sub2api-server-check ./cmd/server
```

Result: PASS.

## 上线前还需要做什么

1. Review this B3 slice together with existing local A1/A2/B1/B2/D1 changes.
2. Run DB-backed RBAC integration tests in an approved local/test database environment if desired.
3. Deploy only through the normal controlled release process after explicit owner approval.
4. After deployment, verify disabling or downgrading an admin role immediately removes API access, menu access, and super-admin bypass for affected test accounts.

## 是否需要数据库迁移

No. This fix does not change database schema and does not require a migration.

## 是否有剩余风险

- Redis cache invalidation is called after successful role update and follows existing broad invalidation patterns; this session did not use a live Redis integration test.
- Existing direct database edits outside `RBACService.UpdateRole` would still need their own cache invalidation or TTL expiry.
- This fix intentionally does not address B4/C/D2-D6 findings.

## 下一个待处理问题是什么

Next queued issue per the user-specified queue: `B4. Admin API Key inherits "first active admin"`.
