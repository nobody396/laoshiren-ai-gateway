# OpenAI 智能路由 V2 基础模块交接

## 交接快照

- 北京时间：2026-08-12 22:38 后完成本轮验证。
- 独立 worktree：`worktrees/intelligent-routing-v2-foundation-20260812`
- 分支：`feat/intelligent-routing-v2-foundation-20260812`
- 起始基线：`origin/main@2daa4daea`
- 本轮代码提交：
  - `6c576e61f` — 文本/图片策略、健康、预算与审计身份隔离；
  - `62ebd30f9` — 共享滚动观测、真实尝试采集、文本成本回填和 Shadow V2 评分；
  - `bff8f37c9` — 被动故障域关联、健康写入完整性与管理健康指标。
- 当前 `origin/main` 已前进到 `c56ed6a5f`；按统一发布协调要求，本轮没有
  rebase、merge 或 cherry-pick。
- 当前没有 PR，也没有该分支的 GitHub Actions 运行。

## 已实现范围

1. 完整路由身份：分组、账号、模型、请求类型、端点哈希、传输和显式故障域。
2. 文本保持已有会话粘性；图片为无粘性的单次选择，两类证据、预算和健康完全隔离。
3. 每一次真实上游尝试（包括被切换掩盖的失败）异步写入共享 Redis：
   - 最近一小时的 5 分钟桶；
   - 最近七个北京时间自然日；
   - 过去八周相同的北京时间周内小时。
4. 聚合指标包括可靠性样本、分类失败、TTFT、完成延迟、半截流、最后观测时间和
   权威结算成本。文本成本使用账号统计的同源 usage 结算；图片/视频成本未接入学习。
5. Shadow 使用 Wilson 可靠性下界、P90 TTFT、P95 完成延迟、半截流、负载、排队、
   倍率、优先级、近期份额和有界探索；所有输入、因子、排除原因和选择均进入审计。
6. 真实结果驱动最窄路由健康。只有显式同故障域内至少两个不同账号，在同一故障
   窗口内出现基础设施类失败，才会升级供应商健康；账号级限流、密钥、模型、余额、
   用户错误和本地传输错误不会扩散。
7. 采集存储失败、队列丢弃、输入拒绝与健康状态写入失败分别计数；证据完整率低于
   99% 时管理健康状态不允许为 Ready。

## 不变量与当前安全边界

- 用户实际选择仍由 Legacy 调度器决定；`enforce` 继续硬禁止。
- 没有修改生产账号状态、Base URL、分组、倍率或 `openai_route_policies`。
- 没有运行生产探针、生产定时任务、迁移或部署。
- 没有推送分支、创建 PR、合并 main 或触发 CI。
- 此前为自主开发创建的本地 2 小时心跳已按协调要求删除；当前没有本任务定时器。
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
```

真实 Redis 的 integration suite 已保留相同故障域用例，但本轮未启动 Docker；普通
单元测试使用 miniredis 覆盖相同 Lua 行为。

## 统一发布后再做

1. 重新读取最新 `origin/main`，审查图片计价和 Windows 修复与本分支的交叉点，再
   选择 rebase；不要在统一发布进行中合并。
2. 补充非敏感长期聚合检查点，以及只由明确授权启动的单主半开探针。
3. 基于 rebase 后提交创建 PR，等待全量 CI；代码发布仍保持策略缺失或 Legacy。
4. 获得单独生产授权后才部署 Shadow，并以自有测试身份验证审计、usage 关联和
   采集完整率。
5. 满足 24–72 小时、每策略切片至少 200 条有效决策、关联完整率不低于 99%、
   账单一致且错误率/P95/P99 不退化后，另一个版本才可实现和讨论 Enforce/Canary。

