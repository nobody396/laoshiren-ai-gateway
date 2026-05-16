# Security Fix Record: D1 Usage Billing Final Limits

Date: 2026-05-16
Repository: `/Users/fujunhao/laoshirenai/DragonCode-sub2api`
Status: Fixed locally, not deployed

## 修复了什么

修复 D1: usage billing 的最终数据库事务现在会强制执行余额和订阅额度限制。

- 余额模式: `users` 扣费更新新增 `balance >= final BalanceCost` 条件。用户存在但余额不足时返回 `service.ErrInsufficientBalance`。
- 订阅模式: `user_subscriptions` 用量更新新增 daily/weekly/monthly 的 `current usage + final SubscriptionCost <= group limit` 条件。
- 订阅额度为 `NULL`、`0` 或负数时仍按现有 `Group.Has*Limit` 语义视为不限额。
- 条件更新失败时会区分 daily、weekly、monthly 超限并返回对应的 `service.ErrDailyLimitExceeded`、`service.ErrWeeklyLimitExceeded`、`service.ErrMonthlyLimitExceeded`。
- 因最终限制失败而返回错误时，事务回滚，之前插入的 `usage_billing_dedup` claim 不会提交，用户后续充值或额度重置后可以重试。
- 并发扣费依赖数据库条件更新和行锁重检，避免多个请求同时花掉同一份剩余额度。

## changed files

- `backend/internal/repository/usage_billing_repo.go`
- `backend/internal/repository/usage_billing_repo_test.go`
- `backend/internal/repository/usage_billing_repo_integration_test.go`
- `SECURITY_FIX_D1_USAGE_BILLING_FINAL_LIMITS_2026-05-16.md`

## 新增/更新了哪些测试

- 新增 SQL mock 单元测试:
  - 余额最终扣费发现用户存在但余额不足时返回 `ErrInsufficientBalance` 并 rollback。
  - 订阅最终扣费发现 daily 额度会超限时返回 `ErrDailyLimitExceeded` 并 rollback。
- 新增 repository integration 测试:
  - 余额 `0.01`、最终成本 `1.00` 失败，余额不变，`usage_billing_dedup` 不提交。
  - 订阅剩余 `0.01`、最终成本 `1.00` 失败，用量不变，`usage_billing_dedup` 不提交。
  - 并发两个余额扣费请求只能一个成功，另一个返回 `ErrInsufficientBalance`，最终余额不透支。
- 现有成功路径测试继续覆盖:
  - balance billing 成功扣费与重复请求 dedup。
  - subscription billing 成功增加用量与重复请求 dedup。

## 已运行哪些测试及结果

修复前，新增 SQL mock 窄测试失败并复现问题:

```bash
go test ./internal/repository -run 'TestUsageBillingRepositoryApply_BalanceFinalLimitRollbackOnInsufficientFunds$' -count=1
```

结果: FAIL。实际返回 `ErrUserNotFound`，没有把“用户存在但最终余额不足”映射为 `ErrInsufficientBalance`。

修复后通过:

```bash
go test ./internal/repository -run 'TestUsageBillingRepositoryApply_BalanceFinalLimitRollbackOnInsufficientFunds$' -count=1
go test ./internal/repository -run 'TestUsageBillingRepositoryApply_(BalanceFinalLimitRollbackOnInsufficientFunds|SubscriptionFinalLimitRollbackOnDailyOverage)$' -count=1
go test ./internal/repository -run 'TestUsageBillingRepositoryApply' -count=1
go test ./internal/service -run 'Test(OpenAIGatewayServiceRecordUsage|GatewayServiceRecordUsage)' -count=1
go test ./internal/repository -count=1
go build -o /tmp/sub2api-server-check ./cmd/server
```

结果: PASS。

尝试运行 D1 repository integration 选择集:

```bash
go test -tags=integration ./internal/repository -run 'TestUsageBillingRepositoryApply_(DeduplicatesBalanceBilling|DeduplicatesSubscriptionBilling|BalanceFinalLimitRejectsInsufficientFunds|SubscriptionFinalLimitRejectsOverage|ConcurrentBalanceFinalLimitPreventsOverspend)$' -count=1
go test -tags=integration -v ./internal/repository -run 'TestUsageBillingRepositoryApply_BalanceFinalLimitRejectsInsufficientFunds$' -count=1
```

结果: command exited PASS/ok, but `-v` confirmed repository integration tests were skipped by the harness because Docker is unavailable: `docker is not available; skipping integration tests (start Docker to enable)`.

Broader service package check:

```bash
go test ./internal/service -count=1
```

结果: FAIL due unrelated existing test `TestGatewayService_AnthropicAPIKeyPassthrough_StreamingTimeoutAfterClientDisconnect`; failure expected message mismatch: got `stream usage incomplete: missing terminal event`, expected text containing `stream usage incomplete after timeout`.

## 上线前还需要做什么

1. Review the D1 repository transaction changes together with the local A1/B1/B2 changes.
2. Run the D1 repository integration tests in an environment with Docker or an approved local test database.
3. Deploy only through the normal controlled release process after explicit owner approval.
4. After deployment, verify low-balance and subscription-near-limit requests fail cleanly and retries work after top-up or quota reset.

## 是否需要数据库迁移

No. This fix only changes conditional SQL updates and tests. No schema change or migration is required.

## 是否有剩余风险

- Local DB-backed integration assertions could not execute in this session because Docker is unavailable. SQL mock coverage proves the rollback/error path; DB concurrency proof remains pending until integration tests run with a local database.
- This fix does not change preflight cache checks; it enforces D1 at the final transaction boundary.
- This fix intentionally does not address account quota, API key quota, payment idempotency, usage log atomicity, queue behavior, subscription extension, agent settlement, or any A2/B3/B4/C/D2-D6 findings.

## 下一个待处理问题是什么

Next queued issue: `A2. refresh token rotation is not atomic`.
