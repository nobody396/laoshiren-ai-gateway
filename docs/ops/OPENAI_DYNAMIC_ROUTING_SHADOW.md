# OpenAI 动态成本路由 Shadow 观测与审计

本文定义动态成本路由从 `legacy` 进入 `shadow` 的生产门禁。`shadow`
只计算候选账号，不改变用户实际使用的 Legacy 账号；当前版本仍硬禁止
`enforce`。

新策略必须显式写 `request_class`。允许值为 `text`、`image` 或显式通配 `*`；
历史策略缺少该字段时只匹配 `text`，避免文本策略意外接管生图请求。不同请求类型
使用独立的路由健康与成本预算。

## 数据完整性边界

除决策审计外，V2 还会被动采集每一次真实上游尝试（包括被后续切换掩盖的失败）。
采集维度为 `group + account + model + request_class + endpoint_hash + transport +
failure_domain`，Redis 中只保存不可逆指纹和聚合计数：成功/分类失败、TTFT 直方图、
完成延迟直方图、半截流、样本数、最后观测时间和已结算成本。窗口包括最近一小时、
最近七个北京时间自然日，以及过去八周相同的北京时间“周内小时”。原始 URL、
请求/响应正文和凭证不会进入该存储。

文本成本来自与账号统计相同的已结算 usage 计算，并通过独立结算事件补入聚合，
不会把一次请求重复计为两次尝试。图片和视频在原始上游扣费尚未完成权威对账前不
参与成本学习，避免用少量 token 或用户售价反推供应商单图成本。

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

被动观测采集器另行暴露 `submitted/written/failed/dropped/rejected/in_flight`、
`completeness`、存储检查和最后成功/失败时间。队列满时丢弃的是学习证据而不是
阻塞用户请求；但完整率低于 99% 时健康状态必须为 `ready=false`，不得据此推进
灰度接管。

健康状态的写入与滚动证据写入分别计数，防止“统计已经落 Redis、健康转换失败”
被误报成整条统计丢失。供应商故障域只接受显式 `routing_failure_domain_id`，且必须
在故障窗口内由至少两个不同账号同时出现 capacity、5xx、畸形流或半截流证据才
升级；单账号重复失败，以及模型不支持、限流、鉴权、余额、用户请求或本地传输
错误均只影响最窄路由。

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
