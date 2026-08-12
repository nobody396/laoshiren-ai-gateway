# OpenAI 动态成本路由 Shadow 观测与审计

本文定义动态成本路由从 `legacy` 进入 `shadow` 的生产门禁。`shadow`
只计算候选账号，不改变用户实际使用的 Legacy 账号；当前版本仍硬禁止
`enforce`。

新策略必须显式写 `request_class`。允许值为 `text`、`image` 或显式通配 `*`；
历史策略缺少该字段时只匹配 `text`，避免文本策略意外接管生图请求。不同请求类型
使用独立的路由健康与成本预算。

## 数据完整性边界

每次**命中已启用 Shadow 策略**的负载均衡评估，都同步写入
`openai_route_shadow_decisions`。记录包含：

- 服务端 `request_id`、客户端 `client_request_id`、失败切换 `attempt`
- Shadow 随机种子、请求传输协议、Compact 要求和本次已排除账号 ID
- 分组、模型、请求类型（`text`/`image`）、策略模式和策略版本
- Legacy 实际选择、Shadow 建议选择、倍率、是否分流差异、是否使用应急预算
- 完整归一化策略和估算基础成本
- 5 分钟、1 小时、24 小时预算窗口的评估前账本与预计评估后账本
- 每个候选账号的健康状态、可靠性下界、TTFT 输入、负载、排队、优先级、倍率
- 每个候选账号的健康/延迟/余量/价格/优先级因子、最终权重、排名和排除原因
- Legacy 候选分数、Top-K、负载偏斜和实际抽样顺序，保证两套算法都可解释
- 评估耗时和评估失败原因

记录明确不包含 Prompt、响应正文、API Key、OAuth Token、账号名称或原始上游
URL。上游端点只保存不可逆短哈希；故障域只保存显式内部路由标识。

该表目前不自动清理，防止观测窗口内证据丢失。后续如需保留策略，必须先建立
归档与按策略版本汇总，不能直接复用通用日志清理任务。

## 不允许静默丢记录

Shadow 的可审计性是运行前置条件：

1. 首次评估前，通过回滚事务验证审计表真实可写，而不产生探针脏数据。
2. 每次评估完成后同步落库；写入超时为 200ms。
3. 写入失败时，当前样本立即标记为 `legacy/audit_persist_failed`，不把它计为有效
   Shadow 样本；用户仍走原 Legacy 账号。
4. 失败后审计健康变为 `ready=false`；后续请求先重新验证存储，验证失败则完全
   跳过 Shadow 评估，避免继续修改未审计的 Shadow 预算账本。
5. `attempted = written + failed + in_flight` 是决策写入完整性的基本不变量；
   请求排空后 `in_flight` 必须回到 0。存储探针次数和失败次数单独暴露，不能用
   普通 Debug 日志代替。

## 管理查询

仅管理员 Ops 路由可访问：

- `GET /api/v1/admin/ops/openai-route-shadow/health`
- `GET /api/v1/admin/ops/openai-route-shadow/decisions`
- `GET /api/v1/admin/ops/openai-route-shadow/stats`

列表和统计支持 `time_range`、`group_id`、`model`、`request_class`、`policy_version`、`reason`、
`request_id`、`client_request_id`、`evaluated`、`diverged`、`emergency` 过滤。
统计同时提供建议账号占比、评估 P50/P95、可关联的真实 Legacy 成功用量、Legacy
失败和 TTFT；客户端请求 ID 缺失时使用服务端请求 ID 的 `local:` 记账键回退
关联。无法关联成功或错误日志的样本单列为 `unlinked_outcome`，不能当作成功。

## 生产开启门禁

必须按以下顺序执行，严禁同一次发布直接进入 `enforce`：

1. 代码和迁移上线时，`openai_route_policies` 保持缺失或全部 `legacy`。
2. 验证应用 `/health`、审计健康 `ready=true`，并确认存储探针没有残留行。
3. 再单独写入版本化 Shadow 策略；策略必须显式包含分组、模型、成本目标、成本
   硬上限、估算基础成本和 `policy_version`。
4. 使用自有测试身份对每个目标分组发起真实请求，确认：
   - 用户实际账号仍等于 Legacy 选择；
   - 每次命中评估均有决策行；
   - `attempted = written + failed + in_flight`，排空后 `in_flight=0`；
   - 决策可通过两种 request id 回查；
   - 统计中的成功、失败、未关联结果能解释总样本数。
5. 观察真实流量的错误率、P95/P99 TTFT、切换耗时、建议账号占比、实际倍率、
   应急预算和账本一致性。样本不足或审计不完整时不得进入灰度接管。

## 回滚

回滚不依赖重新部署：把对应策略 `enabled=false` 或 `mode=legacy`，并使设置缓存
失效即可。历史决策表只追加保留，不能因回滚删除；它是判断策略为什么回滚的
证据。
