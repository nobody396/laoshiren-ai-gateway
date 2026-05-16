# Security Fix Record: B4 Admin API Key Service Principal

Date: 2026-05-16
Repository: `/Users/fujunhao/laoshirenai/DragonCode-sub2api`
Status: Fixed locally, not deployed

## 修复了什么

修复 B4: Admin API Key 不再调用 `GetFirstAdmin`，也不再继承按 ID 排序第一个 active admin 用户的 RBAC 角色。

现在全局 Admin API Key 会认证为一个确定性的 service-principal 管理员上下文:

- `ContextKeyUser`: `AuthSubject{UserID: -1, Concurrency: 0}`
- `ContextKeyUserRole`: `service.RoleAdmin`
- `auth_method`: `admin_api_key`
- `ContextKeyUserPermissions`: `[]string{"*"}`
- `ContextKeyIsSuperAdmin`: `true`
- request context 的 RBAC actor super-admin 标记: `true`

这个选择把 Admin API Key 明确建模为全局超级管理员 service principal，避免权限随真实管理员用户排序变化。

## changed files

- `backend/internal/server/middleware/admin_auth.go`
- `backend/internal/server/middleware/admin_auth_test.go`
- `SECURITY_FIX_B4_ADMIN_API_KEY_SERVICE_PRINCIPAL_2026-05-16.md`

## 新增/更新了哪些测试

- 新增 Admin API Key 回归测试:
  - valid Admin API Key 不调用 `GetFirstAdmin`。
  - valid Admin API Key 设置固定 service-principal subject、`auth_method="admin_api_key"`、`RoleAdmin`、`[]string{"*"}`、`is_super_admin=true`。
  - Admin API Key 可通过 `RequireAPIPermission` 的 super-admin 上下文。
- 新增 invalid/missing Admin API Key 测试:
  - 缺失 header 返回 401。
  - 错误 header 返回 401。
  - 服务端未配置 Admin API Key 时返回 401。
  - 这些失败路径不调用 `GetFirstAdmin`。
- 新增 JWT admin auth 回归测试:
  - JWT 管理员认证仍使用真实用户 ID 加载 RBAC 权限。
  - JWT 管理员上下文保留真实 `AuthSubject.UserID`，且不会被 Admin API Key service-principal 行为影响。

## 已运行哪些测试及结果

修复前，新增窄测试失败并复现问题:

```bash
go test -tags=unit ./internal/server/middleware -run 'TestAdminAuth(APIKey|JWTUsesActualUserRBACContext)' -count=1
```

结果: FAIL。`TestAdminAuthAPIKeyUsesDeterministicServicePrincipal` 发现 valid Admin API Key 调用了 `GetFirstAdmin` 1 次。

修复后通过:

```bash
go test -tags=unit ./internal/server/middleware -run 'TestAdminAuth(APIKey|JWTUsesActualUserRBACContext)' -count=1
```

结果: PASS。

```bash
go test -tags=unit ./internal/server/middleware -run 'Test(AdminAuth|RequireAPI|RequirePermission)' -count=1
```

结果: PASS。

```bash
go test -tags=unit ./internal/server/middleware -count=1
```

结果: PASS。

```bash
go build -o /tmp/sub2api-server-check ./cmd/server
```

结果: PASS。

## 上线前还需要做什么

1. Review B4 with the existing local A1/A2/B1/B2/B3/D1 security changes.
2. Deploy only through the normal controlled release process after explicit owner approval.
3. Treat the Admin API Key as a global super-admin credential; confirm storage, access, rotation, and audit expectations before production rollout.
4. After deployment, verify a valid Admin API Key still reaches intended admin integration endpoints and an invalid key still returns 401.

## 是否需要数据库迁移

No. This fix only changes middleware context setup and tests. No database migration is required.

## 是否有剩余风险

- The global Admin API Key remains a super-admin-level credential. This fix makes that deterministic and explicit, but it does not add scoped Admin API Key permissions.
- Existing generated Admin API Keys are not rotated by this code change.
- This fix intentionally does not address C1/C2/D2-D6 findings.

## 下一个待处理问题是什么

Next queued issue per the user-specified queue: `C1. Google/Gemini/Antigravity API key auth lacks parity checks`.
