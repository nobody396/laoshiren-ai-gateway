# Security Fix Record: D7 Pay-as-you-go Dust Balance Free Usage

Date: 2026-08-04
Branch: `fix/payg-balance-gate-20260804`
Status: Fixed locally, not deployed

## 修复了什么

修复按量（standard 分组）计费的一个资金漏洞：余额低于单次请求成本的用户可以无限免费使用。

根因是 D1（2026-05-16）引入的最终扣费防线与前置余额闸门之间存在缝隙：

- 前置闸门（`api_key_auth.go`、`BillingCacheService.checkBalanceEligibility`）
  只在 `balance <= 0` 时拦截；
- 最终扣费（`deductUsageBillingBalance`）要求 `balance >= 本次成本`，
  否则整体回滚、余额分文不动；
- 计费发生在上游请求完成之后，回滚并不挽回已产生的上游成本。

于是余额落在 `(0, 单次成本)` 区间的用户（如 $0.004）：闸门永远放行、
扣费永远失败、余额永远不变——请求无限免费。

生产实证（2026-07-28 balance_lots 上线后）：3 个从未充值的用户在余额
$5.20 耗尽后继续产生 913 个请求、账面 $46.97、真实上游成本约 $18.81，
且当时仍在继续。

## 修复方式

`deductUsageBillingBalance` 改为余额不足全额时扣到 0（钳制，不透支）：

```sql
UPDATE users
SET balance = GREATEST(balance - $1, 0), updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL
RETURNING balance
```

- 单条 UPDATE + 行锁串行化，余额不会变负（D1 的防透支目标仍然成立）；
- 余额收敛到 0 后，下一个请求即被现有 `balance <= 0` 闸门拦截，
  用户至多再多跑一个无法全额扣费的请求；
- 扣费成功提交后走既有缓存链路（QueueDeductBalance / 缓存余额变负），
  缓存闸门同步生效；
- FIFO 批次归因仍按全额 BalanceCost 执行，缺口沿用既有
  `legacy_reconcile` 批次兜底，账务归因完整；
- 订阅（月卡）模式的分日/周/月限额防线不受影响，保持不变。

## changed files

- `backend/internal/repository/usage_billing_repo.go`
- `backend/internal/repository/usage_billing_repo_test.go`
- `backend/internal/repository/usage_billing_repo_integration_test.go`
- `SECURITY_FIX_D7_PAYG_DUST_BALANCE_FREE_USAGE_2026-08-04.md`

## 新增/更新了哪些测试

- 单元测试（sqlmock）：
  `TestUsageBillingRepositoryApply_BalanceFinalLimitClampsToZeroOnInsufficientFunds`
  ——余额不足时扣费提交、返回余额 0，替代原回滚断言。
- 集成测试：
  `TestUsageBillingRepositoryApply_BalanceFinalLimitClampsInsufficientFundsToZero`
  ——余额 $0.01、成本 $1.00，扣费成功、余额归零、dedup 落库。
  `TestUsageBillingRepositoryApply_ConcurrentBalanceFinalLimitNeverOverdrafts`
  ——并发两笔 $0.75（余额 $1.00）都成功、余额钳制到 0 且不为负，
  替代原"一笔成功一笔 ErrInsufficientBalance"断言。

## 已运行哪些测试及结果

```bash
go build ./...
go vet ./internal/repository/
go test ./internal/repository -run 'TestUsageBillingRepositoryApply' -count=1
go test ./internal/repository -count=1
go test ./internal/service -run 'Test(OpenAIGatewayServiceRecordUsage|GatewayServiceRecordUsage|BillingCache)' -count=1
```

结果：全部 PASS。

集成测试（Testcontainers）：

```bash
go test -tags=integration ./internal/repository -run 'TestUsageBillingRepositoryApply_(BalanceFinalLimitClampsInsufficientFundsToZero|ConcurrentBalanceFinalLimitNeverOverdrafts)$' -count=1 -v
```

结果：PASS（Testcontainers postgres:18.1 + redis:8.4，两个用例均通过）。

## 上线前还需要做什么

1. 集成测试跑绿后，通过正常受控发布流程部署（需 owner 明确批准）。
2. 部署后观察：当前处于零头余额状态的用户（如 user 13/14/86）
   再发一个请求即被扣到 0 并拦截；也可选择在部署前人工把这批
   零头余额清零以立即止血。
3. 低余额用户会触发既有"余额不足预警"邮件，属预期行为。

## 是否需要数据库迁移

No. 只修改条件 SQL 与测试，无 schema 变更。

## 是否有剩余风险

- 余额为正的零头用户在修复后仍能免费跑"最后一个请求"（成本上限为
  单次请求成本），这是请求后计费系统的固有边界，已由 budget guard
  的低余额并发限制进一步压缩风险窗口。
- 本修复不改变预检缓存检查口径；D1 的订阅限额逻辑、API Key 配额、
  支付幂等等均不在本次范围内。
