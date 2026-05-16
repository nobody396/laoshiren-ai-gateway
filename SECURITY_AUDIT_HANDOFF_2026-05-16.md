# DragonCode-sub2api Security Audit Handoff

Date: 2026-05-16
Repository: `/Users/fujunhao/laoshirenai/DragonCode-sub2api`

This document is a handoff for a new repair session. It is based on a read-only security audit using four subagent review lines:

- Auth/session state
- RBAC/admin/agent permissions
- API key/gateway/secrets/config
- Billing/payment/orders/commission integrity

No code, database schema, deployment configuration, or production service was modified during the audit.

## 1. Current Decision

Do not wait until every low-risk or speculative item is fully validated before fixing. That would keep confirmed severe issues open.

Use this rule:

- Fix confirmed P0/P1 issues first, but require a failing or targeted regression test before each fix where practical.
- Verify any "needs validation" item before fixing only if it can change the design of the same P0/P1 fix.
- Defer P2/P3 items unless they block a P0/P1 fix or are extremely cheap and directly adjacent.
- Do not bundle unrelated fixes. Use small slices with clear tests and rollback boundaries.

Recommended workflow:

1. Verification gate: for each P0/P1 item, prove the current behavior with a focused test, httptest, repository test, or static evidence.
2. Fix gate: implement only that issue's minimal fix.
3. Regression gate: run the targeted test plus affected package tests.
4. Integration gate: after a batch, run broader backend tests that do not require production services.
5. Stop before deployment. Produce a changed-files summary and residual-risk list.

This approach avoids two failure modes:

- Fixing unverified theories and breaking production behavior.
- Delaying confirmed high-impact fixes while investigating lower-priority hardening.

## 2. Audit Boundaries

Allowed in the repair session:

- Local code changes.
- Local unit tests, httests, mock tests, or local integration tests against a disposable test database.
- New tests that reproduce P0/P1 behavior.
- Local static checks and `go test`.

Not allowed unless the owner explicitly approves:

- Production deployment.
- Production database access, production migrations, real payment callbacks, real recharge/deduct/refund operations.
- Printing real API keys, JWTs, OAuth tokens, database passwords, upstream account credentials, or TOTP seeds.
- Destructive commands such as database reset, data deletion, or force checkout of user changes.
- Reverting unrelated dirty worktree changes.

Current worktree had unrelated existing changes before this handoff. Start by running `git status --short` and avoid touching unrelated frontend docs/router/store changes unless required by the fix.

## 3. P0/P1 Repair Queue

### Batch A: Auth Revocation

#### A1. P0: `TokenVersion` is not persisted

Evidence:

- `service.User` has `TokenVersion`: `backend/internal/service/user.go`
- JWT and refresh token compare claims against `user.TokenVersion`: `backend/internal/server/middleware/jwt_auth.go`, `backend/internal/service/auth_service.go`
- Ent `users` schema does not define `token_version`: `backend/ent/schema/user.go`
- `user_repo.Update` does not persist it: `backend/internal/repository/user_repo.go`
- Password change only increments an in-memory object: `backend/internal/service/user_service.go`

Impact:

- Password change or reset may not invalidate old access tokens or refresh tokens.
- The check appears present in middleware but cannot work reliably because the value reloads as default zero.

Required fix:

- Add a persisted `token_version` field to the user model/schema and migration.
- Ensure repository mapping loads and saves it.
- Ensure password change, password reset, and revoke-all-session flows increment it atomically.
- Ensure old access tokens and old refresh tokens are rejected after version change.

Acceptance criteria:

- A test logs in, changes password, reloads user from persistence, then old access token is rejected.
- A test logs in, changes password, then old refresh token is rejected.
- `POST /api/v1/auth/revoke-all-sessions` invalidates old access tokens as well as refresh tokens.

#### A2. P1: refresh token rotation is not atomic

Evidence:

- Current flow reads token, validates it, deletes it, then generates a new token: `backend/internal/service/auth_service.go`
- Redis operations are separate: `backend/internal/repository/refresh_token_cache.go`

Impact:

- Two concurrent refresh requests can both read the same refresh token before deletion and both issue new token pairs.
- Reuse detection does not revoke the token family.

Required fix:

- Make refresh token consume/rotate atomic.
- Reuse of an already-consumed refresh token should revoke the family or otherwise invalidate descendants.
- If token storage succeeds but set membership fails, do not leave a valid token that revoke-all cannot find.

Acceptance criteria:

- Concurrent refresh with the same token returns at most one success.
- Reusing an old refresh token invalidates the family according to policy.
- Partial Redis/set failure does not create a valid orphan refresh token.

### Batch B: RBAC/Admin Privilege Boundaries

#### B1. P0: menu `permission_key="*"` grants all API access

Evidence:

- `GetUserPermissionKeys` merges menu permission keys and API permission keys into one list: `backend/internal/service/rbac_service.go`
- `RequireAPIPermission` treats `*` as all API permission: `backend/internal/server/middleware/rbac_permission.go`
- Menu create request accepts arbitrary `permission_key`: `backend/internal/handler/admin/rbac_handler.go`

Impact:

- A non-super admin with menu/role binding permissions can create a menu with permission key `*`, bind it to a role, and get full backend API access.

Required fix:

- Do not allow menu permissions to grant API wildcard.
- Reject `permission_key="*"` for menu create/update unless it is never merged into API permission checks.
- Prefer separating menu permission keys from API permission keys in authorization context.

Acceptance criteria:

- Non-super admin cannot gain API access via menu permission `*`.
- Existing super admin behavior remains intact.
- A regression test proves menu `*` does not satisfy `RequireAPIPermission`.

#### B2. P0: non-super admin can create or assign super-admin roles

Evidence:

- `CreateRoleRequest` accepts `is_super_admin`: `backend/internal/handler/admin/rbac_handler.go`
- `CreateRole` and `UpdateRole` persist `IsSuperAdmin` without checking current actor: `backend/internal/service/rbac_service.go`, `backend/internal/repository/rbac_repository.go`
- `SetUserRoles` allows assigning any role to an admin user.

Impact:

- A delegated RBAC admin can create a super role and bind it to themselves or another admin.

Required fix:

- Only current super admin can create, update, assign, or revoke roles with `is_super_admin=true`.
- Non-super admin cannot assign a super role even if they have role-assignment API permission.
- Add audit/logging for super-role mutations if existing audit infrastructure supports it.

Acceptance criteria:

- Non-super admin with RBAC management API receives 403 for creating/updating/assigning super role.
- Super admin can still perform intended super-role operations.
- Regression tests cover self-assignment and assignment to another admin.

#### B3. P1: inactive roles still authorize; role update does not clear cache

Evidence:

- `GetRolesByUserID` does not filter `admin_roles.status='active'`: `backend/internal/repository/rbac_repository.go`
- Permission cache TTL is 30 minutes: `backend/internal/repository/rbac_cache.go`
- `UpdateRole` does not invalidate all permissions: `backend/internal/service/rbac_service.go`

Impact:

- Disabling or downgrading a role may not revoke access immediately.
- An inactive super role can still be treated as super admin.

Required fix:

- Filter active roles during permission and super-admin resolution.
- Invalidate relevant/all RBAC permission caches when role status or `is_super_admin` changes.

Acceptance criteria:

- Inactive role grants no API permission and no super-admin bypass.
- Role downgrade takes effect immediately after update.

#### B4. P1: Admin API Key inherits "first active admin"

Evidence:

- `validateAdminAPIKey` calls `GetFirstAdmin`: `backend/internal/server/middleware/admin_auth.go`
- `GetFirstAdmin` orders active admin users by ID: `backend/internal/repository/user_repo.go`

Impact:

- Global admin key's authority changes depending on which admin user is first.
- In most deployments this likely means super-admin-equivalent access.

Required fix:

- For immediate severe-risk mitigation, restrict Admin API Key to explicit super-admin-only operations or introduce a stable service principal/scope model.
- At minimum, make behavior explicit and test it; avoid silently inheriting arbitrary first admin.

Acceptance criteria:

- Admin API Key authorization is deterministic and scoped.
- Tests cover key success/failure and permission behavior.

### Batch C: Gateway/API Key Authorization

#### C1. P1: Google/Gemini/Antigravity API key auth lacks parity checks

Evidence:

- Standard middleware checks IP restrictions, expired status, runtime expiry, and quota exhaustion: `backend/internal/server/middleware/api_key_auth.go`
- Google middleware checks active/user/balance/subscription but does not apply the same IP/expiry/quota checks: `backend/internal/server/middleware/api_key_auth_google.go`
- `/v1beta` and `/antigravity/v1beta` use Google middleware: `backend/internal/server/routes/gateway.go`

Impact:

- Users who set API Key IP restrictions, expiry, or quota may still be exposed on Gemini/Antigravity endpoints.

Required fix:

- Reuse a shared API Key validation helper between standard and Google middleware.
- Ensure Google-style errors preserve response format while enforcing the same security checks.
- Reject query-based key usage unless a deliberate compatibility exception is explicitly accepted.

Acceptance criteria:

- Expired key is rejected on `/v1beta` and `/antigravity/v1beta`.
- Quota-exhausted key is rejected.
- IP whitelist/blacklist is enforced.
- Query `key=` is rejected or gated by an explicit, tested compatibility policy.

#### C2. P1/P2: API key and upstream credential response exposure

Evidence:

- User API key DTO returns full `key`: `backend/internal/handler/dto/types.go`, `backend/internal/handler/dto/mappers.go`
- Admin account DTO returns `Credentials`: `backend/internal/handler/dto/types.go`, `backend/internal/handler/dto/mappers.go`

Impact:

- Full user API keys and upstream OAuth/API credentials can be exposed through list/detail responses, frontend state, browser cache, screenshots, and logs.

Required fix:

- P1 immediate: admin account credentials should be redacted by default in list/detail responses.
- P2 follow-up: user API keys should be one-time reveal at creation; list/detail/update should return masked/prefix metadata.
- If export endpoints intentionally include credentials, require explicit permission and make the response behavior clear.

Acceptance criteria:

- Admin account list/detail responses do not include full upstream tokens by default.
- User API key list/detail does not return full key after creation.
- Tests assert sentinel secret substrings are absent from normal responses.

### Batch D: Billing, Payment, Subscription, Commission

#### D1. P0: usage billing does not enforce final balance/quota limits

Evidence:

- Preflight only checks balance `> 0`: `backend/internal/service/billing_cache_service.go`
- Subscription preflight checks existing usage against limit, not `usage + final_cost <= limit`.
- Final SQL subtracts balance without lower-bound condition: `backend/internal/repository/usage_billing_repo.go`

Impact:

- A user with tiny positive balance can make an expensive request and go negative.
- A subscription near limit can exceed daily/weekly/monthly quota via one high-cost request or concurrent requests.

Required fix:

- In the billing transaction, apply conditional updates using final cost:
  - balance mode: update only if `balance >= cost`, unless explicit overdraft policy exists.
  - subscription mode: update only if `current_usage + cost <= limit`.
- Return a clear billing error if final enforcement fails.

Acceptance criteria:

- Balance 0.01, cost 1.00 fails final billing and does not commit billing effects.
- Subscription remaining 0.01, cost 1.00 fails final billing.
- Concurrent requests cannot overspend the same remaining balance/quota.

#### D2. P1: usage billing, usage_log, and commission are not atomic

Evidence:

- Billing applies before usage log insert: `backend/internal/service/gateway_service.go`
- Commission depends on `usageLog.ID` and is asynchronous: `backend/internal/service/gateway_service.go`

Impact:

- User can be charged without a usage log.
- Commission can be skipped because no usage log ID exists.

Required fix:

- Prefer putting usage_log creation, billing dedupe/effects, and a commission outbox marker in the same transaction.
- If full refactor is too large, at minimum fail the request or retry safely when usage log persistence fails after billing.

Acceptance criteria:

- Inject usage log insert failure: no committed charge, or a durable recovery/outbox record exists.
- Commission processing has a stable idempotency source.

#### D3. P1: usage record queue can drop chargeable usage

Evidence:

- Default overflow policy is `sample`: `backend/internal/config/config.go`
- Queue-full path drops most tasks under sample mode: `backend/internal/service/usage_record_worker_pool.go`

Impact:

- Under load, successful requests may have no usage log, no billing, and no commission.

Required fix:

- For chargeable standard mode, use sync/fail-closed or durable outbox.
- Sampling can remain only for non-chargeable telemetry, not billing.

Acceptance criteria:

- Queue full does not silently drop chargeable usage.
- Test with queue size 1 confirms chargeable task is synchronously processed or rejected.

#### D4. P1: payment order completion is not DB-level idempotent

Evidence:

- `payment_service.completeOrder` reads order status then updates unconditionally.
- `payment_order_repo.UpdateStatus` has no `WHERE status='pending'`.
- Topup already has the correct pattern via `CompleteIfPending`.

Impact:

- Concurrent payment notify/query can both complete the same order and grant/extend subscription more than once.

Required fix:

- Add `CompleteIfPending` for payment orders.
- Only the single successful state transition performs subscription assignment.

Acceptance criteria:

- Concurrent completion of one payment order produces exactly one subscription side effect.

#### D5. P1: subscription extension has lost update and nested transaction risk

Evidence:

- `AssignOrExtendSubscription` reads existing expiry, computes new expiry, then writes absolute value.
- It starts its own transaction even when called inside payment/redeem transaction contexts.

Impact:

- Concurrent extensions can overwrite each other.
- Inner transaction can commit subscription changes even if outer payment/redeem transaction fails.

Required fix:

- Make extension atomic using row lock or SQL expression.
- Do not open an inner transaction when the context already contains a transaction.

Acceptance criteria:

- Two concurrent +30 day extensions result in +60 days.
- Inject outer failure after subscription assignment: no committed subscription change remains.

#### D6. P1: agent settlement concurrent overpayment

Evidence:

- Settlement checks current unsettled amount, then inserts settlement without lock.
- Insert path has no conditional balance decrement or advisory lock.

Impact:

- Two admins can settle the same available commission amount concurrently.

Required fix:

- Use transaction plus row/advisory lock per agent.
- Or maintain a settlement balance and conditionally decrement it.

Acceptance criteria:

- Concurrent settlement requests cannot create completed settlements totaling more than available unsettled commission.

## 4. Further Validation Queue

Validate these after or alongside the related P0/P1 batches. Do not let them block unrelated confirmed fixes.

### Auth/session validation

- OAuth pending session callback should consume state atomically.
- Redis partial failures should not create refresh tokens that revoke-all cannot find.
- TOTP verification attempts should fail closed or degrade safely if cache errors occur.
- Frontend multi-tab refresh should not race refresh token rotation.

### RBAC/admin validation

- RBAC nil service should not fail open in any production path.
- Admin agent-management APIs are global; confirm whether delegated operator roles should be scoped by agent ownership.
- Existing RBAC integration tests bypass real middleware; add true route-level tests.

### Gateway/config validation

- Confirm production config does not use placeholder JWT secret.
- Confirm URL allowlist, private hosts, insecure HTTP, and trusted proxies are production-safe.
- Confirm API key cache invalidation works across multiple backend instances.

### Billing/business validation

- Confirm `usageBillingRepo` cannot be nil in production standard mode.
- Confirm first-topup reward and commission record consistency.
- Confirm admin manual balance/concurrency adjustments either write account_change_records transactionally or fail visibly.

## 5. Suggested New Session Prompt

Copy the following prompt into the new repair session:

```text
你是 Codex，工作目录是 /Users/fujunhao/laoshirenai/DragonCode-sub2api。

请先阅读本地文件 SECURITY_AUDIT_HANDOFF_2026-05-16.md，然后只修复其中确认的 P0/P1 严重问题。项目是正式商业中转站，目标是保证用户登录、权限、API 调用、余额/订阅/支付/分润不出严重安全和资金错误。

严格要求：
- 开始前运行 git status --short，识别已有用户改动；不要回滚或覆盖无关改动。
- 不部署、不连接生产数据库、不跑生产迁移、不触发真实支付/充值/扣费/回调、不打印真实 secret。
- 每个修复先加或更新能证明问题的本地测试；无法加测试时必须说明原因和替代验证。
- 只修 P0/P1，P2/P3 先不要顺手改，除非它们直接阻塞 P0/P1。
- 不做大重构；按小批次提交代码层面的修改。
- 每个批次后运行受影响的 go test；最后给出 changed files、测试结果、剩余风险。

优先修复顺序：
1. Auth revocation:
   - 持久化 users.token_version。
   - 改密、重置密码、revoke-all-sessions 后旧 access token 和 refresh token 都失效。
   - 修复 refresh token 并发轮转和复用撤销。

2. RBAC:
   - 禁止非 super admin 创建/修改/分配 is_super_admin 角色。
   - 禁止菜单 permission_key=\"*\" 赋予 API 通配权限。
   - inactive role 不参与授权。
   - role 更新后立即清理 RBAC 权限缓存。

3. Gateway API key:
   - /v1beta 与 /antigravity/v1beta 执行和标准 /v1 一致的 IP、过期、额度、用户状态、余额/订阅检查。
   - 禁止或显式受控 query key=，默认按安全策略拒绝。

4. Billing/payment/subscription:
   - usage billing 事务内按最终成本校验余额和订阅额度，不能负余额或超套餐。
   - chargeable usage 不能因 worker queue full/sample 默认策略被静默丢弃。
   - payment order 用 pending -> completed 条件更新，只允许一个完成者发放权益。
   - subscription extension 使用行锁或原子 SQL，避免并发 lost update；不要在已有事务 ctx 内开内层事务。
   - agent settlement 防并发超结算。

5. Secrets exposure:
   - 管理端 account credentials 默认脱敏。
   - 用户 API key 后续 list/detail/update 默认脱敏；如保留创建时一次性展示，测试要覆盖。

验收标准：
- 所有新增/修改测试通过。
- 旧 token 失效、非超管不能自提权、Gemini API key 限制一致、余额/订阅不能超扣、payment 幂等、subscription 并发延期、agent settlement 并发结算都有回归测试。
- 最终不要部署，只报告如何验证和剩余需要人工确认的配置项。
```

## 6. Recommended Repair Batches

Do not repair all categories in one change. Suggested batch order:

1. Auth P0/P1.
2. RBAC P0/P1.
3. Gateway API Key P1.
4. Billing/payment/subscription P0/P1.
5. Credential response redaction P1/P2.

After each batch, stop and review tests before proceeding to the next batch.

