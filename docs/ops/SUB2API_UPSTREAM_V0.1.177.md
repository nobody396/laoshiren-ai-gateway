# Sub2API v0.1.177 上游正式版审计与选择性回移

更新时间：2026-08-18（北京时间）

## 1. 版本与边界

| 项目 | 值 |
| --- | --- |
| 上次已审计正式版 | `v0.1.173`；依据 `origin/main:docs/ops/SUB2API_UPSTREAM_V0.1.173.md` |
| 本地审计基线 | `origin/main@1ddb6811037848760bee3b62af71c62960e86444` |
| 最新正式 Release | `v0.1.177`，2026-08-15 21:40:05（北京时间）发布，非 draft / 非 prerelease |
| Release URL | https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.177 |
| annotated tag object | `4ad52642a1548c46533f458a60d30c192026795e` |
| tag 目标 commit | `073e92d17178a1ccdb0a27017f572f10c9c7ab62` |
| 审计区间 | `v0.1.173^{commit}@29009f0b2ea14edf3b11ae2564fb617ff91a03b4..v0.1.177^{commit}` |
| 区间规模 | 124 commits（78 non-merge）、244 files、+15,459/-944 |
| 工作分支 | `codex/sub2api-v0.1.177-audit-20260816` |
| 临时工作树 | `/Users/fujunhao/laoshirenai/worktrees/sub2api-upstream-v0.1.177-20260816` |

本轮只认 Wei-Shaw/sub2api 的正式 GitHub Release。`v0.1.174` 没有对应正式 Release，未被单独当作发布基线；其代码变化仍通过 `v0.1.173..v0.1.175` 的 commit 区间纳入审计。没有追 `upstream/main`，没有 merge 上游分支或整树覆盖，没有修改生产配置、部署生产或使用客户凭据/余额。

### 正式 Release 追溯

| Release | 发布时间（北京时间） | annotated tag object | 目标 commit | 相对前一正式版 |
| --- | --- | --- | --- | --- |
| `v0.1.175` | 2026-08-12 19:07:29 | `b898c60c422d1de059968c56aca22f6643f1fed4` | `93c32fa1a2450351561abc46156d2e28cb5f74ca` | 84 commits / 114 files |
| `v0.1.176` | 2026-08-13 09:46:35 | `14e6d7ee7bdb1e4cb6bc59129a7ee1dd1110c52a` | `e803e3851c0a7e222cfadeafad7b8636ab959d11` | 23 commits / 101 files |
| `v0.1.177` | 2026-08-15 21:40:05 | `4ad52642a1548c46533f458a60d30c192026795e` | `073e92d17178a1ccdb0a27017f572f10c9c7ab62` | 17 commits / 76 files |

## 2. 逐项适用性矩阵

状态含义：`applicable` 本轮回移；`already-covered` 本地已有同等或更强实现；`local-rewrite-needed` 方向适用但必须按本地合同重写；`not-applicable` 当前产品/协议不适用；`defer` 证据或风险门槛不足，拆到后续。

| 风险域 | 上游 release / commit | 改动摘要 | 本地对应代码 | 状态 | 风险、建议与所需测试 |
| --- | --- | --- | --- | --- | --- |
| B0 Claude OAuth 指纹污染 | v0.1.175 / `fe2c265c91` | 拒绝本地构建/哨兵版本 UA 污染账号级持久指纹，并自愈存量坏值 | `backend/internal/service/identity_service.go` | `applicable` | 本轮本地重写；不记录原始异常 UA。测试首次创建、升级拒绝、存量自愈、合法升级、默认值自洽。 |
| B0/B1 API Key 数值合同 | v0.1.175 / `f5c108c836` | quota、5h/1d/7d 限额必须为有限非负数；有效期天数必须大于 0 | `backend/internal/handler/api_key_handler.go`、`backend/internal/service/api_key_service.go` | `applicable` | handler + service 双层校验，避免负额度或非有限值进入配额状态。测试 create/update、NaN/Inf/负数/零天数。 |
| B0 WebSocket / cyber 审计 | v0.1.175 / `6564d376e5`、`2d9920ba7d`、`c418fd522f` | 调整 cyber 事件范围，恢复并去重 WS 审计 | 本地 `backend/internal/service/openai_ws_forwarder.go`、`backend/internal/handler/ops_error_logger.go` 为独立观测链 | `local-rewrite-needed` + `defer` | 不直接搬上游审计事件模型；后续需以本地 WS/HTTP bridge 回合边界验证一次且仅一次、失败/断线和隐私字段。 |
| B1 OpenAI 嵌套 usage | v0.1.175 / `04dc540b23`、`a163742fc9` | 解析 `data.usage` / `data.response.usage`，并保持原生路径优先级 | `backend/internal/service/openai_gateway_service.go` | `applicable` | 本轮本地重写；防成功请求静默记 0，同时避免 wrapper 覆盖原生 usage。测试四种路径和优先级。 |
| B1 service-tier 计费 | v0.1.175 / `9261dd7734` | priority/flex 应进入账号成本 | `billing_service.go:serviceTierCostMultiplier`、OpenAI usage record | `already-covered` | 本地已把 service tier 传入成本计算并有倍率测试；保留本地定价解析链。 |
| B1 按上游响应模型计费 | v0.1.175 / `9096492b55`、`b689e5b401`、`e5b325e481`、`33351c7bc7` | 可按上游实际返回模型计费，并收紧准入 | 本地使用 requested/upstream mapping 的 `BillingModelSource`，没有该可选功能 | `not-applicable` | 不为未验证需求引入新的资金口径和管理端开关；若需要，必须独立设计可信模型来源、别名、流式/WS、一致性和回滚。 |
| B1 Grok 缺 usage / guard | v0.1.175 / `8ea68bd689`、`ba92d70422`、`5c52fa93d5` | 对聊天、兼容账号的缺失 usage 做一致防漏计处理 | 本地 Grok Responses/Chat bridge 与媒体计费为独立实现 | `local-rewrite-needed` + `defer` | 资金项不能照搬。后续覆盖 native/chat/messages、流中断、缺 usage fail-closed、月卡/余额/返佣和重复提交。 |
| B1 OpenAI 个人订阅到期 | v0.1.175 / `358e4a89a1` | 不让 workspace entitlement 覆盖个人订阅到期 | 本地账号 extra、quota 快照与调度器口径不同 | `local-rewrite-needed` + `defer` | 需以本地账号类型、快照来源和恢复策略重写；先补 personal/workspace/过期/刷新并发测试。 |
| B2 OpenAI HTML 403 | v0.1.175 / `12abb54700` | CDN/代理 HTML 403 不应升级为账号处罚 | `backend/internal/service/ratelimit_service.go` | `applicable` | 本轮回移；仍允许请求级 failover，但不递增 403 计数、不 cooldown/永久下线。测试 HTML 与结构化 JSON 403 分流。 |
| B2 Gemini tool schema | v0.1.175 / `c8d9af6ce1` | Gemini 不支持 `exclusiveMinimum`；整数边界可安全转成 `minimum=n+1` | `gemini_messages_compat_service.go:cleanToolSchema` | `applicable` | 本轮回移；仅转换可精确递增的整数，fractional/overflow/number 类型丢弃。测试嵌套、已有更严格 minimum 和模糊值。 |
| B2 Chat reasoning 别名 | v0.1.175 / `8aa425d22f` | 接受 `reasoning` 作为 `reasoning_content` 兼容别名 | `backend/internal/pkg/apicompat` | `applicable` | 本轮按本地转换链实现；`reasoning_content` 明确优先。测试 assistant 历史转换和冲突优先级。 |
| B2 确定性 400 | v0.1.175 / `591d47fb9b` | 原生 Responses 的确定性 400 不应被改成可重试 502 | `openai_gateway_service.go:shouldFailoverOpenAIPassthroughResponse` | `already-covered` | 本地 passthrough 只把 429/529 作为该层 failover，400 原样走客户端错误路径；保留现有测试。 |
| B2 compact silent EOF | v0.1.175 / `2f109e74ca` | keepalive 提交头后无有效 SSE 时发 `response.failed` | `backend/internal/handler/stream_error_event.go`、`openai_gateway_handler.go` | `already-covered` | 本地已有提交后合规终止事件和回归测试；不重复搬。 |
| B2 空 completed / 图片流失败 | v0.1.175 / `280c1c8623`、`9763765ebf` | 空 completed、图片流读取错误触发 failover | 本地首输出守卫、Codex 生图 bridge 与固定流合成不同 | `local-rewrite-needed` + `defer` | 需验证“无可见输出”与合法空响应、客户端断开后计费连续性、首输出前后 failover 和幂等；不在本轮混入。 |
| B2 capacity backoff / pool auth retry | v0.1.175 / `74fcdf3d42`、`7045f89de4` | 保留指数退避，pool auth 先同号重试再换号 | 本地调度、403 计数、OAuth refresh 和 failure-domain 逻辑不同 | `local-rewrite-needed` + `defer` | 后续以本地 runtime block、refresh CAS、粘连和故障域测试重写。 |
| B2 reasoning item ID | v0.1.175 / `9f31df3fa8` | API-key passthrough 剥离不被上游接受的 reasoning item id | 本地续链会保留 item reference/id，且 HTTP/WS 契约不同 | `local-rewrite-needed` + `defer` | 不能全局删除。需按上游类型验证 replay、previous_response_id、function call 和 WS v2。 |
| B2 TTFT / 调度快照 / 阈值缓存 | v0.1.175 / `900194fab2`、`e24cb99b79`、`ab326c96eb`、`3d3aee2e72`、`99b31067f7`、`3e1674a060` | 修正可见输出 TTFT、终止事件与陈旧阈值快照 | 本地 WS v2、监控和 scheduler snapshot 已高度分叉 | `local-rewrite-needed` + `defer` | 后续需真实 HTTP/WS 流、no-delta、陈旧/重置快照和缓存 miss 基准，不直接摘取。 |
| B3 Codex 指纹收敛 | v0.1.175 / `c0ab3a00ea`；v0.1.177 / `fce41e318f` | 四档会话标识收敛；v0.1.177 改为显式 opt-in 并覆盖 passthrough | 本地只做 API Key 隔离的 session/conversation id，没有该产品开关 | `not-applicable` | 不引入会静默改写客户端身份的功能；若未来需要，必须默认关闭并做真实 Codex/风控验证。 |
| B3 大文件备份分卷 | v0.1.175 / `bbc8b6e906` | 分卷上传与恢复 | 本地 backup archive/S3 实现不同 | `defer` | 非本轮核心；需资产大小、失败重试、校验、恢复原子性和旧格式兼容测试。 |
| B4 运营/安全菜单/用量列/Composite UI | v0.1.175 / `943f09d357`、`0d7b6ae64c`、`5350b3d98e`、`9b54b46b06` | 上游后台显示与权限开关调整 | 本地前端、RBAC、资源/用量页为独立产品 | `not-applicable` | 保留本地前端；本轮后端契约不要求 UI 改动。 |
| B0/B1 风控 fail-closed 回退 | v0.1.175 / `e01c917a97` 后被 `af6928a268` 回退 | 风控后端异常时是否阻断提示词 | 本地无同一 risk-control 产品合同 | `not-applicable` | 不吸收已在同一发布区间回退的策略。 |
| B1 分组逐模型/长上下文定价 | v0.1.176 / `f3d9491071`、`b830bc14d6` | Group → Channel → 内置定价链和长上下文开关 | 本地以 channel pricing / interval 为核心，资金与 31 天月卡合同不同 | `local-rewrite-needed` + `defer` | 大型资金/迁移项。需先冻结解析优先级、存量默认值、精度、月卡/返佣、迁移 checksum 与回滚。 |
| B1 Realtime 音频计费 | v0.1.176 / `678eb22a40` | 仅观察到音频后计费，修正求值顺序 | OpenAI Live/Realtime 不在默认吸收范围 | `not-applicable` | 无已验证本地需求，不混入。若启用需独立协议和实际音频 E2E。 |
| B1 未登记 Grok 文本模型 | v0.1.176 / `8c4c3c09ce`；v0.1.177 / `e29b93a1fb` | 未知文本回退价卡，但排除 image/video/audio 媒体族 | 本地文本目录与媒体 endpoint/计费函数分离 | `already-covered` | `billing_service.go` 对 Grok 4.6/4.5 有显式价卡，媒体走 image/video 专用计费；继续保留未知模型 fail-safe 测试。 |
| B2 Grok 4.6 | v0.1.176 / `a04ce49016` | 模型目录、官方定价、200k 长上下文倍率和请求路径 | 本地生成 catalog、billing fallback、Grok 路由 | `already-covered` | 本地已有 `grok-4.6` 价格、缓存价、200k 阈值和路由测试；不重复搬上游前端。 |
| B2 x_search | v0.1.176 / `0de6d7e9ba`、`c4d883b8da` | 独立端点并在 Chat↔Responses 保留字段/来源 | 本地 Grok native tools 已支持 `x_search`，但没有同一独立产品端点 | `already-covered` / `not-applicable` | 工具转发已覆盖；独立 `/x_search` 与上游按次计费产品不引入。 |
| B2 Responses probe | v0.1.176 / `fd9ce53281` | 截断/失败响应不得误记“不支持 Responses” | `openai_apikey_responses_probe.go` | `already-covered` | 本地仅 404/405 判不支持，2xx 截断不会写 false，避免同类错误；后续可加强“未完成不写 true”。 |
| B2 多实例备份 leader 锁 | v0.1.176 / `bba6a55e0f` | 定时备份加 leader 锁 | 本地只做进程内互斥 | `local-rewrite-needed` + `defer` | 多副本部署存在重复备份风险；需独立 PR 选择 Redis/DB 锁、fencing、TTL、崩溃恢复与对象存储幂等。 |
| B1/B2 缓存失效与定价冲突 | v0.1.176 / `814ecfba7c`、`bd404c16f2` | 分组平台变更失效 channel cache；统一价卡冲突 key | 本地 channel/group cache 和定价 schema 不同 | `local-rewrite-needed` + `defer` | 资金/路由热路径，需本地缓存 key、跨实例失效、冲突报错、回滚与并发测试。 |
| B2/B3 Grok JWT 档位和调度 | v0.1.176 / `bb9e74285e`、`363cc4994b`、`0ae151a238`、`69648476d` | JWT tier、Heavy 区分、模型级封禁和实时徽章 | 本地 quota、sticky、runtime block 与前端账号页不同 | `local-rewrite-needed` + `defer` | 先验证真实 claim、过期刷新、free/paid、模型隔离、缓存/并发和 UI 误导，再拆独立 PR。 |
| B0/B2 Codex turn-state | v0.1.177 / `8219dcfc87`、`4d9fedee20` | 回传 `x-codex-turn-state`，并阻止 failover 后跨账号回显 | 本地 WS 已缓存/重放握手 turn-state；HTTP 仅允许请求头透传，通用响应过滤和跨账号 provenance 不同 | `local-rewrite-needed` + `defer` | 本地是部分覆盖，不能把 WS 会话缓存等同于 HTTP failover 的铸造账号证明。必须覆盖首输出前后 failover、passthrough/transform、WS、多副本 TTL 与真实 Codex。 |
| B2 remote compaction v2 | v0.1.177 / `9662cff2e7`、`a8b9ea22b7`、`8ae6d8f67e` | 原生 v2 保留 `/responses`，发送 beta feature，探测不再走下线的 legacy endpoint | 本地仍有 `/responses/compact` bridge、账号能力缓存、模型映射和本地 250k 预压缩逻辑 | `local-rewrite-needed` + `defer` | 高优先协议项但范围跨 handler/scheduler/forward/probe。需真实 Codex v2 请求、compaction item、路由/模型限制、API-key 网关、legacy 回滚和生产灰度计划。 |
| B1/B3 分组用量日汇总 | v0.1.177 / `cb7b03795d`、`89d826be29`、`45dcce0e49` | 新增日汇总表、trigger/backfill、时区修复和仪表盘查询 | 本地无同一 rollup 表，账务/用量清理和北京时间口径不同 | `local-rewrite-needed` + `defer` | 大迁移且影响用量正确性。独立 PR 评审 UTC/北京时间边界、历史回填、双写/重算、清理顺序、校验与回滚。 |
| B1 Grok 长上下文开关 | v0.1.177 / `fd82dfd52d` | Grok 长上下文仅跟随分组开关 | 本地无上游同名 group flag，Grok 4.6 阶梯来自生成 catalog | `already-covered` | 没有 OpenAI 账号开关交叉否决路径；保留本地 200k 价阶测试。 |
| B4 账号页自动刷新偏好 | v0.1.177 / `e215c98c2c` | 修复前端偏好恢复时序 | 本地账号页实现不同 | `not-applicable` | 不搬上游前端。 |

## 3. 本轮实际回移

本轮只实现六个边界清晰、能以本地回归测试锁定的候选：

1. 拒绝 Claude OAuth 账号级指纹被畸形/哨兵 UA 污染，并自愈存量坏值；
2. API Key quota / rate limit / expires-in 输入的有限、非负和正有效期校验；
3. OpenAI 兼容响应 `data.usage` / `data.response.usage` 解析并保持原生路径优先；
4. OpenAI HTML 403 保持请求级，不写账号处罚；
5. Gemini tool schema 的整数 `exclusiveMinimum` 安全归一化；
6. Chat Completions assistant 历史接受 `reasoning` 别名，`reasoning_content` 优先。

没有回移数据库 migration、前端、主题、支付、订阅周期、月卡、返佣、生产配置、remote compaction v2 或 turn-state provenance。

## 4. 测试门禁

实际执行证据：

- 六项 focused default/unit 回归通过；
- `go test ./internal/service ./internal/handler ./internal/pkg/apicompat -count=1`：通过；
- `go test -tags=unit ./internal/service ./internal/handler ./internal/pkg/apicompat -count=1`：通过；
- `go test ./... -count=1`：通过；
- `go test -tags=unit ./... -count=1`：通过；
- `go test -race ./internal/pkg/apicompat -run 'TestChatCompletionsToResponsesAssistantReasoningAlias' -count=1`：通过；
- `go test -race -tags=unit ./internal/service -run 'TestExtractOpenAIUsageFromJSONBytes|TestCleanToolSchema|TestRateLimitServiceHandleUpstreamErrorOpenAIHTML403|TestIsAcceptableFingerprintUserAgent|TestGetOrCreateFingerprint|TestValidate(Create|Update)APIKeyRequest' -count=1`：通过；
- `go test -race ./internal/handler -run 'TestValidateAPIKey(Create|Update)Request' -count=1`：通过；
- `go vet ./...`、`golangci-lint v2.12.2 run ./...`、`git diff --check`：通过。

前端未改，因此没有运行前端测试或构建；这不代表前端、真实上游链路或生产已验证。

## 5. Action required

1. **Remote compaction v2** 是最高优先协议项：上游明确指出 legacy `/responses/compact` 已下线。本地仍有自定义 bridge/探测/能力缓存，必须独立本地重写并用真实 Codex v2 链路验证后再考虑合并。
2. **Codex turn-state provenance** 涉及换号后的跨账号回显；需要和 remote compaction v2 一起验证 HTTP、passthrough、WS、首输出 failover、多副本 TTL。
3. **Grok 缺 usage 计费**、**分组逐模型定价**、**分组日汇总迁移**均触及资金或用量口径，必须拆分独立高风险 PR，冻结本地 31 天月卡、余额、返佣、精度、迁移与回滚合同。
4. **多实例备份 leader 锁** 本地尚无分布式锁，应单独设计 fencing/TTL/崩溃恢复，不能用进程内 mutex 冒充多副本安全。
