# Security Fix Record: A2 Refresh Token Rotation Atomic

Date: 2026-05-16
Repository: `/Users/fujunhao/laoshirenai/DragonCode-sub2api`
Status: Fixed locally, not deployed

## 修复了什么

修复 A2: Refresh Token 轮转现在以缓存层原子消费为边界，同一个 refresh token 最多只能成功刷新一次。

- `RefreshTokenCache` 新增 `ConsumeRefreshToken`，Redis 实现使用 Lua 脚本在单次 Redis 执行中读取、删除 active token，并写入有界 TTL 的 consumed marker。
- 已消费 token 再次出现时返回 `service.ErrRefreshTokenReused`，服务会撤销对应 token family。
- token family 撤销使用 Redis Lua 脚本设置有界 TTL 的 `token_family_revoked:{family_id}` marker，并删除 family 内 active refresh token。
- 新 refresh token 加入 family set 时会检查 family revoked marker；如果 replay 与新 token 创建并发发生，新 token 会失败关闭并删除已存储 token，避免 replay 漏掉 descendant。
- `StoreRefreshToken` 成功但加入 user/family tracking set 失败时，服务会删除刚存储的 refresh token 并返回错误，不再留下 revoke-all 或 family revoke 找不到的有效 orphan token。
- A1 的 expired token、inactive/deleted user、TokenVersion mismatch 行为保持原错误路径；TokenVersion mismatch 仍返回 `ErrTokenRevoked`。

## changed files

- `backend/internal/service/refresh_token_cache.go`
- `backend/internal/repository/refresh_token_cache.go`
- `backend/internal/service/auth_service.go`
- `backend/internal/service/auth_refresh_rotation_test.go`
- `backend/internal/service/auth_service_sso_test.go`
- `backend/internal/repository/user_repo_integration_test.go`
- `SECURITY_FIX_A2_REFRESH_TOKEN_ROTATION_ATOMIC_2026-05-16.md`

## 新增/更新了哪些测试

- 新增 service unit tests:
  - 并发两个 `RefreshTokenPair` 使用同一个 refresh token 时，确定性验证只有一个成功，另一个返回 reuse。
  - replay 已消费的旧 refresh token 后，family 被撤销，第一次刷新产生的 descendant refresh token 失效。
  - `GenerateTokenPair` 中 `AddToUserTokenSet` 或 `AddToFamilyTokenSet` 失败时失败关闭，缓存中不保留 active refresh token。
  - `RefreshTokenPair` 生成 descendant 时 family set 添加失败会失败关闭，并且旧 token replay 仍返回 reuse。
- 更新测试用 in-memory refresh token cache/fake，实现新的 `ConsumeRefreshToken` 接口。

## 已运行哪些测试及结果

修复前，新增窄测试失败并复现问题:

```bash
go test -tags=unit ./internal/service -run 'TestAuthServiceRefreshTokenPair_ConcurrentRefreshAllowsExactlyOneSuccess$' -count=1
```

结果: FAIL。两个并发 refresh 都成功，`successes` 为 `2`。

修复后通过:

```bash
go test -tags=unit ./internal/service -run 'TestAuthServiceRefreshTokenPair_|TestAuthServiceGenerateTokenPair_SetMembershipFailureFailsClosed' -count=1
```

结果: PASS。

```bash
go test -tags=unit ./internal/service -run 'Test(AuthServiceRefreshTokenPair_|AuthServiceGenerateTokenPair_SetMembershipFailureFailsClosed|AuthRevocation)' -count=1
```

结果: PASS。

```bash
go test -tags=unit ./internal/service -run 'TestAuthRevocation' -count=1
```

结果: PASS。

```bash
go test -tags=unit ./internal/service -run 'TestAuthService_SSO' -count=1
```

结果: PASS。

```bash
go test -tags=unit ./internal/repository -count=1
```

结果: PASS。

```bash
go test ./internal/repository -run 'TestNonExistentRefreshTokenCacheCompileCheck' -count=1
```

结果: PASS, no tests to run. 用于编译 repository package 中的 Redis refresh token cache。

```bash
go build -o /tmp/sub2api-server-check ./cmd/server
```

结果: PASS。

尝试运行更宽的 service unit package:

```bash
go test -tags=unit ./internal/service -count=1
```

结果: interrupted after about 2 minutes with no output. Targeted A2/A1 service checks had already passed.

## 上线前还需要做什么

1. Review A2 changes together with the existing local A1/B1/B2/D1 changes.
2. Run Redis-backed refresh token cache integration coverage in an environment with an approved local Redis if desired; this session used deterministic unit fakes and repository package compile coverage, not production Redis.
3. Deploy only through the normal controlled release process after explicit owner approval.
4. After deployment, verify login, refresh, refresh replay, and logout/revoke-all behavior with non-production test accounts first.

## 是否需要数据库迁移

No. This fix only changes Redis cache keys/operations, service behavior, tests, and documentation. No database migration is required.

## 是否有剩余风险

- Redis integration behavior was not exercised against a live Redis in this session. The Go code containing the Lua scripts compiled through repository package tests, and A2 behavior is covered deterministically with unit fakes.
- Existing consumed/revoked marker TTLs are bounded by the original refresh-token/family TTL where Redis has TTL metadata.
- This fix intentionally does not address B3/B4/C/D2-D6 findings.

## 下一个待处理问题是什么

Next queued issue: `B3. inactive roles still authorize; role update does not clear cache`.
