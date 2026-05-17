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
- [x] 检查 diff，提交并推送 GitHub。
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
- 已推送 GitHub：`4ee2777 feat(agent): add automatic tier evaluation`。

# 代理收款与结算中心任务清单

## 目标
- 代理端填写支付宝收款资料并上传收款码。
- 后台代理商管理页显示收款资料状态、未结算金额、$50 起结门槛和结算差额。
- 管理员人工扫码付款后，点击确认结算生成结算记录。
- 结算记录保存收款资料快照，后续代理修改资料不影响历史记录。
- 不做汇率换算，不接支付宝 API，不自动转账。

## 执行状态
- [x] 新增代理收款资料表和 $50 起结门槛设置。
- [x] 新增代理端收款资料保存、二维码上传和查看接口。
- [x] 新增后台结算列表、收款资料查看和确认结算接口。
- [x] 后台代理商管理页增加结算筛选、起结门槛、结算状态和确认结算弹窗。
- [x] 代理端中心增加收款信息、起结进度、差额和二维码预览。
- [x] 结算记录保存支付宝资料和二维码路径快照。
- [x] 本地真实 E2E 验证通过。
- [x] 提交并推送 GitHub。
- [x] 生产部署上线。

## 本地 E2E 验证记录
- 使用本地 PostgreSQL、Redis、后端和前端 dev server 进行真实链路验证。
- 代理创建收款资料并上传支付宝二维码后，代理中心能展示资料、二维码、未结算金额、$50 起结门槛和差额。
- 未结算 $49 时，后台列表显示还差 $1，结算按钮禁用；直接调用结算接口返回 `SETTLEMENT_BELOW_MINIMUM`。
- 累计到 $55 后，后台列表显示 Ready，结算按钮可用，结算弹窗显示支付宝姓名、账号、电话和二维码。
- 点击确认结算后生成 1 条结算记录，未结算金额变为 $0，已结算金额变为 $55。
- 重复结算被拒绝，返回 `SETTLEMENT_BELOW_MINIMUM`。
- 结算后修改代理收款资料和二维码，历史结算记录仍保留结算时的旧资料快照。

## 回归验证记录
- `go test ./...` 通过。
- `go build -o /tmp/sub2api-server-check ./cmd/server` 通过。
- `pnpm --dir frontend run typecheck` 通过。
- `pnpm --dir frontend run test:run` 通过，56 个测试文件、349 个测试。
- `pnpm --dir frontend run build` 通过。

## 边界
- 已收到用户明确上线确认。
- 上线流程执行完成后，以 `/Users/fujunhao/laoshirenai/log.md` 和最终回复为准记录生产结果。
