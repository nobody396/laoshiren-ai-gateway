# 智能路由 V2 完成度审计与剩余交付

> 审计时间：2026-08-13（北京时间）  
> 基线：PR #144 精确提交 `5be68095a934752a5f05837fda39e6af9c9b999c`，基于
> `main@5a6a3ad630f1619671dbdfaf2a1b5a22b23a9556`。  
> 本文只描述代码与证据，不代表任何生产授权。

## 不可改变的目标

完整目标不是“写一个加权随机数”，而是形成闭环：

```text
真实请求被动采集
  -> 完整线路身份和故障域聚合
  -> 非平稳时段画像与保守评分
  -> Shadow 反事实建议和可关联审计
  -> 24h/72h 数据质量与用户体验门禁
  -> 人工批准的 1%/5%/20%/50%/100% 确定性放量
  -> 每档复评、止损和一键 Legacy 回滚
```

文本请求必须保持现有粘性；图片请求必须按单次逻辑请求无粘性调度。任何自动计算都
只能提出建议或触发只读复评，不能自动跨越真实流量档位。

## 逐项证据审计

| 要求 | 当前权威证据 | 结论 |
|---|---|---|
| 完整路由身份 | `OpenAIRouteKey` 包含 group/account/model/request_class/endpoint_hash/transport/failure_domain；迁移 181 和单测覆盖 | 已实现 |
| 文本/图片隔离 | request class 进入健康、预算、观测、策略和审计；文本沿用 session sticky，图片使用单请求分类 | 已实现 |
| 持续被动采集 | 每次真实上游 attempt 进入有界 collector；Redis 保存 1h 快窗，PostgreSQL 小时聚合恢复 7 个北京时间自然日与 8 周同周内小时 | 本分支代码就绪；双写任一失败都会使证据完整率失败关闭 |
| 动态评分 | Wilson 下界、P90 TTFT、P95 完成延迟、半截流、负载、排队、倍率、优先级、份额和有界探索均进入快照 | 已实现 |
| 成本约束 | 文本至少 20 条权威结算后用 20 条先验收缩并限制 0.25x--4x；图片不从 token 样本推成本 | 已实现 |
| 路由健康 | 真实结果驱动 route circuit，失败分类保持最窄作用域 | 被动部分已实现 |
| 故障域 | 只有同一显式故障域内至少两个不同账号的基础设施类失败才能升级 provider circuit | 已实现 |
| 单主半开探针 | Redis permit/续租、只对到期 open route 执行的 runner、独立统计与状态机已有；编译期开关为 false，未接真实 client/cron | 本分支代码就绪，生产保持关闭 |
| Shadow 审计 | 有界异步写库、usage/error 关联、完整率、策略快照和选择因子审计 | 已实现 |
| 72h 连续证据 | 原门禁只看首末时间，两个边缘突发可能伪装成连续观测 | 本分支新增相对 T0 的有效小时桶覆盖门禁 |
| 激活周期身份 | migration 182、策略校验、审计字段和评估精确切片共同绑定 activation_id/T0 | 本分支已实现；历史行不能晋级 |
| 分阶段放量 | PR #144 硬禁止 enforce，没有确定性 1/5/20/50/100 cohort | 本分支新增纯合同与 cohort 原语，但仍硬禁止真实流量 |
| 文本粘性/图片单次 cohort | 尚无 canary cohort | 本分支新增 text_affinity 与 image_request 两种不可混用的确定性桶 |
| 一键回滚 | Shadow 可回 Legacy；未来 Canary 未实现 | 本分支新增不依赖遥测/存储的回 Legacy 转移合同；尚未接入控制面 |
| 自动心跳/复评 | 评估仅返回 T0+24h/T0+72h 计划，不保存执行历史、不创建任务 | 本分支新增纯只读 heartbeat 状态机；持久化与一次性唤醒仍待 T0 授权后接入 |
| 生产 Shadow 与 T0 | 当前生产策略、探针、cron、Codex 任务全部关闭 | 未授权，不能执行 |

## 本分支新增的安全修正

### 0. Activation 与真实 T0

- 每次已启用 Shadow 必须显式携带不可复用 `activation_id` 和 UTC RFC3339
  `shadow_started_at`；缺失、非法、非 UTC 或未来时间均 fail closed；
- migration 182 把二者作为顶层非敏感审计字段持久化，快照同时保留副本；历史数据
  维持空 ID/NULL T0，不能伪装成新一轮证据；
- 列表、统计和只读 assessment 支持 activation 精确切片；晋级门禁要求唯一 ID、唯一
  T0，并要求 `window_start == shadow_started_at`；
- 因此同一 `policy_version` 停用后再启用，也不能把两段流量拼成 72 小时连续窗口。

### 1. 连续小时覆盖

首末决策相距 71 小时并不能证明中间 71 小时有证据。统计查询现在只用
`evaluated=true` 的决策计算：

- 相对 `window_start` 的 distinct one-hour buckets；
- 有效决策的 `MIN(created_at)` 与 `MAX(created_at)`。

72 小时主评估至少覆盖 71 个相对小时桶；扩大复评窗口时仍只允许缺一个小时桶。
因此“两端各一批请求，中间完全没有数据”的切片不能晋级。

### 2. 默认关闭的确定性放量合同

新增 `OpenAIRouteRolloutStage`：

```text
legacy -> shadow -> canary_1 -> canary_5 -> canary_20 -> canary_50 -> canary_100
```

- 只能逐档前进，任何档位都可直接回 `legacy`；
- 每次前进都要求与 activation/policy/request_class/目标档位完全一致的证据和独立人工
  批准；批准不能跨档复用；
- Shadow 到 1% 至少需要 72 小时证据；后续每档至少驻留并评估 24 小时；
- 引入 dormant enforce 代码的 release 与实际启用 release 必须不同；
- `OpenAIRouteEnforceCodeAvailable=false` 仍是编译期硬门。当前代码即使得到一份
  canary 配置，也只能计算 cohort，不能授权真实 Adaptive 选择。

确定性桶固定为 10,000 个，扩容百分比只提高阈值，不改变 hash 输入，因此 1% 已入桶
的会话在 5%/20%/50%/100% 阶段仍保持入桶：

- 文本：使用现有 session/previous-response affinity key；缺失时 fail closed 到 Legacy；
- 图片：使用单次逻辑 request ID，重试保持同桶，不跨请求建立粘性；
- 审计只需要记录 bucket/basis，不保存原 affinity key。

### 3. 只读 heartbeat 状态机

`BuildOpenAIRouteReviewHeartbeat` 只计算下一个到期动作：

1. 等待真实 T0；
2. T0+24h 健康检查；
3. T0+72h 主评估；
4. 未通过后每 24h 一次只读复评；
5. 通过后停在人工评审，不自动晋级；
6. stop/rollback 后不再安排动作。

它不创建 timer、不写策略、不访问凭证。真正的一次性 Codex 唤醒只能在生产 Shadow
得到单独授权并记录真实 T0 后创建。

### 4. 默认关闭的单主半开探针

- `OpenAIRouteActiveProbesCodeAvailable=false` 是独立编译期门；公共 runner 在当前版本
  不会获取租约、更不会发出网络请求；
- 只有精确 route circuit 为 `open` 且 `OpenUntil` 已到期才有资格执行；进程内去重后
  还必须获取 Redis 单主 permit；运行期间只允许当前 owner 续租，失去所有权立即取消
  probe，且不写健康状态；
- Probe client 接口只接收不含 URL/凭证的 `OpenAIRouteKey`，未来实现必须固定为自有
  测试身份；主动结果只进入独立 probe 统计和 `Probe=true` 的健康转换，不进入真实
  用户的被动可靠性、TTFT、成本聚合；
- 当前没有 Wire 注入、扫描循环、cron 或生产 client，因此即使部署代码也不会主动
  探测。网络接线与生产启用仍需后续独立发布和老板明确授权。

## 算法依据

- Conservative Bandits 要求探索过程相对基线策略持续满足保守约束；本方案把 Legacy
  作为基线，把成本、错误率和延迟门禁置于探索之前：
  https://proceedings.mlr.press/v48/wu16.html
- Contextual Conservative Interleaving Bandits 强调逐轮安全约束，而不只是最终平均值；
  本方案因此保留每次请求硬过滤、预算和份额上限：
  https://proceedings.mlr.press/v202/takemura23a.html
- 非平稳 bandit 研究支持滑动窗口/折扣历史和显式变点处理；当前 1h/7d/同周内小时
  三层画像先采用可解释、可审计的保守实现，不直接上线黑盒在线自调参：
  https://arxiv.org/abs/2003.10113
- Google Canary Analysis Service 将部分流量、限时观测、指标评估和回滚分开；本方案
  同样不允许“指标通过”直接等于“自动放量”：
  https://research.google/pubs/canary-analysis-service/

## 剩余代码顺序

1. PostgreSQL 小时聚合检查点已经在本分支实现：只保存 route 指纹/维度、稀疏计数、
   直方图、权威成本合计与最后观测时间；Redis 只负责 1 小时快窗，长期窗口由数据库
   权威读取，避免双计。没有回填 timer 或 cron。
2. 在后续独立发布中实现自有测试身份 probe client 和扫描编排；接线后仍默认关闭，
   经授权才可启用。
3. 在另一发布版本中把 rollout contract 接入 scheduler；该版本先保持 Legacy/Shadow
   再观察，不能在引入接线的同一次部署启用 1%。
4. 获单独授权后才部署 Shadow、记录 T0、创建 T0+24h/T0+72h 一次性只读唤醒。

## 当前操作边界

- PR #144 保持 OPEN、未合并、未部署。
- 本分支只允许开发和测试；没有写生产设置、账号、Base URL、分组、倍率或调度状态。
- Shadow、Enforce、主动探针、cron、Codex 定时任务和 T0 全部保持关闭。
