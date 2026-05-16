# 代理自动升降级任务清单

## 目标
- 实现代理自动升降级，快速口径：只按直属客户 `usage_logs.actual_cost` 统计消耗。
- 每月 1 号按上月自然月消耗和累计消耗评估代理等级。
- 本地完整测试和回归测试通过后提交并推送 GitHub。
- 不部署、不重启生产、不上线；生产上线必须等待用户明天明确确认。

## 业务规则
- 轻代理：5%，默认基准档。
- 标准代理：10%，累计直属客户消耗达到 1000 美元后永久生效。
- 核心代理：15%，上月直属客户消耗达到 1500 美元则当月临时生效；累计达到 10000 美元后永久生效。
- 超级代理：20%，上月直属客户消耗达到 3000 美元则当月临时生效；累计达到 50000 美元后永久生效。
- 最终生效等级取“手动基准等级、永久等级、月度临时等级”中返佣比例最高者。
- 已生成的佣金流水不重算；新佣金流水使用评估后的当前比例快照。

## 执行状态
- [x] 创建 10 分钟线程心跳。
- [x] 确认初始工作区无未提交改动。
- [x] 新增数据库迁移。
- [x] 新增后端等级模型、仓储和评估服务。
- [x] 接入代理返佣比例解析。
- [x] 增加后台接口。
- [x] 扩展后台代理管理页面。
- [x] 增加单元测试和仓储测试。
- [x] 执行本地回归测试。
- [ ] 检查 diff，提交并推送 GitHub。
- [x] 明确未上线。

## 验收命令
- `go test ./internal/service ./internal/handler/... ./internal/repository/...`
- `go build -o /tmp/sub2api-server-check ./cmd/server`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run build`

## 备注
- 本阶段接受快速口径的限制：赠送余额、优惠码、人工补偿余额被消费后也会进入升级和返佣统计。
- 所有生产操作禁止执行，直到用户明确说可以上线。

## 本地验证记录
- `go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/server/routes ./cmd/server` 通过。
- `go test ./internal/service ./internal/handler/... ./internal/repository/...` 通过。
- `go build -o /tmp/sub2api-server-check ./cmd/server` 通过。
- `pnpm --dir frontend run typecheck` 通过。
- `pnpm --dir frontend run build` 通过。
- `git diff --check` 通过。
