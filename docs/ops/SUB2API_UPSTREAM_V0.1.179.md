# Sub2API v0.1.179 上游正式版审计

更新时间：2026-08-21（北京时间）

## 1. 版本与边界

| 项目 | 值 |
| --- | --- |
| 上次审计正式版 | `v0.1.178`；同分支 `SUB2API_UPSTREAM_V0.1.178.md` |
| 本地审计基线 | `origin/main@ab317e4c29b7b925250829449c7c409ce277261a` |
| 最新正式 Release | `v0.1.179`，2026-08-20 15:06:32（北京时间）发布，`draft=false`、`prerelease=false` |
| Release URL | https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.179 |
| annotated tag object | `3c28fad50472b409e18666df617f4237d8ba7007` |
| tag 目标 commit | `75f88be5f75c27771836b586f7de1503afa0e3bc` |
| 审计区间 | `v0.1.178..v0.1.179` |
| 区间规模 | 78 commits（49 non-merge）、212 files、+9,567/-1,446 |
| 复用 Draft PR | `#210` / `codex/sub2api-v0.1.178-audit-20260820` |

本轮只认 Wei-Shaw/sub2api 的正式 GitHub Release/tag，没有跟随 `upstream/main`，
没有整树 merge/cherry-pick。由于 `v0.1.178` 的三个本地重写仍在同一个 Draft PR
中等待合并，本轮先把该单提交重放到最新 `origin/main`，再对 `v0.1.179` 增量做
完整适用性分类。为降低回归风险，`v0.1.179` 本轮只审计、不扩大生产代码范围。

## 2. 适用性矩阵

状态含义：`applicable` 可进入独立本地实现；`already-covered` 本地已有等价或
更强合同；`local-rewrite-needed` 方向有价值但不能直接搬；`not-applicable`
产品/协议不同；`defer` 当前证据或风险门不足。

| 风险域 | 上游 commits | 改动摘要 | 本地 locator | 状态 | 决策、迁移/回滚与验证边界 |
| --- | --- | --- | --- | --- | --- |
| B2 OpenAI capacity recovery | `c3063e01a`、`539064798` | message-only / request-scoped capacity 失败恢复，覆盖 HTTP、WS 与首次输出边界 | `backend/internal/service/openai_capacity_shed_test.go`、`openai_ws_http_bridge_failover_test.go`、handler failover loop | `already-covered` | 本地已有“未输出可换号、已输出不重放”和 capacity 错误净化；不以不同上游架构替换本地状态机。验证继续由现有 capacity/WS failover suites 承担。 |
| B2 Responses→Chat reasoning 回放 | `612436a5a`、`401dd43b4` | 按 reasoning item id 缓存并在链式工具调用回注 `reasoning_content` | 本地 `backend/internal/pkg/apicompat`、Responses/Chat fallback、prompt-cache-key 合同 | `local-rewrite-needed` + `defer` | 会改变跨 turn 状态、缓存 key、工具回放和账务输入；无迁移但需独立 PR，覆盖乱序、重复 item id、跨账号重试、WS/HTTP 和缓存失效。 |
| B4 移除 Sora 遗留 | `7e45634df` | 删除已移除平台的 README/配置/UI 文案 | 本地保留独立 Grok 图片/视频产品和本地配置模板 | `not-applicable` | 上游平台生命周期与本地媒体合同不同；不删除本地有效功能。 |
| B3 usage 聚合单扫描 | `a9514a68d` | 聚合统计一次扫描并新增 migration 226 索引 | 本地 usage repository、账单日报、route/finance 统计；本地 migration 最新为 196 | `local-rewrite-needed` + `defer` | 读放大优化方向有价值，但 SQL、请求类型和 migration namespace 已分叉；必须独立基准、EXPLAIN、checksum/notx 和日报/对账回归，不能直接引入上游 226。 |
| B4 版本/CI/README | `49504adc9`、`3d21d6160`、`85cb732c` | 版本号、CI 重跑、README star 图 | 本地版本/CI/README | `not-applicable` | 没有产品语义；不回移。 |
| B3 渠道配额监控 | `1128df259`、`c41ae19e5`、`e2dfb3b8c`、`c9effc456`、`2c250bfd7` | scheduler 对齐、数据源校验、single-load、表单/占位模型 UI | 本地 monthly probe、账号健康、上游 benchmark 和飞书半小时报 | `local-rewrite-needed` + `defer` | 本地已经采用不同的探针/真实流量双口径；若有需求需独立定义调用成本、缓存、调度一致性和 UI，不与网关 bugfix 混合。 |
| B2/B4 CN provider 账号测试 | `ac6208de1` | 国产供应商 chat test 路由 | 本地 provider manifest/账号测试合同 | `not-applicable` | 未批准引入对应供应商；不扩大供应商面。 |
| B2 Chat 非流式读取故障转移 | `b228b93e9` | buffered SSE 读取失败时生成可换号传输错误 | `openai_gateway_chat_completions.go`、`openai_gateway_messages.go`、本地 failover loop | `local-rewrite-needed` + `defer` | 本地方言仍在各端点内维护 scanner；方向适用，但必须保证客户端未收到任何字节、`bufio.ErrTooLong` 不重放、请求取消不换号，并新增故障注入测试。独立 PR，可整体 revert。 |
| B2 Responses input token preflight | `bfac49fef` | 新增 Responses input-token 预检/路由和 ops 归类 | 本地 count_tokens、endpoint guard、计费/ops 归类 | `local-rewrite-needed` + `defer` | 新入口会触及 API surface、鉴权、计费排除和告警；需独立合同与真实 Codex 请求证据，不在清理 PR 中启用。 |
| B2 Grok 4.6 xhigh | `892787723` | 保留 Grok 4.6 的最高 reasoning effort | `openai_gateway_grok.go`、`openai_gateway_grok_test.go` | `already-covered` | 本地仅对不支持 reasoning 的 Composer 模型删除相关字段，4.5/4.6 保留；补丁语义已由本地模型能力测试覆盖。 |
| B2/B3 Composite/CN 扩展 | `58e147fba`、`b171bb0e4`、`4d3b300a2`、`aa673062e`、`499a8ee4` | Composite 支持 Codex/CN、多入口门禁和 migration 227 | 本地 group/scheduler/model catalog、31 天月卡和供应商 manifest | `not-applicable` | 会改变分组、调度和迁移合同，且无已批准产品需求；不回移。 |
| B2 WS 客户工具与 429 后续 turn 恢复 | `b94e484e2`、`e4896c41d`、`fefd0d514`、`82cbe6aff` | 跨 WS turn 保留客户工具，429 后恢复后续 turn | 本地 `responses_client_tools_support.go`、WS turn-state/protocol/failover tests | `local-rewrite-needed` + `defer` | 本地已有工具映射和首 turn 429 failover，但后续 turn 的 ownership、账务一次性和账号处罚需单独证明；不能把上游 WS 状态机直接覆盖本地实现。 |
| B4 前端小修 | `f917d19d3`、`994fbfedd`、`63839f193`、`1f2a87adb` | Grok placeholder、CN quota 布局、角色样式、平台筛选 | 本地前端设计系统和 provider 目录 | `not-applicable` | 不属于安全/账务/稳定性阻塞；不为 UI 差异扩大本 PR。 |
| B3 Proxy probe 目标 | `b0464a986`、`d5484866f`、`ec5a34593`、`1ab325678` | 配置化并校验代理连通性探针 URL | 本地 Hostinger/EdgeOne/上游 benchmark 与网络安全边界 | `local-rewrite-needed` + `defer` | 可配置外连目标扩大 SSRF/网络边界；需 allowlist、DNS/IP 重绑定防护和生产配置迁移计划后独立实现。 |
| B2 Grok 图片工具 | `99a8b8470`、`b0cdea303` | 避免 `view_image` 冲突并覆盖多入口内联图片工具 | `openai_gateway_grok_cache*.go/test`、Grok image bridge/tool protocol tests | `already-covered` | 本地已有原生图片意图、`view_image` 共存和多入口工具测试，并绑定本地计费/图片归属；不覆盖。 |
| B2 CN header override | `1b30a2d74` | 国产供应商自定义 header | 本地账号凭据/header override 与 provider manifest | `not-applicable` | 未批准供应商，不扩大凭据/header 权限面。 |
| B1 渠道分层/区间倍率/Fast | `fce90ecf8`、`5b2a386ed`、`7dae055f2`、`26be82cc8`、`d536795e9`、`d4d2c746c` | migration 228、渠道倍率、上下文区间、Anthropic Fast 和前端 | 本地 `model_pricing_resolver.go`、AccountingCommand、31 天月卡、返佣与公开定价 | `local-rewrite-needed` + `defer` | 资金热路径；上游 schema、精度、时区和 migration 均不兼容。必须独立财务设计、双算/对账、回滚和真实最小支付验证，绝不进入本 PR。 |
| B2 Grok tool-search | `5b2089c5a`、`9ede0f716` | 将 tool-search discovery 降级/提升为可调用客户工具 | `responses_client_tools_support.go`、`openai_gateway_grok_tool_protocol_test.go` | `already-covered` | 本地已有 built-in 与自定义工具冲突 fail-closed、call/output/stream/terminal 映射及命名空间测试；保留本地协议。 |
| B2 Ops SLA 归类 | `6b0ec50f2` | 模型配置错误不计 SLA | PR #212/#213；`ops_error_logger.go`、`ops_alert_diagnostics.go` | `already-covered` | 本地已把 endpoint/model configuration 归 client，并在告警诊断中排除 client、business、count_tokens、probe 和 recovered；生产读回正常。 |
| B2/B4 自适应 API 协议 | `85051616f`、`b3092145d` | CN 账号自动选择协议并改前后端 | 本地账号协议、model mapping、OpenAI/Gemini/Grok 独立路由 | `not-applicable` | 会改变真实调度和账号测试，不属于本地已批准智能路由 Shadow 合同；不回移。 |

## 3. 本轮实际代码范围

`v0.1.179` **没有新增生产代码进入 PR #210**。PR #210 仍仅包含：

1. v0.1.178 Anthropic SSE overload 的本地 529 语义修复；
2. v0.1.178 Gemini mixed tools typed-transform 修复；
3. v0.1.178 Ops 批量写失败不逐条重放；
4. 一个已合并 PR #196 遗留的 test-only 函数签名同步；
5. v0.1.178 与 v0.1.179 两份审计文档。

这使合并单元保持可整体 revert、无 migration、无前端、无新供应商、无支付/
月卡/返佣变化。v0.1.179 中有价值但尚未满足安全合同的 buffered read failover、
Responses token preflight、WS later-turn resume、usage 聚合和渠道倍率均记录为独立后续，
不为了“跟最新版”而扩大当前发布风险。

## 4. 验证边界

在最新 `origin/main@ab317e4c29b7b925250829449c7c409ce277261a` 重放单提交后：

- `git diff --check origin/main...HEAD`：通过；
- `go test ./internal/pkg/antigravity ./internal/service -count=1`：通过；
- `go test ./... -count=1`：通过；
- `go test -tags=unit ./... -count=1`：通过；
- 两组相关 `go test -race`：通过；
- `go vet ./...` 与 `go vet -tags=unit ./...`：通过；
- `/Users/fujunhao/go/bin/golangci-lint` v2.12.2 使用独立 cache 执行
  `run ./...`：`0 issues`；
- `make test-backend-integration`：通过；首次 CI 中失败的 affiliate/native-checkout
  用例和 Redis Testcontainers 初始化本次均通过；
- 尚需推送精确新 head 并等待 PR Required CI 全绿；CI 未全绿前不得合并；
- 本轮没有前端改动，因此前端/Windows 路径应由 changed-path gate 正确跳过；
- 以上均不是生产验证，本轮清理任务不部署生产。

## 5. 回滚和后续边界

- 当前 PR 无 migration/config/data repair；合并后可整体 revert 单一提交。
- deferred 项没有保留活动开发分支；只有本审计文档作为 future intake。
- 任何 deferred 项重新启动时必须从当时最新 `origin/main` 建新独立分支，不能复活
  本次清理掉的旧 worktree/branch。
