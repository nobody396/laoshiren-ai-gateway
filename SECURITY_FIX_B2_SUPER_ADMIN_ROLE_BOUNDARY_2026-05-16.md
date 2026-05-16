# Security Fix Record: B2 Super Admin Role Boundary

Date: 2026-05-16
Repository: `/Users/fujunhao/laoshirenai/DragonCode-sub2api`
Status: Fixed locally, not deployed

## 修复了什么

修复 P0 B2: 非超级管理员不能再创建、更新、分配或撤销 `is_super_admin=true` 的 RBAC 角色。

- 管理员认证加载 RBAC 上下文时，同时把当前操作者是否为超级管理员写入 request context，保留原有 gin context 标记。
- `RBACService.CreateRole` 会拒绝非超级管理员创建超级管理员角色。
- `RBACService.UpdateRole` 会读取现有角色；只要现有角色是超级管理员，或本次更新会改变 `is_super_admin` 标记，非超级管理员都会收到 403。
- `RBACService.AssignUserRoles` 会检查目标用户当前角色和请求的新角色集合；只要涉及超级管理员角色的分配或撤销，非超级管理员都会收到 403。
- `RBACHandler.UpdateRole` 先读取现有角色再合并 PATCH 字段，避免请求未传 `is_super_admin` 时被零值覆盖。

## changed files

- `backend/internal/service/rbac_actor_context.go`
- `backend/internal/service/rbac_service.go`
- `backend/internal/server/middleware/admin_auth.go`
- `backend/internal/handler/admin/rbac_handler.go`
- `backend/internal/service/rbac_service_super_admin_boundary_test.go`
- `backend/internal/handler/admin/rbac_handler_super_admin_boundary_test.go`
- `SECURITY_FIX_B2_SUPER_ADMIN_ROLE_BOUNDARY_2026-05-16.md`

## 新增/更新了哪些测试

- 新增 RBAC service 单元测试:
  - 非超级管理员创建超级管理员角色返回 403。
  - 非超级管理员更新现有超级管理员角色返回 403。
  - 非超级管理员提升普通角色为超级管理员返回 403。
  - 非超级管理员给自己或另一名管理员分配超级管理员角色返回 403。
  - 非超级管理员撤销目标用户已有超级管理员角色返回 403。
  - 超级管理员仍可创建、更新、分配、撤销超级管理员角色。
  - 普通非超级角色分配仍可用。
- 新增 RBAC handler 单元测试:
  - `UpdateRole` 部分请求会保留现有角色字段。
  - 非超级管理员对现有超级管理员角色做部分更新会返回 403，而不是被零值误合并。

## 已运行哪些测试及结果

修复前，新增窄测试失败并复现问题:

```bash
go test -tags=unit ./internal/service ./internal/handler/admin -run 'TestRBAC(Service|Handler).*(SuperAdmin|Partial)' -count=1
```

失败点:

- service 允许非超级管理员创建、更新、分配、撤销超级管理员角色。
- handler 对部分更新构造零值角色，导致遗漏字段覆盖和错误响应。

修复后通过:

```bash
go test -tags=unit ./internal/service ./internal/handler/admin -run 'TestRBAC(Service|Handler).*(SuperAdmin|Partial)' -count=1
go test -tags=unit ./internal/service -run 'TestRBACService' -count=1
go test -tags=unit ./internal/handler/admin -run 'TestRBACHandler' -count=1
go test -tags=unit ./internal/server/middleware -run 'Test(AdminAuth|RequireAPI|RequirePermission)' -count=1
go build -o /tmp/sub2api-server-check ./cmd/server
```

Handler integration role tests were compile-checked but skipped because no local test database was configured:

```bash
go test -tags=integration -v ./internal/handler/admin -run 'TestRBACIntegration_(CreateRole|UpdateRole|SetUserRoles)' -count=1
```

Result: PASS, selected tests skipped due `TEST_DATABASE_URL` not being set.

## 上线前还需要做什么

1. Review B2 code and tests together with the existing A1/B1 local changes.
2. Deploy only through the normal controlled release process after explicit owner approval.
3. After deployment, verify that a delegated RBAC admin can still manage ordinary roles but receives 403 for super-admin role creation/update/assignment/revocation.

## 是否需要数据库迁移

No. This fix does not add or change database schema.

## 是否有剩余风险

- This fix does not address A2, B3, B4, C, or D findings.
- Existing users who already have super-admin roles are not modified by this local code change.
- Role cache invalidation behavior for role status or super-admin flag updates remains the separate B3 queue item.

## 下一个待处理问题是什么

Next queued issue: `D1. usage billing does not enforce final balance/quota limits`.

`B3. inactive roles still authorize; role update does not clear cache` remains open later in the queue.
