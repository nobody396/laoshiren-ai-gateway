# SECURITY FIX D3: Usage Record Queue Fail-Closed

日期：2026-05-16

## 修复了什么

- 修复 usage record worker pool 队列满、sample 策略丢弃或 pool stopped 时，Gateway/OpenAI Gateway 的计费用量任务被静默丢弃的问题。
- `UsageRecordWorkerPool.Submit` 的 sample/drop/sync 语义保持不变，仍可供非关键任务使用。
- handler 提交计费用量任务时现在会检查 `Submit` 返回值：
  - `enqueued`：保持异步执行。
  - `sync_fallback`：保持 pool 内同步 fallback 结果。
  - `dropped`：handler 用带 10 秒 timeout 的 context 同步执行 task，并 recover panic。
- 保留了原先无 worker pool 注入时的同步 fallback 和 panic recover 行为。

## Changed Files

- `backend/internal/handler/usage_record_submit_task.go`
- `backend/internal/handler/gateway_handler.go`
- `backend/internal/handler/openai_gateway_handler.go`
- `backend/internal/handler/usage_record_submit_task_test.go`
- `SECURITY_FIX_D3_USAGE_RECORD_QUEUE_FAIL_CLOSED_2026-05-16.md`

## 新增/更新了哪些测试

- 更新 `backend/internal/handler/usage_record_submit_task_test.go`
- 新增饱和 worker pool 场景：
  - `TestGatewayHandlerSubmitUsageRecordTask_PoolDropSyncFallback/drop`
  - `TestGatewayHandlerSubmitUsageRecordTask_PoolDropSyncFallback/sample_drop`
  - `TestOpenAIGatewayHandlerSubmitUsageRecordTask_PoolDropSyncFallback/drop`
  - `TestOpenAIGatewayHandlerSubmitUsageRecordTask_PoolDropSyncFallback/sample_drop`
- 测试构造 `WorkerCount=1`、`QueueSize=1`、worker 与 queue 已满，并验证 handler 在 `Submit` 返回 dropped 时会同步执行 task。
- 这些新增测试在旧代码下失败：handler 忽略 dropped 返回值，task 不会执行。

## 已运行哪些测试及结果

- `go test -tags=unit ./internal/handler -run 'Test.*UsageRecord.*PoolDropSyncFallback' -count=1`
  - 旧代码：失败，复现 dropped task 未同步执行。
  - 修复后：通过。
- `go test -tags=unit ./internal/handler -run 'Test.*UsageRecord.*' -count=1`
  - 通过。
- `go test -tags=unit ./internal/service -run 'TestUsageRecordWorkerPool' -count=1`
  - 通过。
- `go build -o /tmp/sub2api-server-check ./cmd/server`
  - 通过。

## 上线前还需要做什么

- 在 staging 或等价非生产环境压测 usage record worker pool 队列满场景，确认请求尾部同步 fallback 的延迟影响可接受。
- 观察 `usage_record.submit_dropped_sync_fallback` 日志频率；如果频繁出现，应调大 worker/queue 或排查 usage 记录链路耗时。
- 确认 D2 的同步持久化 usage log 再 billing 路径已随本次版本一起上线。

## 是否需要数据库迁移

不需要。本修复只改变 handler 提交 usage record task 的运行时行为，不新增或修改数据库 schema。

## 是否有剩余风险

- 当 worker pool 长时间饱和时，handler 会同步执行计费用量记录，成功请求的尾部延迟会增加。
- 同步 fallback 使用 10 秒 timeout；如果下游持久化或 billing 长时间阻塞，task 仍可能因超时失败，但不会再因为 queue overflow 被静默丢弃。
- panic 会被 recover 并记录错误日志，避免影响 handler 进程稳定性；业务错误仍依赖 task 内部日志和 D2 计费路径处理。

## 下一个待处理问题

D4: payment order completion is not DB-level idempotent
