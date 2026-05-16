# SECURITY FIX D6: Agent Settlement Concurrent Overpayment

Date: 2026-05-16

## 修复了什么

- 修复代理商结算中“先读取未结算佣金、再插入结算流水”的并发竞态。
- 新增原子 repository 边界 `CreateAgentSettlementIfAvailable`：
  - 在单个数据库事务内执行；
  - 使用 `pg_advisory_xact_lock(agent_id)` 获取每个代理商的事务级锁；
  - 锁内重新计算累计佣金与已完成结算总额；
  - 可结算余额不足时返回 `SETTLEMENT_EXCEEDS_UNSETTLED`；
  - 余额足够时插入 `completed` settlement 并返回 `id` / `created_at`。
- `CommissionService.CreateAgentSettlement` 保留代理身份与正数金额校验，但不再使用无锁 summary 读取作为最终授权。

## Changed Files

- `backend/internal/service/commission.go`
- `backend/internal/service/commission_admin_service.go`
- `backend/internal/repository/agent_commission_admin_repo.go`
- `backend/internal/service/commission_admin_service_test.go`
- `backend/internal/repository/agent_commission_admin_repo_integration_test.go`
- `SECURITY_FIX_D6_AGENT_SETTLEMENT_CONCURRENT_OVERPAYMENT_2026-05-16.md`

## 新增/更新测试

- Service unit tests:
  - 正常创建 settlement 时走原子 repository 方法；
  - `amount <= 0` 返回 `INVALID_SETTLEMENT_AMOUNT`；
  - repository 返回余额不足时透出 `SETTLEMENT_EXCEEDS_UNSETTLED`。
- Repository integration test:
  - 本地 Testcontainers Postgres 中创建 available commission = 100；
  - 并发两次 settlement，每次 amount = 75；
  - 验证最终只有一笔成功，`completed` settlements 总额不超过 100。

## 已运行测试及结果

- `go test -tags=unit ./internal/service -run 'Test.*AgentSettlement|Test.*Commission.*Settlement|Test.*D6' -count=1`
  - Result: passed
- `go test -tags=integration ./internal/repository -run 'Test.*AgentSettlement.*(Concurrent|Available|D6|Create)' -count=1`
  - Result: passed
- `go build -o /tmp/sub2api-server-check ./cmd/server`
  - Result: passed

## 上线前还需要做什么

- 主会话审查 D6 diff，确认与 A1/B1/B2/D1/A2/B3/B4/C1/C2/D2/D3/D4/D5 已有修复没有冲突。
- 按项目发布流程在非生产环境验证代理商结算 API。
- 获得明确上线许可后再部署；本修复未执行部署。

## 是否需要数据库迁移

- 不需要。
- 修复使用现有 `commission_records` 与 `agent_settlements` 表，并通过 PostgreSQL advisory transaction lock 实现并发保护。

## 剩余风险

- 本修复保护通过 `CreateAgentSettlementIfAvailable` 的结算创建路径。
- 若未来新增其他直接写入 `agent_settlements` 的路径，必须同样走原子余额检查或等价锁机制。
- 浮点金额比较仍沿用现有 `float64` 服务模型，并保留 `0.00000001` 容差；未在本任务内做金额类型重构。

## 下一个待处理问题

- P0/P1 队列已清空。
- P2/validation 暂停。
