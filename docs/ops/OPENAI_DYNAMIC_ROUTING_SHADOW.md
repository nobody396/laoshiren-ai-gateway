# OpenAI 动态成本路由 Shadow 观测与审计

本文定义动态成本路由从 `legacy` 进入 `shadow` 的生产门禁。`shadow`
只计算候选账号，不改变用户实际使用的 Legacy 账号；当前版本仍硬禁止
`enforce`。

新策略必须显式写 `request_class`。允许值为 `text`、`image` 或显式通配 `*`；
历史策略缺少该字段时只匹配 `text`，避免文本策略意外接管生图请求。不同请求类型
使用独立的路由健康与成本预算。

每一次单独获授权的 Shadow 启用周期还必须生成一个**不可复用**、仅含 ASCII
字母/数字/`-._:` 的 `activation_id`，并写入真实 UTC RFC3339
`shadow_started_at`。缺失、非 UTC 或未来 T0 的已启用 Shadow 策略会 fail closed，
继续使用 Legacy。停用再开启时即使策略正文和 `policy_version` 没变，也必须使用新的
activation，防止两轮证据被混为一个连续观察窗。

## 数据完整性边界

除决策审计外，V2 还会被动采集每一次真实上游尝试（包括被后续切换掩盖的失败）。
采集维度为 `group + account + model + request_class + endpoint_hash + transport +
failure_domain`，Redis 与 PostgreSQL 中只保存不可逆指纹和聚合计数：成功/分类失败、TTFT 直方图、
完成延迟直方图、半截流、样本数、最后观测时间和已结算成本。窗口包括最近一小时、
最近七个北京时间自然日，以及过去八周相同的北京时间“周内小时”。原始 URL、
请求/响应正文和凭证不会进入该存储。

文本成本来自与账号统计相同的已结算 usage 计算，并通过独立结算事件补入聚合，
不会把一次请求重复计为两次尝试。图片和视频在原始上游扣费尚未完成权威对账前不
参与成本学习，避免用少量 token 或用户售价反推供应商单图成本。

Shadow 的候选级成本预测以策略 `estimated_base_cost_usd` 为先验。一个完整路由身份
累计至少 20 条最近七个北京时间自然日内的权威文本结算后，才使用 20 条等效先验
样本与真实均值做收缩，并把结果限制在配置值的 `0.25x--4x`；零值、非有限值、稀疏
样本和所有图片请求继续使用配置值。候选审计同时记录配置值、预测基础成本、预测
账号成本、真实均值、样本量和来源。评分、预算预览、Redis 原子预留和 Shadow 结算
使用同一个候选预测，不能出现“按低成本加权、按另一成本记账”。

每次**命中已启用 Shadow 策略**的负载均衡评估，都提交到有界异步队列写入
`openai_route_shadow_decisions`。记录包含：

- 服务端 `request_id`、客户端 `client_request_id`、失败切换 `attempt`
- Shadow 随机种子、请求传输协议、Compact 要求和本次已排除账号 ID
- 分组、模型、请求类型（`text`/`image`）、策略模式和策略版本
- 本次授权周期的 `activation_id` 和精确 `shadow_started_at`（T0）
- Legacy 实际选择、Shadow 建议选择、倍率、是否分流差异、是否使用应急预算
- 完整归一化策略和估算基础成本
- 5 分钟、1 小时、24 小时预算窗口的评估前账本与预计评估后账本
- 每个候选账号的健康状态、可靠性下界、TTFT 输入、负载、排队、优先级、倍率
- 每个候选账号的健康/延迟/余量/价格/优先级因子、最终权重、排名和排除原因
- 每个候选账号的成本先验、学习后估算、结算样本数和估算来源
- Legacy 候选分数、Top-K、负载偏斜和实际抽样顺序，保证两套算法都可解释
- 评估耗时和评估失败原因

记录明确不包含 Prompt、响应正文、API Key、OAuth Token、账号名称或原始上游
URL。上游端点只保存不可逆短哈希；故障域只保存显式内部路由标识。

该表目前不自动清理，防止观测窗口内证据丢失。后续如需保留策略，必须先建立
归档与按策略版本汇总，不能直接复用通用日志清理任务。

## 不允许静默丢记录

Shadow 的可审计性是运行前置条件：

1. 首次评估前，通过回滚事务验证审计表真实可写，而不产生探针脏数据。
2. 每次评估完成后只把不可变快照提交给 `8 workers / 4096 queue` 的有界队列；
   账号选择不等待数据库，单次后台写入超时为 200ms。
3. 队列无法接受时，当前请求立即标记为 `legacy/audit_persist_failed`；队列已接受但
   后台写入失败时，数据库中不会出现该样本，失败计数和完整率会阻止晋级。两种
   情况都不改变用户实际使用的 Legacy 账号。
4. 存储失败后 `storage_ready=false`；后续请求先重新验证存储，验证失败则完全
   跳过 Shadow 评估。即使存储后来恢复，早先缺失的决策仍保留在完整率分母，不能
   被一次成功探针清零。
5. `attempted = written + failed + in_flight` 是决策写入完整性的基本不变量；
   请求排空后 `in_flight` 必须回到 0。存储探针次数和失败次数单独暴露，不能用
   普通 Debug 日志代替。

被动观测采集器另行暴露 `submitted/written/failed/dropped/rejected/in_flight`、
`completeness`、存储检查和最后成功/失败时间。完整率以
`submitted + rejected` 为分母，因此仍在排队的证据也不会被提前当成已完成。健康
状态应用另有 `outcome_applied/failed/in_flight/completeness`；观测已写入但熔断状态
未更新不能被视为完整。队列满时丢弃的是学习证据而不是阻塞用户请求，但任一完整率
低于 99% 时健康状态必须为 `ready=false`，不得据此推进灰度接管。

Shadow 读取这些共享聚合时使用 5 秒、最多 512 个候选集合的进程内只读 L1，并用
singleflight 合并相同集合的并发共享存储回源；回源独立限时 20ms，错误不缓存，调用方
超时也不会取消正在给其他请求预热的有界回源。缓存深拷贝可变直方图/错误分类，候选
顺序和重复项不会制造不同缓存键。Redis 是 1 小时近期信号的权威源；PostgreSQL 小时
聚合是 7 日和 8 周同时段窗口的权威源，二者不相加以免双计。路由/供应商熔断状态仍在
每次评估时单独读取，不会被这层缓存延迟。命中、未命中、真实回源、合并返回、失败、
淘汰和当前条目数通过 Shadow 健康响应的 `observation_profile_cache` 暴露。
Redis 读失败时允许使用 PostgreSQL 长窗并把近期窗口置空；Redis 的采集检查/双写失败
仍会使完整率和 readiness 失败关闭，不能据此晋级。PostgreSQL 长窗失败则本次 Shadow
直接失败并继续走 Legacy。

路由和供应商熔断状态不做 L1 缓存，但也不再按候选顺序产生 `2N` 次 Redis 往返：
控制器把全部 `route + provider` 键交给一次 Redis pipeline，供应商共享键自动去重，
缺失键显式视为 `warmup`。任何无效键、Redis 错误、损坏状态或返回集合缺项都会使
本次 Shadow 评估失败并继续使用 Legacy，不会猜测为健康。
健康 pipeline 与共享观测读取彼此独立，会在同一次评估内并发执行并共同受调用方
截止时间约束；任一失败都会取消本次评估，不能用另一份成功数据掩盖依赖故障。

## 多 Base URL 主动证据桥（默认关闭）

`benchmark_prior_enabled` 和 `route_variants` 都是策略级、仅限 `text + Shadow` 的显式
开关；默认缺失时不会查询 `base_url_benchmarks`，不会增加候选，也不会改变现有策略的
评分或审计策略快照。`enforce` 仍由编译期硬关闭。

```json
{
  "benchmark_prior_enabled": true,
  "route_variants": [
    {"account_id": 23, "base_url": "https://hk.pomoai.xyz"},
    {"account_id": 23, "base_url": "https://hk2.pomoai.xyz"},
    {"account_id": 23, "base_url": "https://jp.pomoai.xyz"},
    {"account_id": 23, "base_url": "https://us.pomoai.xyz"}
  ]
}
```

安全不变量：

- Base URL 只能是无凭证、无 query/fragment 的公网 HTTPS 地址，单策略最多 8 条；原始
  URL 只用于进程内构造候选，持久化策略审计仅记录 `account_id + endpoint_hash`。
- 变体只复用已经经过 Legacy 硬过滤的同一个 `Account`，因此不能绕过分组、模型、
  schedulable、传输或账号状态；它不克隆密钥、不修改账号 Base URL，也不会执行建议线路。
- 同一账号的所有 URL 共享账号份额和 `failure_domain`，防止把同一密钥/供应商伪装成
  独立冗余；端点哈希保持独立，以便分别学习延迟和线路故障。
- 只有 Responses HTTP/SSE 文本路由会展开变体。OAuth、图片、Chat Completions 和其他
  传输保持原候选，不会被隐式改写。
- 主动探测是弱先验：读取最近 72 小时全局窗、最近 6 小时窗和过去 4 周同一北京时间
  周内小时，按 24 小时半衰期衰减；少于 12 条样本严格为零影响，最大置信度 25%。
  生成的变体在主动先验没有产生有效伪样本且被动真实结果不足 12 条时，以
  `insufficient_benchmark_evidence` 排除，不能因一条测速结果成为“时段赢家”。
- 置信度缩放先计算一个精确、受限的有效伪样本预算，再用中点分位数对有序延迟
  直方图下采样；不能逐桶独立四舍五入后把稀疏线路的 TTFT/完成延迟全部归零，造成
  不同 Base URL 看起来同速。该伪样本仍不进入真实成本、份额或供应商独立账号计数。
- 主动样本不进入真实流量份额、成本结算或供应商相关故障的独立账号计数。真实用户
  被动结果保持完整权重；主动证据只添加有界伪样本。
- `base_url_benchmarks` 是可选运维表；默认策略不依赖它。启用先验后缺表、查询超时或
  返回过多数据均 fail closed 为本次 Shadow 失败，用户仍由 Legacy 服务。读取使用
  1 分钟有界 L1、singleflight 和 20ms 回源超时，不把数据库查询放到每请求热路径。

候选审计新增 `route_fingerprint`、`route_variant`、主动证据来源、原始/有效/近期/
同周内小时样本、置信度、衰减权重和最近观测时间；选择与排除都按完整 route
fingerprint 关联，不能因同一账号有多个端点而同时标记多个候选。统计同时按账号、
故障域和端点汇总建议分布；晋级门要求三类选择计数都完整覆盖有效决策。

这些数据仍然只是“如果采用该线路，算法会建议什么”的 Shadow 证据。未实际执行的
URL 没有真实用户反事实成功率；任何 Canary 动态切换必须在独立审批包中明确比例、
确定性分桶、熔断、1--5 分钟 watchdog 和原子回滚，不能由该先验自动开启。

健康状态的写入与滚动证据写入分别计数，防止“统计已经落 Redis、健康转换失败”
被误报成整条统计丢失。供应商故障域只接受显式 `routing_failure_domain_id`，且必须
在故障窗口内由至少两个不同账号同时出现 capacity、5xx、畸形流或半截流证据才
升级；单账号重复失败，以及模型不支持、限流、鉴权、余额、用户请求或本地传输
错误均只影响最窄路由。客户端主动取消单列为 `client_cancelled`，只保留聚合计数，
不进入可靠性分母、不触发熔断，也不扩散到供应商故障域。

## 管理查询

仅管理员 Ops 路由可访问：

- `GET /api/v1/admin/ops/openai-route-shadow/health`
- `GET /api/v1/admin/ops/openai-route-shadow/decisions`
- `GET /api/v1/admin/ops/openai-route-shadow/stats`
- `GET /api/v1/admin/ops/openai-route-shadow/assessment`

列表和统计支持 `time_range`、`group_id`、`model`、`request_class`、`policy_version`、
`activation_id`、`reason`、
`request_id`、`client_request_id`、`evaluated`、`diverged`、`emergency` 过滤。
统计同时提供建议账号占比、评估 P50/P95、可关联的真实 Legacy 成功用量、Legacy
失败和 TTFT；客户端请求 ID 缺失时使用服务端请求 ID 的 `local:` 记账键回退
关联。无法关联成功或错误日志的样本单列为 `unlinked_outcome`，不能当作成功。

`assessment` 是纯只读晋级评估，必须用固定的 `start_time`、`end_time` 和完整策略
切片 `group_id + model + request_class + policy_version + activation_id` 查询；服务端强制只统计
`policy_mode=shadow`，不允许附带 `evaluated/diverged/emergency/reason/request_id`
等会美化样本的结果过滤器。它自动检查：

- 查询窗口不少于 72 小时；由于查询右边界不包含在窗口内，真实首末有效决策跨度
  不少于固定的 71 小时。延期复评为收集 200 个自然独立新会话而扩大窗口时不再
  无限抬高跨度要求，但最后一个有效决策必须在最近 24 小时内；24 小时只作为早期
  健康检查点；
- 查询 `window_start` 必须精确等于该 activation 持久化的 `shadow_started_at`，并且
  整个切片只能有一个非空 activation 和一个非空 T0；
- 仅以有效评估计算相对 T0 的覆盖：至少 36 个独立小时、3 个北京时间日期并覆盖
  全部 4 个六小时时段；目标固定，避免文本粘性让延期窗口永远无法完成；
- 至少 200 条成功评估决策，评估完整率和真实结果关联率均不低于 99%；
- `no_shadow_candidate` 作为显式 fail-closed 弃权单列，只有占全部决策不超过 5% 且
  后续 Canary 已验证原子回退 Legacy 时才可接受；timeout/取消/策略错误等仍计为
  不完整评估；
- 成功与失败不能同时关联，审计写入、被动采集和健康状态应用完整率均不低于 99%，
  且审计队列已经排空；
- 同一策略版本只有一个归一化策略快照，没有应急预算决策；
- 建议账号、建议故障域和建议端点的计数都必须与有效评估总数完全相等，缺失建议不能稀释集中度；
- Shadow 建议账号与故障域集中度不超过该快照中的账号/供应商份额上限。

即使所有自动门禁通过，返回值也只会是
`automated_evidence_ready_for_manual_review` / `consider_1_percent_canary`，并且固定
`manual_approval_required=true`、`enforce_available=false`。权威上游账单一致性、
相对 Legacy 的最终用户错误率、恢复率、P95/P99 延迟、文本粘性/图片无粘性以及
老板授权仍是人工门禁；该接口不会写设置、调度账号或改变任何真实流量。
其中决策、结果关联和集中度严格按策略切片统计。审计/被动采集健康由 PostgreSQL
持久进程 epoch 证明，并以
`health_sampling_scope=durable_process_epochs_overlap_conservative` 标识。正常发布通过
清洁封存与新 epoch 衔接保留连续性；未清洁 epoch、超过 90 秒的空档或历史失败都
阻止晋级。实验 T0 起的 `stats` 永久保留用于分析；若持久 epoch 在实验中途首次
上线，自动门只使用 `promotion_evidence_start` 之后的 `promotion_evidence_stats`，
既不删除旧数据，也不把无法追溯的迁移前完整率冒充为 100%。
两类存储探针从计数器启动后还必须保持零失败；后续探针恢复成功不能抹掉此前可能
跳过 Shadow 评估或被动观测的证据空档。

响应还返回只读 `review_schedule`：以真实证据起点计算 24 小时检查点、72 小时主
评估和失败后的 24 小时复评间隔，固定 `automatic_promotion=false`。该字段只是供
获授权后的外部一次性任务使用；接口自身不创建定时器。完整流程见
`docs/ops/OPENAI_INTELLIGENT_ROUTING_V2_ROLLOUT.md`。

## 生产开启门禁

必须按以下顺序执行，严禁同一次发布直接进入 `enforce`：

1. 代码和迁移上线时，`openai_route_policies` 保持缺失或全部 `legacy`。
2. 验证应用 `/health`、审计健康 `ready=true`，并确认存储探针没有残留行。
3. 再单独写入版本化 Shadow 策略；策略必须显式包含分组、模型、成本目标、成本
   硬上限、估算基础成本、`policy_version`、本轮唯一 `activation_id` 和写入当时的真实
   UTC `shadow_started_at`。不得预填预计上线时间，也不得复用已停止周期的 activation。
4. 使用自有测试身份对每个目标分组发起真实请求，确认：
   - 用户实际账号仍等于 Legacy 选择；
   - 每次命中评估均有决策行；
   - `attempted = written + failed + in_flight`，排空后 `in_flight=0`；
   - 决策可通过两种 request id 回查；
   - 统计中的成功、失败、未关联结果能解释总样本数。
5. 连续观察至少 72 小时真实流量的错误率、P95/P99 TTFT、切换耗时、建议账号占比、实际倍率、
   应急预算和账本一致性。样本不足或审计不完整时不得进入灰度接管。

## 回滚

回滚不依赖重新部署：把对应策略 `enabled=false` 或 `mode=legacy`，并使设置缓存
失效即可。历史决策表只追加保留，不能因回滚删除；它是判断策略为什么回滚的
证据。
