# OpenAI 智能路由 V2 基础模块交接

## 交接快照

- 北京时间：2026-08-13 00:52 后完成主线同步、第一轮 CI 与热路径复盘。
- 独立 worktree：`worktrees/intelligent-routing-v2-foundation-20260812`
- 分支：`feat/intelligent-routing-v2-foundation-20260812`
- 当前基线：`origin/main@beae9a8e1`（PR #143 合并提交）。
- 本轮代码提交：
  - `a7240e827` — 文本/图片策略、健康、预算与审计身份隔离；
  - `a1cf38ab1` — 共享滚动观测、真实尝试采集、文本成本回填和 Shadow V2 评分；
  - `cd1c15c00` — 被动故障域关联、健康写入完整性与管理健康指标；
  - `fbe7af455` — 只读 Shadow 晋级证据门禁；
  - `92a268bb8` — 共享观测 L1/singleflight；
  - `4faaea059` — route/provider 健康批量读取；
  - `a359c44df` — 候选级权威文本成本学习；
  - `f6e1c8662` — 72 小时主评估与只读复评计划；
  - `82e34c8db` — 真实 PostgreSQL 晋级策略快照断言。
  - `4dc2fe443` — Shadow 审计异步有界队列、端到端完整率门禁与客户端取消隔离。
- 两次 rebase 均在统一发布明确授权后执行；`range-diff` 证明原七个提交内容未改变，
  最新图片价格表 PR 仅改前端，与本分支无交叉。
- PR：[#144](https://github.com/nobody396/laoshiren-ai-gateway/pull/144)。第一轮完整 CI
  已通过；后续每个新增提交都必须以 PR 当前精确 HEAD 的新一轮绿色 CI 为准。

## 已实现范围

1. 完整路由身份：分组、账号、模型、请求类型、端点哈希、传输和显式故障域。
2. 文本保持已有会话粘性；图片为无粘性的单次选择，两类证据、预算和健康完全隔离。
3. 每一次真实上游尝试（包括被切换掩盖的失败）异步双写共享存储：Redis 保存最近
   一小时的 5 分钟桶；PostgreSQL 按小时保存非敏感检查点，权威恢复最近七个北京
   时间自然日和过去八周相同的北京时间周内小时。
4. 聚合指标包括可靠性样本、分类失败、TTFT、完成延迟、半截流、最后观测时间和
   权威结算成本。文本成本使用账号统计的同源 usage 结算；图片/视频成本未接入学习。
5. Shadow 使用 Wilson 可靠性下界、P90 TTFT、P95 完成延迟、半截流、负载、排队、
   倍率、优先级、近期份额和有界探索；所有输入、因子、排除原因和选择均进入审计。
6. 真实结果驱动最窄路由健康。只有显式同故障域内至少两个不同账号，在同一故障
   窗口内出现基础设施类失败，才会升级供应商健康；账号级限流、密钥、模型、余额、
   用户错误和本地传输错误不会扩散。
7. 采集存储失败、队列丢弃、输入拒绝与健康状态写入失败分别计数；证据完整率低于
   99% 时管理健康状态不允许为 Ready。
8. 增加精确策略切片的只读 Shadow 晋级评估：固定窗口、样本、关联、审计/采集健康、
   策略版本纯度、应急预算及账号/故障域集中度均为显式门禁；通过也只允许进入人工
   复核，不会自动开启 1% 流量。
9. 共享观测热路径增加 5 秒有界 L1 与 singleflight：相同候选集合并发只回源一次，
   20ms 超时、错误不缓存、深拷贝返回；Redis 负责近期窗，PostgreSQL 负责长期窗，健康熔断仍每次评估读取；缓存
   命中/回源/失败/淘汰指标进入管理健康响应。
10. 每次 Shadow 评估的 route/provider 健康读取由顺序 `2N` 次 Redis 请求改为一次
    pipeline；共享故障域自动去重，缺失状态按 warmup 处理，非法/损坏/缺项均
    fail-closed 回到 Legacy。健康 pipeline 与共享观测读取并发执行，任一失败即取消
    本次 Shadow 评估。
11. 文本候选在同一完整路由身份积累至少 20 条七日权威结算后，使用 20 条等效先验
    做收缩估算并限制在配置值 `0.25x--4x`。评分、预算预览、Redis 原子预留和 Shadow
    结算使用同一个候选估算；图片/视频始终保留配置值。来源、样本、均值和估算全部
    进入审计。
12. 24 小时只作为健康检查点，自动证据主评估要求查询窗口达到 72 小时；考虑查询
    右边界不包含且真实流量不与边界同步，实际首末有效决策跨度至少固定 71 小时。
    延期复评扩大查询窗口收集 200 个自然独立新会话时不再无限抬高跨度要求，同时
    要求最近 24 小时内仍有有效决策，并固定要求 36 个独立小时、3 个北京时间日期、
    全 4 个六小时时段。响应给出确定性的 24h/72h/失败后 24h 复评计划，但不创建定时器且固定
    `automatic_promotion=false`；完整协议见 `OPENAI_INTELLIGENT_ROUTING_V2_ROLLOUT.md`。
    `no_shadow_candidate` 另按 fail-closed 弃权统计，上限 5%；只有未来 Canary 已证明
    原子回退 Legacy 时才可接受，timeout/取消/策略错误仍属于评估不完整。
13. Shadow 决策写库从账号选择热路径移入 8 worker / 4096 queue 的有界队列。提交
    成功即返回 Legacy 选择；排队、失败与丢弃可观测，服务退出时排空已接受任务。
    审计完整率以全部尝试（含在途）为分母，被动观测完整率也把在途与拒绝纳入分母；
    健康状态应用另有独立完整率，三者任一低于 99% 都阻止 72 小时晋级。
14. 客户端主动 `context.Canceled` 被标为 `client_cancelled`：保留非敏感计数，但不进入
    上游可靠性、延迟或半截流评分分母，不触发线路熔断，也不升级共同供应商故障域。
15. 72 小时账号集中度按账号汇总，不按当时倍率拆行；同一账号在观察期内发生倍率
    变化时也不能借由多个统计行低估其总建议份额。
16. 审计与被动采集完整率按进程 epoch 持久化；正常发布会清洁封存旧 epoch 并由新
    epoch 连续衔接，不重置实验 T0。若该能力在已运行实验中途首次上线，响应保留
    T0 起的完整 `stats`，同时把两类 epoch 都可证明的最晚起点作为一次性的
    `promotion_evidence_start`；晋级门只使用其后的 `promotion_evidence_stats`。
    未清洁 epoch、超过 90 秒的交接空档或存储失败必须阻止晋级，不能靠新进程的
    100% 完整率抹掉旧损失。
17. 有效评估的建议账号计数和建议故障域计数都必须完整覆盖有效评估总数；任何缺失
    建议都会触发 `adaptive_selection_completeness`，不能通过缺行稀释集中度。
18. `/v1/messages` 兼容分发使用实际调度模型作为线路观测身份；真实请求模型继续写入
    用量日志，但成功后的权威成本样本与 Shadow 候选、线路尝试落入同一个映射模型桶。

## 不变量与当前安全边界

- 用户实际选择仍由 Legacy 调度器决定；`enforce` 继续硬禁止。
- 没有修改生产账号状态、Base URL、分组、倍率或 `openai_route_policies`。
- 没有运行生产探针、生产定时任务、迁移或部署。
- 已推送独立分支并创建 PR #144；没有合并 main，也没有触发生产发布。
- 此前为自主开发创建的本地 2 小时心跳已按协调要求删除；当前没有本任务定时器。
- 晋级评估固定返回 `manual_approval_required=true`、`enforce_available=false`，不会
  写策略或执行账号调度。
- 决策和滚动聚合不保存 Prompt、响应正文、凭证、账号名称或原始 URL。

## 已通过验证

在该 worktree 的 `backend` 目录执行并通过：

```text
go test ./internal/service -count=1 -timeout=10m
go test ./internal/repository -count=1 -timeout=10m
go test ./internal/handler/... -count=1 -timeout=10m
go test ./cmd/server -count=1 -timeout=10m
go test -race ./internal/service -run 'Test(OpenAIRouteController|OpenAIRouteObservationCollector|BuildOpenAIRouteAllocationPlan)' -count=1 -timeout=10m
go test -race ./internal/repository -run 'Test(OpenAIRouteHealthCache_ProviderEvidenceRequiresDistinctAccounts|TestOpenAIRouteObservationCache)' -count=1 -timeout=10m
go test ./internal/service ./internal/repository ./internal/handler/admin ./internal/server ./cmd/server -count=1 -timeout=10m
go vet ./internal/service ./internal/repository ./internal/handler/admin ./internal/server ./cmd/server
go test -race ./internal/service -run 'Test(BuildOpenAIRoutePromotionAssessment|ValidateOpenAIRoutePromotionFilter|OpenAIRouteAuditService|OpenAIRouteObservationCollector)' -count=1 -timeout=10m
go test -race ./internal/repository -run 'Test(OpenAIRouteDecisionRepositoryStatsScansPromotionEvidence|BuildOpenAIRouteShadowWhere)' -count=1 -timeout=10m
go test -race ./internal/handler/admin -run 'Test(OpenAIRoutePromotionAssessmentHandler|ParseOpenAIRoutePromotionAssessmentFilter)' -count=1 -timeout=10m
go test -race ./internal/service -run 'Test(OpenAIRouteController|OpenAIRouteObservationProfileCache)' -count=1 -timeout=10m
go test ./internal/service -run 'TestOpenAIRouteObservationProfileCache(CoalescesConcurrentMisses|CallerCancellationDoesNotPoisonWarmup|ExpiresAndStaysBounded)' -count=50 -timeout=10m
go test ./internal/service -run '^$' -bench '^BenchmarkOpenAIRouteObservationProfileCacheHit$' -benchmem -benchtime=200000x -count=3
go test -race ./internal/service -run 'TestOpenAIRouteController' -count=1 -timeout=10m
go test -race ./internal/repository -run 'TestOpenAIRouteHealthCache' -count=1 -timeout=10m
go test -race ./internal/service -run 'Test(EstimateOpenAIRouteBaseCost|BuildOpenAIRouteAllocationPlan|AllocateAndReserveOpenAIRoute|OpenAIRouteController)' -count=1 -timeout=10m
go test -race ./internal/service -run 'Test(BuildOpenAIRoutePromotionAssessment|ValidateOpenAIRoutePromotionFilter)' -count=1 -timeout=10m
go test ./...
golangci-lint run ./...  # v2.12.2, 0 issues
make test-backend-integration
```

本机 Apple M3 的缓存命中基准三次为约 `1.18–1.28us/op`；这是本地微基准，不替代
生产 Shadow 的 `evaluation_duration_us` 和 Redis 延迟观测。

真实 PostgreSQL 18.1 与 Redis 8.4 的确定性 Testcontainers 集成门禁已执行；首次运行
发现晋级快照 fixture 未写份额上限，修复后完整 integration suite 和 sentinel 通过。
Testcontainers 退出后本机容器为空。

## PR 与后续独立授权阶段

1. PR #144 更新后等待全量 CI；代码发布仍保持策略缺失或 Legacy。
2. 依赖 PR 已补充非敏感长期聚合检查点和硬关闭的单主半开探针 runner；真实 probe
   client/扫描编排仍需后续独立发布与明确授权。
3. 获得单独生产授权后才部署 Shadow，并以自有测试身份验证审计、usage 关联和
   采集完整率。
4. 以真实 Shadow 起点创建 24 小时检查与 72 小时主评估的一次性任务；未通过则继续
   Shadow 并按 24 小时间隔复评，不自动调参或切流。
5. 满足 72 小时、每策略切片至少 200 条有效决策、关联完整率不低于 99%、账单一致
   且错误率/P95/P99 不退化后，只能申请人工 1% 灰度授权；Enforce/Canary 仍属于另一
   版本和另一发布窗口。
