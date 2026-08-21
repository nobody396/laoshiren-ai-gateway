# Sub2API v0.1.178 上游正式版审计与选择性回移

更新时间：2026-08-20（北京时间）

> 续审说明：正式 Release `v0.1.179` 已于北京时间 2026-08-20 发布；本分支已在
> 最新 `origin/main@ab317e4c29b7b925250829449c7c409ce277261a` 上重新验证，并追加
> `SUB2API_UPSTREAM_V0.1.179.md`。`v0.1.179` 没有新增进入本 PR 的生产代码，
> 本 PR 仍只包含本文件第 3 节列出的三个 v0.1.178 边界修复和一个 test-only
> 签名同步，避免把新的计费、迁移、WS 或供应商大改混入同一发布单元。

## 1. 版本与边界

| 项目 | 值 |
| --- | --- |
| 上次已审计正式版 | `v0.1.177`；依据 `origin/main:docs/ops/SUB2API_UPSTREAM_V0.1.177.md` |
| 本地审计基线 | `origin/main@6a76cb761020f546616bab740cca125f952dccd6` |
| 最新正式 Release | `v0.1.178`，2026-08-18 18:03:00（北京时间）发布，`draft=false`、`prerelease=false` |
| Release URL | https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.178 |
| annotated tag object | `15290e66c66801a7ce435a6d24b178ee9486f284` |
| tag 目标 commit | `e0c48a19ed794a565e3858662520afe0a1f9f0ba` |
| 审计区间 | `v0.1.177^{commit}@073e92d17178a1ccdb0a27017f572f10c9c7ab62..v0.1.178^{commit}` |
| 区间规模 | 107 commits（68 non-merge）、301 files、+19,904/-1,017 |
| 工作分支 | `codex/sub2api-v0.1.178-audit-20260820` |
| 临时工作树 | `/Users/fujunhao/laoshirenai/worktrees/sub2api-upstream-v0.1.178-20260820` |

本轮只认 Wei-Shaw/sub2api 的正式 GitHub Release/tag，没有追 `upstream/main`，没有 merge 上游分支或整树覆盖。`v0.1.177` 审计 PR #172 已在 2026-08-19 12:02:32（北京时间）合并，因此本轮以已合并的 `origin/main` 审计文档为本地基线，不使用旧自动化记忆代替实时读取。

## 2. 逐项适用性矩阵

状态含义：`applicable` 本轮回移；`already-covered` 本地已有等价或更强实现；`local-rewrite-needed` 方向适用但必须按本地合同重写；`not-applicable` 当前产品/协议不适用；`defer` 证据或风险门槛不足，拆到后续。

| 风险域 | 上游 release / commit | 改动摘要 | 本地对应代码 | 状态 | 风险、建议与所需测试 |
| --- | --- | --- | --- | --- | --- |
| B0 邀请码 TOCTOU | v0.1.178 / `b8642ef67` | 用户创建与一次性邀请码占用原子化 | `auth_service.go:withinTx`、`user_repo.go:transactionClientFromContext` | `already-covered` | 本地已在同一 UnitOfWork 中创建用户并执行条件 `Use`，失败回滚且拒绝注册；现有回滚/事务测试保留。 |
| B0/B2 Codex OAuth 身份收敛 | v0.1.178 / `6793d5ac8`、`bb6c3b4f6`、`a34123959`、`16e4f7ecc` | 统一凭据面、探针、模型发现和推理出站指纹 | `openai_codex_fingerprint.go`、`openai_codex_identity.go`、本地 HTTP/WS 转发链 | `local-rewrite-needed` + `defer` | 本地指纹、实际 Codex 协议和账号风控已高度分叉，且上游含 migration；需独立 PR 覆盖新老凭据、HTTP/WS、探针与真实 Codex E2E。 |
| B1 认证快照定价字段 | v0.1.178 / `674570ca1` | 在上游 group 快照中保留 long-context/model pricing | `api_key_auth_cache*.go`、`model_pricing_resolver.go`、`channel_service.go` | `already-covered` | 本地定价真值由 group ID 经 `ChannelService` 热缓存解析，不依赖上游 `Group.ModelPricing`快照；不把上游资金 schema 照搬进本地。 |
| B1/B3 渠道分时倍率 | v0.1.178 / `9f24a5530` | 渠道模型按时段倍率计费，WS 按 turn 起始时刻冻结 | `model_pricing_resolver.go`、本地动态成本路由、OpenAI WS 计费 | `local-rewrite-needed` + `defer` | 资金热路径且涉及北京/服务器时区、跨零点长连接和月卡/返佣；需冻结优先级、精度、时区、回滚和 HTTP/WS 对账合同后独立 PR。 |
| B1/B2 国产供应商入场 | v0.1.178 / `901a0439f`、`4b667ccd4`、`7cdca9e49`、`10c8b7020`、`8f6f45983` | Kimi/智谱/DeepSeek 多协议、调度、计费、余额/额度监控 | 本地供应商 manifest、渠道/账号路由和计费合同 | `not-applicable` | 属未验证新供应商/协议，不因上游一等支持就自动入场。 |
| B1/B4 Grok 聚合用量/别名 | v0.1.178 / `269fbcac0`、`c46d07ca0`、`fd42d3722` | 补 24h/7d/30d 本站聚合，统一响应模型别名，空值不展示 | 本地 Grok usage/billing/media 和账号页 | `local-rewrite-needed` + `defer` | 用量展示必须与本地账单真值一致；先定义响应模型归一和时间窗口不变式，不搬上游 UI。 |
| B2 Anthropic SSE overload | v0.1.178 / `76a13a5a8` | HTTP 200 内嵌 `event:error` 的 `overloaded_error` 在未输出前按 529 处理 | `gateway_service.go:Forward/handleStreamingResponse` | `applicable` | 本轮按本地单体网关重写；保留原始错误 body，仅未向客户端输出时触发 529 账号策略，已输出则保留原 403/failover 边界。 |
| B2 OpenAI API-key/终止事件自定义工具 | v0.1.178 / `44ef88f65`、`7e579cb28`、`c253bd2c7` | 在 API-key、WS-HTTP bridge 和 terminal events 中恢复客户工具 | `pkg/apicompat/responses_client_tools*.go`、`openai_gateway_grok_tool_protocol.go`、本地 WS bridge | `already-covered` | 本地已有客户工具映射、流恢复和终止事件测试；保留本地 Grok/OpenAI 协议边界。 |
| B2 Gemini 混合工具 | v0.1.178 / `cb5e03a72`、`3c3bb2fa1`、`971544570`、`1ba92449c` | 保留混合 tool config，在函数工具 + Google Search 时发送 `includeServerSideToolInvocations=true` | `pkg/antigravity/gemini_types.go`、`request_transformer.go` | `applicable` | 本轮按本地 typed transform 链回移；测试 mixed/function-only/search-only，避免无条件改变请求。 |
| B2 Gemini 4xx / Skipped policy | v0.1.178 / `ab0fcd1a0` | 避免把确定性 4xx 硬改 500，区分 pool/custom-code 并保真原生错误 | `gemini_messages_compat_service.go`、`gemini_chat_completions_compat_service.go`、`antigravity_gateway_service.go` | `local-rewrite-needed` + `defer` | 本地仍有 `ErrorPolicySkipped -> 500`分支，但三条协议、池模式、自定义错误码和账号处罚已分叉；需独立修复并覆盖 400/401/403/429/5xx、换号、原生/messages/chat 真实 E2E。 |
| B2 Ops 批量写失败 | v0.1.178 / `5f1943310` | 批量写失败后不再逐条重放 | `ops_service.go:RecordErrorBatch` | `applicable` | 批量结果可能处于未知状态，逐条重放会重复记录并放大 DB 压力；本轮改为直接返回错误并锁定“不调用单条写”。 |
| B2 透传模型发现 | v0.1.178 / `a288bab73` | 网关热路径中对齐 passthrough 模型发现 | 本地 `gateway_service.go`、OpenAI/Grok 模型目录和路由 | `local-rewrite-needed` + `defer` | 上游依赖其 platform/group 结构；需先对照本地生成 catalog、分组 allowlist 和实际 passthrough 路由不变式。 |
| B2 OpenAI Team 联动熔断 | v0.1.178 / `d677d67dd` | Team 组内账号联动 runtime block | 本地 scheduler/runtime block/failure-domain | `local-rewrite-needed` + `defer` | 联动处罚可扩大故障半径；必须先定义组绑定、失败域、TTL、康复与孤立账号回退。 |
| B2 Claude deferred tools | v0.1.178 / `9c36b75a7`、`0b35370a7` | 支持顶层 deferred tools 并移除其非法 `cache_control` | 本地 Anthropic tool/cache rewrite | `local-rewrite-needed` + `defer` | 本地没有同等 deferred-tool 产品合同；需用真实 Claude Code 请求验证 tool replay、cache block 上限和流式计费。 |
| B2 到期邮件无 SMTP 时跳过 | v0.1.178 / `79c2eb502` | 未配 SMTP 时不尝试发送订阅到期提醒 | 本地 `subscription_expiry_service.go` | `not-applicable` | 本地该服务只更新过期状态，未发送提醒邮件；不引入新的自动外发路径。 |
| B2 Docker Go builder | v0.1.178 / `11e1e2288` | builder 跟随上游 `go.mod` 到 1.26.6 | `backend/go.mod`、`backend/Dockerfile` | `already-covered` | 本地 `go.mod` 与 builder 均为 1.26.2，版本已一致；不单独抄上游工具链版本。 |
| B3 渠道监控配额模式 | v0.1.178 / `615e6901e`、`c44711ac9`、`6a6fd304f`、`41344c20f`、`7ab6d3db6`、`22df600d0` | migration 226、8 平台配额快照、负缓存/singleflight 和前端 | 本地 channel/account 监控与配额快照 | `defer` | 大功能+数据迁移，不与 bugfix 混合；需明确本地监控窗口、调用成本、负缓存过期和数据回滚。 |
| B3/B4 OpenAI 批量账号设置 | v0.1.178 / `76b70b168`、`8d82bb069` | 后端批量验证与管理端入口 | 本地账号编辑、RBAC 和审计 | `not-applicable` | 无已验证本地需求；不扩大批量修改权限面。 |
| B3/B4 Ollama 用量查询 | v0.1.178 / `9aac3b73f` | 管理端查询 Ollama 用量 | 本地供应商目录 | `not-applicable` | 未使用供应商，默认不吸收。 |
| B4 UI/运营显示集合 | v0.1.178 / `5e72deb7d`、`3bff4b64b`、`7d796f111`、`a6d868f27`、`35e8ba2a3`、`0d5e3ca9b`、`e8ff2017c`、`cb7841d85`、`22fc0cdbf`、`1977810cf`、`03c3f3b6f`、`5cbd0c96a` | Ops 时间区间/图例/SLA、暗色表单、仪表盘 cache token、i18n、账号助手与 Select | 本地前端和 Ops 页 | `not-applicable` | 保留本地前端；本轮三个后端修复不需要新 UI 合同。 |

其余 lint/test-only/merge/version 提交不产生独立可回移语义，已通过上述所属功能项追溯，不单独搬运。

## 3. 本轮实际回移

本轮只实现三个边界清晰且可以用本地回归测试锁定的候选：

1. Anthropic HTTP 200 + SSE `overloaded_error` 在未输出时按语义 529 处理，保留原始 body 和账号策略链；
2. Gemini typed transform 在函数工具与 Google Search 混用时发送 `includeServerSideToolInvocations=true`；
3. Ops 错误批量写失败后不再逐条重放，防止重复和写放大。

此外，首次 `go test -tags=unit ./...` 暴露已合并 PR #196 遗留的 unit-only 测试签名漂移：生产函数新增 `groupID *int64` 参数，`scheduler_shuffle_test.go` 的 7 处旧调用未同步。本分支仅在这些测试调用中显式传 `nil`，未改调度生产语义；修复后完整 unit-tag 套件通过。

没有回移数据库 migration、前端、主题、支付、订阅周期、月卡、返佣、生产配置、新供应商、Codex 指纹收敛或分时定价。

## 4. 测试门禁

实际执行证据：

- 三项 focused 回归：通过；
- `go test ./... -count=1`：通过；
- `go test -tags=unit ./... -count=1`：首次因 PR #196 遗留的 unit-only 测试签名漂移失败；仅修正 7 处测试参数后全量重跑通过；
- `go test -race ./internal/pkg/antigravity -run 'TestGeminiToolConfigIncludeServerSideToolInvocations' -count=1`：通过；
- `go test -race ./internal/service -run 'TestGatewayService_Forward_(PreOutputSSEOverloadedErrorUsesSemantic529|PostOutputSSEOverloadedErrorKeepsExistingStatus)|TestOpsServiceRecordErrorBatch_DoesNotFallbackToSingleInsertsWhenBatchFails' -count=1`：通过；
- `go vet ./...` 与 `go vet -tags=unit ./...`：通过；
- `/Users/fujunhao/go/bin/golangci-lint` v2.12.2，使用独立 cache 执行 `run ./...`：0 issues；
- `git diff --check`：通过。

前端未改，因此不运行前端测试或构建；这不代表前端、真实上游链路或生产已验证。

## 5. Action required

1. Gemini `ErrorPolicySkipped` 的 4xx/5xx 协议保真是最高优先后续；必须独立重写并验证 native/messages/chat、pool/custom-code、同号重试/换号和账号处罚。
2. Codex 出站身份收敛涉及 migration 和 HTTP/WS/探针一致性，不应与本轮 bugfix 混合。
3. 渠道分时定价及 WS turn 起始时刻触及资金与时区口径，必须先冻结北京/服务器时区、精度、31 天月卡、返佣和回滚合同。
4. 渠道配额监控是大功能+数据迁移，如有本地需求应拆独立 PR，不与本轮合并。
