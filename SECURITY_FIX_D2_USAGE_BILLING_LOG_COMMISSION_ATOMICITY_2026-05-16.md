# SECURITY FIX D2: usage billing, usage_log, and commission atomicity

Date: 2026-05-16

## 修复了什么

- 标准计费模式下，`GatewayService.RecordUsage`、`GatewayService.RecordUsageWithLongContext`、`OpenAIGatewayService.RecordUsage` 现在先同步持久化 `usage_logs` 并取得 `usage_logs.id`，再执行 billing。
- 如果 `usage_logs.Create` 返回错误，`RecordUsage` 会返回错误，并且不会调用 `UsageBillingRepository.Apply` 或旧的扣费 fallback。
- 佣金仍只在 billing 成功且实际 applied 后触发，并继续使用已持久化的 `usage_logs.id` 作为 `source_id` 幂等键。
- Simple mode 仍保持非计费 best-effort usage log，不纳入本次 D2 标准计费 fail-closed 行为。

## Changed files

- `backend/internal/service/gateway_service.go`
- `backend/internal/service/openai_gateway_service.go`
- `backend/internal/service/gateway_record_usage_test.go`
- `backend/internal/service/openai_gateway_record_usage_test.go`
- `SECURITY_FIX_D2_USAGE_BILLING_LOG_COMMISSION_ATOMICITY_2026-05-16.md`

## 新增/更新了哪些测试

- 更新 `TestGatewayServiceRecordUsage_UsageLogCreateErrorSkipsBilling`：覆盖标准模式下 usage log `Create` 返回错误时，billing repo 不被调用，余额/API key 配额也不扣。
- 新增 `TestGatewayServiceRecordUsageWithLongContext_UsageLogCreateErrorSkipsBilling`：覆盖 long-context 标准计费路径下 usage log `Create` 返回错误时，billing repo 不被调用，余额/API key 配额也不扣。
- 更新 `TestOpenAIGatewayServiceRecordUsage_UsageLogCreateErrorSkipsBilling`：覆盖 OpenAI gateway 同样的 fail-closed 行为。
- 更新 `TestOpenAIGatewayServiceRecordUsage_UsageLogNotPersistedSkipsBilling`：覆盖 `MarkUsageLogCreateNotPersisted` 错误不会继续计费。
- 保留并通过 `TestGatewayServiceRecordUsage_TriggersCommissionWithPersistedUsageLogID` 和 `TestOpenAIGatewayServiceRecordUsage_TriggersCommissionWithPersistedUsageLogID`：证明佣金继续使用已持久化的 `usage_logs.id` 作为 `source_id`。
- 更新 billing error 相关旧期望：新安全顺序下 billing 失败前 usage log 已经持久化，不再断言 “billing error skips usage log write”。
- 更新 detached context 相关测试：验证同步持久化 usage log 与 billing 仍使用脱离请求取消的上下文。

## 已运行哪些测试及结果

- `go test -tags=unit ./internal/service -run 'Test(GatewayServiceRecordUsage|OpenAIGatewayServiceRecordUsage).*' -count=1`：通过。
- `go test -tags=unit ./internal/repository -run 'TestUsageBillingRepositoryApply' -count=1`：通过。
- `go build -o /tmp/sub2api-server-check ./cmd/server`：通过。

## 上线前还需要做什么

- 在 staging 环境用非生产凭据验证标准计费请求的 usage log、billing idempotency、commission `source_id` 行为。
- 准备运维核对：如发现 billing 失败但 usage log 已存在的记录，需要用既有 request_id / usage_logs.id 进行人工或脚本化 reconciliation。
- 确认 D1 的 final balance/quota enforcement 改动已随同构建进入同一发布批次。

## 是否需要数据库迁移

不需要。本次只调整服务层顺序和测试，复用已有 `usage_logs.id` 与 commission `source_id` 幂等机制。

## 是否有剩余风险

- 本次是小范围 fail-closed/可重试修复，不是真正的 usage log + billing + commission 单数据库事务。
- billing 成功后 commission 仍是异步处理，依赖 `usage_logs.id` 作为 `source_id` 的幂等能力以及后续 backfill/reconciliation；当前没有 durable outbox。
- 如果 usage log 已持久化但后续 billing 返回错误，会留下未扣费 usage log，需要上线前准备 reconciliation/重试策略。

## 下一个待处理问题

D3: usage record queue can drop chargeable usage.
