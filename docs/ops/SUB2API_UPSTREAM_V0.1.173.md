# Sub2API v0.1.173 上游正式版审计与选择性回移

更新时间：2026-08-10 09:01（北京时间）

## 1. 版本与边界

| 项目 | 值 |
| --- | --- |
| 上次已审计正式版 | `v0.1.172`；依据 `origin/main:docs/ops/SUB2API_UPSTREAM_V0.1.172.md` |
| 本地审计基线 | `origin/main@4406ac794b0d403c461e37e86686465a14624e9c` |
| 最新正式 Release | `v0.1.173`，2026-08-09 16:26:09（北京时间）发布，非 draft / 非 prerelease |
| Release URL | https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.173 |
| annotated tag object | `9e2a27ad39201a14074982bae331c4610161586a` |
| tag 目标 commit | `29009f0b2ea14edf3b11ae2564fb617ff91a03b4` |
| 审计区间 | `v0.1.172^{commit}@155c494964c3ea6ecc31f52679525c1034bf0f16..v0.1.173^{commit}` |
| 区间规模 | 120 commits（109 non-merge）、352 files、+33,307/-2,271 |
| 工作分支 | `chore/upstream-v0.1.173-audit-backport-20260810` |
| 临时工作树 | `/Users/fujunhao/laoshirenai/worktrees/sub2api-v0.1.173-audit-20260810` |

本地与 Wei-Shaw 上游仍不能安全直接 merge 或整树覆盖。本轮只按正式 Release/tag 审计；不追 `upstream/main`，不吸收草稿或预发布版本，不改生产配置，不部署生产。

## 2. 逐项适用性矩阵

状态含义：`applicable` 本轮回移；`already-covered` 本地已覆盖；`local-rewrite-needed` 适用但必须按本地架构重写；`not-applicable` 当前本地不适用；`defer` 独立后续项。

| 风险域 | 上游 release / commit | 改动摘要 | 本地对应代码 | 状态 | 风险、建议与所需测试 |
| --- | --- | --- | --- | --- | --- |
| B1 Gemini 生图计费 | v0.1.173 / `b6eb6c1ef` | 按上游实际回吐的 `inlineData` 图片数计费，避免自定义模型别名记 $0 | `backend/internal/service/gemini_messages_compat_service.go`、新增请求级图片观察器 | `applicable` | 本轮本地改写；取单 payload 最大值避免累积 SSE 重复计费，并保留模型名兜底。测试 camel/snake case、多图、空图、累积流、重置、原生非流接线。 |
| B1 OpenAI 非流式生图中断 | v0.1.173 / `cbf2be05a` | 客户端断开不取消已产生上游成本的生图请求 | `backend/internal/service/openai_images.go` | `applicable` | 本轮使用 `context.WithoutCancel` 语义继续上游 round trip，仍受 transport timeout 约束。测试取消后的上游 request context 与实际图片数。 |
| B2 Gemini 池模式 429 | v0.1.173 / `cbc2a3dd4` | 池模式同号重试期间不写账号级长限流 | `backend/internal/service/gemini_messages_compat_service.go` | `applicable` | 本轮本地改写；显式 custom error code 仍优先。测试 pool/non-pool/OAuth/custom policy。 |
| B0/B2 Grok OAuth nil client | v0.1.173 / `769f1c2de` | 缺失 OAuth client 时返回明确错误而非 panic | `backend/internal/service/grok_oauth_service.go` | `applicable` | 本轮在 code exchange、refresh、SSO conversion 前 fail closed。测试 refresh/SSO 缺失 client。 |
| B2 Grok 导入探测背压 | v0.1.173 / `962da308c` | 探测队列上限 64，pending/in-flight 去重 | `backend/internal/handler/admin/grok_import_probe.go` | `applicable` | 本轮回移；防批量导入制造无界内存/上游压力。测试并发上限、去重、队列边界、超时。 |
| B0 Grok OAuth 多副本会话 | v0.1.173 / `25d2b03e9`、`a3aae134e`、`6d632eec4` | Redis 共享、一次性消费、SSO/Cookie 隔离、隐藏密码登录 | 本地 `xai.SessionStore` 仍为进程内存，且本地没有同一 Redis session abstraction | `local-rewrite-needed` + `defer` | 不能只摘部分状态机。需独立安全 PR：多副本回调、重放、TTL、Redis 故障、敏感凭证不落日志/不持久化。 |
| B1 Grok 异步视频计费 | v0.1.173 / `b59a3df5b`、`21d0905c7`、`85b65284e`、`e01ce90d4` | 仅 `status=done` 且有 `video.url` 时按真实参数计费，避免 pending/失败误扣或重复扣 | 本地 `grok_media.go` 目前在 create/accept 阶段设置 `VideoCount=1` | `local-rewrite-needed` + `defer` | 高风险资金项，不在混合修复中照搬。需独立 PR 设计 request 归属、done 时一次性结算、重复 status 幂等、失败/过期不扣、duration/resolution 发现、并发轮询、月卡/返佣一致性和迁移/回滚。 |
| B1 Grok 搜索/Voice 计费 | v0.1.173 / `79df1647d`、`245d06960`、`d7c9e7167`、`3ae94df72`、`3a415e6d0` | 搜索按千次、流式去重、Voice 定价和 request-id 接线 | 本地 Grok Responses/媒体为独立实现，暂无同一 `SearchCount` 状态机和价格 schema | `local-rewrite-needed` + `defer` | 独立资金 PR；测试同一 search 事件多帧不重复、主/桥接路径一致、流中断、缺配置 fail-closed、月卡/返佣、精度与幂等。 |
| B2 OpenAI OAuth routing hints / legacy beta | v0.1.173 / `915cc7e7b`、`815035fcc`、`de349187d` | 停止注入废弃 beta 头并发送安全 routing hint | 本地 `openai_gateway_service.go` 仍有多处 `responses=experimental` 注入；另有本地 Codex 身份、compact、G7/G8/G9 分支边界 | `local-rewrite-needed` + `defer` | 协议适用但不能无观察直接切换。独立 PR 覆盖 OAuth HTTP/passthrough/WS/compact/messages bridge、API-key beta 保留、header 注入防护和真实 Codex 客户端矩阵。 |
| B2 Grok free gate / 调度 | v0.1.173 / `ba58e74b3`、`9bc99e8b6`、`2413441b5`、`7eb131070` | 24h free 软门禁、team+model 冷却、7d/30d 阈值、stream idle 换号 | 本地已有独立 Grok quota、sticky、冷却与账号选择实现，但口径不同 | `local-rewrite-needed` + `defer` | 属调度器大项。测试 free/paid 判定、模型级隔离、fail-open/closed、粘连、额度恢复、并发和真实网关 usage；不可改变本地月卡 31 天口径。 |
| B3 渠道监控 V2 | v0.1.173 / `a58048ac3`…`04d9eeaf0` | 被动流量聚合、13 个 migration、用户/管理端新 UI | 本地已有 ops/probe/Shadow 观测，不存在同一 V2 schema | `defer` | 约 5k+ 行跨 DB、服务、API、前端的大型系统改造；默认不吸收。若有产品需求，独立设计隐私、保留期、回填负载、迁移回滚和生产数据量测试。 |
| B3/B4 Grok Voice/Realtime/custom voices/web_search 与管理端 | v0.1.173 / `99ad01f6d`、`35faaa6d2`、`d0767eab9` 等 | 新协议、定价、管理端真实媒体预览 | 本地未验证同等产品需求；Realtime/Live 明确不在默认吸收范围 | `not-applicable` / `defer` | 不搬整套前端、Voice/Realtime/custom voices/web_search。只有明确需求与独立协议/计费/E2E 计划时再开单独 PR。 |
| B4 Grok 跨客户端模型映射默认关闭 | v0.1.173 / `74249b8fe`、`e9aaa325f`、`cda6de44d` | 不再把 `gpt-*`/`claude-*` 隐式改为 Grok | 本地主要走账号显式 `model_mapping`，没有同一全局隐式映射开关 | `already-covered` | 保持本地显式映射；不引入上游设置页。回归需锁定未配置映射时不跨厂商改写。 |
| B0 依赖漏洞 nanoid | v0.1.173 / `8ad0a5ff5`、`a4faa015a` | 升级到 patched 版本 | 本地 lockfile 已强制 `nanoid@<3.3.17: 3.3.18` | `already-covered` | 无代码变更；最终仍跑 production audit，不能只依赖版本字符串。 |
| B2 response model 热路径 | v0.1.173 / `6e34fb09c` | 优化上游响应模型观察器 | 本地没有上游同名 `upstream_response_model` 子系统 | `not-applicable` | 不创建半套 observer；本地如后续需要 response self-reported model，按 v0.1.172 审计结论独立实现。 |
| B0 注册邮箱域名限量 | v0.1.173 / `4999231d6`、`563a72ca7` | 非白名单主域名限注册一个，开关默认关闭 | 本地已有 allowlist/suffix policy，但无该产品需求 | `not-applicable` | 默认关闭的新产品/风控策略不自动引入；若启用需评审 eTLD+1、别名、并发唯一性、误伤与申诉流程。 |
| B4 上游整套前端/主题 | v0.1.173 / 多提交 | 新监控、Grok 管理、设置和用量 UI | 本地品牌、月卡、资源页、运营后台为独立产品 | `not-applicable` | 保留本地前端；本轮后端 bug 不要求前端契约变更。 |

## 3. 本轮实际回移

本轮只实现五个边界清晰、能以本地回归测试锁定的候选：

1. Gemini 实际图片输出计费；
2. OpenAI 生图客户端断开后的上游结算连续性；
3. Gemini pool-mode 429 不误封整账号；
4. Grok OAuth 缺失 client 明确失败；
5. Grok 导入探测队列背压与去重。

没有回移数据库 migration、前端、主题、支付、订阅周期、返佣口径或生产配置。

## 4. 测试门禁

实际执行证据：

- focused unit：上述五项新增/回归用例通过；
- `go test ./internal/service ./internal/handler/admin -count=1`：通过；
- `go test -tags=unit ./internal/service ./internal/handler/admin -count=1`：通过；
- `go test ./... -count=1`：通过；
- `go test -tags=unit ./... -count=1`：通过；
- `go test -race -tags=unit ./internal/handler/admin -run 'TestGrokImportProbeScheduler'`：通过；
- 新增计费/认证用例的 `go test -race -tags=unit ./internal/service`：通过；
- `go vet ./...`、`git diff --check`：通过；
- `pnpm audit --prod --audit-level moderate`：`No known vulnerabilities found`。

中间曾有一条旧 unit 用例失败：它预期缺失 Grok OAuth service 时发生 panic 并由 worker recovery 捕获；代码改为 fail-closed 明确返回 `GROK_OAUTH_CLIENT_NOT_CONFIGURED` 后，同步更新该契约用例并重跑全量 default/unit，最终均通过。

前端未改，因此没有运行前端测试或构建；这不代表前端或生产已验证。

只有所有实现与证据完整时才推分支并创建唯一 Draft PR：`chore(upstream): audit/backport Sub2API v0.1.173`。PR 仅等待人工审核；不得自动合并或部署。

## 5. Action required

Grok 异步视频计费是本轮最高优先级未实现项：本地 create/accept 阶段即设置视频计费单位，而 v0.1.173 的正式修复改为 done + video URL 后按真实参数、幂等结算。该改动触及资金、月卡、返佣、请求归属和并发轮询，必须拆独立高风险 PR，并先冻结精确结算契约与迁移/回滚方案。
