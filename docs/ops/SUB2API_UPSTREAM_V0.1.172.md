# Sub2API v0.1.172 上游吸收与发布审计

更新时间：2026-08-08（北京时间）

## 1. 结论与边界

本轮不能执行普通 `git merge`，也不能用上游目录覆盖本地仓库。

- `origin/main` 与 `wei-shaw/v0.1.172` 的 `git merge-base` 不存在；两边是无共同 Git 祖先的独立历史。
- 本地不是“只改了前端”的轻量 fork，已经包含独立的后端、数据模型、月卡、计费、返佣、支付、运维和前端产品实现。
- `wei-shaw/v0.1.136..v0.1.172` 有 1,075 个非 merge 提交、1,939 个变更文件；整树覆盖会直接破坏本地资金和产品逻辑。

因此本轮采用 **Release 边界 + 风险域 backport**：逐项确认上游问题在本地是否存在，存在则按本地架构改写，并为每项保留来源 commit、回归测试和发布门禁。它不是声称“1,075 个提交逐字合并”，而是先完成 B0-B2 的安全、资金正确性和稳定性修复；B3 大型能力只做适用性审计并拆分后续独立 PR；B4 保留本地前端，仅补后端契约必需项。

## 2. 已锁定基线

| 项目 | 值 |
| --- | --- |
| 本地基线 | `origin/main@ce5874e05f099c51978d20475f67ffc0bb5c8afe` |
| 上游 Release commit | `wei-shaw/v0.1.172^{commit}@155c494964c3ea6ecc31f52679525c1034bf0f16` |
| 上游 annotated tag object | `61ba94d2e85a00ba639fc870b91946b1bd2f990d`（不是代码 commit） |
| 上游上次本地正式基线 | `wei-shaw/v0.1.136@a2f76e411d3b97811f262bdfd569bc9901706068` |
| 工作分支 | `upgrade/sub2api-v0.1.172-p0` |
| 工作树 | `/Users/fujunhao/laoshirenai/worktrees/sub2api-v0.1.172-p0-20260808` |
| PR | `#86` |

`wei-shaw/main` 在 v0.1.172 后的未发布提交不在本轮默认范围内。

## 3. B0：安全热修（已完成）

### 3.1 上游 URL path 注入护栏

- 上游来源：`017f6bbd5edffea0639ef3c84c0391161983f1f3`
- 本地提交：`baeece24a`
- 本地确认风险点：Responses 子路径、OpenAI 上游 URL、Gemini 模型/action path、AI Studio 查询 path。
- 本地改写：统一闭集 path segment 验证，在 handler 和最终 URL 构造两层拒绝危险 path。
- 不适用项：本地没有上游 Grok video `request_id` URL 构造路径。

### 3.2 OAuth 外部邮箱所有权边界

- 上游来源：`02e50cc22d038dabf3c6af92dbb92d1e0321f8d5`
- 本地提交：`9aa45235d`
- 上游 pending exchange 的精确攻击链在本地不存在，但本地允许关闭 OIDC `require_email_verified`，未验证邮箱此前可能参与已有账号匹配。
- 本地改写：把 `email_verified` 传入 AuthService；未验证 claim 可按原配置创建新用户，但不能仅凭同邮箱登录已有用户，也不能在唯一键并发冲突后接管胜出账号。
- 覆盖：未验证撞号拒绝、已验证邮箱兼容登录、并发唯一键冲突拒绝。

### 3.3 依赖安全

- 本地提交：`9aa45235d`
- `govulncheck ./...`：从 5 个可达 Go 漏洞降至 0。
- `pnpm audit --prod --audit-level moderate`：从 20 个 high 降至 0 已知漏洞。
- 旧 `xlsx@0.18.5` 无 patched npm 版本，管理员 XLSX 导出已迁移到 `write-excel-file`，没有用 ignore 掩盖告警。
- 前端 typecheck 与 579 项测试已通过。

## 4. B1：资金、配额和 usage 正确性（已完成）

| 能力/缺陷 | 上游来源 | 本地提交 | 本地处理 |
| --- | --- | --- | --- |
| NUMERIC(20,8) 金额量化 | `e2652eb85` | `42fec5862` | 所有 usage 金额在持久化边界统一量化，避免 PG 数值溢出/精度漂移 |
| usage 队列满时静默丢账 | `a1b2b32e0` | `dca160f72` | 队列压力下 fail-closed 同步落库，不把成功请求变成未记账请求 |
| 日额度午夜刷新 | `99b357083` | `90e8a9c68` | 保留本地订阅周期，同时恢复每日额度按本地日界线重置 |
| Anthropic 流中断部分 usage | `bd52e5d77` | `e78a50030` | 已观察到 usage 时即使 terminal/read error 也进入计费记录；无 usage 时不猜测 |
| 返佣按“计划扣款”多结算 | 本地审计发现 | `f39f7abde` | 只按实际成功扣到的按量余额结算，低余额时 clamp |
| 同账号重试误触发 cache_read 计费 | `a2acbf553` | `35b56518c` | 只有真实换号才强制缓存计费；同账号原地重试不改计费口径 |
| 同步流 failover 后缓存计费不一致 | `addd5ef1d` | `35b56518c` | passthrough/normal 两条路径统一传播 `ForceCacheBilling` |
| Anthropic 同步返回 cache 分类 | `addd5ef1d` | `35b56518c` | 响应 usage 与内部 usage 对象同时改写，防止重复分类 |
| hosted image tool token 漏计 | `865128998` | `35b56518c` | 合并 `tool_usage.image_gen.output_tokens_details.image_tokens`；主 usage 优先 |
| payment 文本 NUL 导致 PG 写入失败 | `e76e0499d` | `35b56518c` | 按本地 Alipay 持久化字段清理 PostgreSQL 非法 NUL |
| WebSocket 多轮模型串账 | `7ce6e8d65` | `47be813b5` | 每 turn 独立做渠道映射、账号映射、上游模型恢复和 usage 归属；ingress/passthrough 均覆盖 |

补充测试修正：`b8fdf3f3c` 把旧集成断言校准为“同账号重试不切换缓存计费，真正换号后才切换”。

### 4.1 月卡额度耗尽客户报错（新增，已完成）

事故样本：请求 ID `7bb1546b-5fac-411c-b5ea-35daffefc545`，网关内部原因为 `MONTHLY_LIMIT_EXCEEDED`。旧行为返回 HTTP 429，Codex 会重试并最终只显示 `exceeded retry limit, last status: 429`，客户容易误判为平台或上游限流。

本地提交：`1686237a8`

新契约：

- 月卡额度耗尽返回 HTTP **403**，reason 保持 `MONTHLY_LIMIT_EXCEEDED`。
- 文案明确：这是客户账号月卡额度耗尽，不是服务故障；重试无效；需要购买并激活新月卡，或切换到有余额的按量 API Key。
- 日/周频控仍保持 HTTP 429，不混淆真正的可重试限流。
- Google 兼容错误映射为 403 `PERMISSION_DENIED`。
- ops 分类为 `error_owner=client`、`is_business_limited=true`、不可重试，不计入平台 SLA 故障率。
- 标准 middleware 不再用 `validateErr.Error()` 覆盖精确业务 reason/message。

## 5. B2：网关稳定性（已完成）

| 能力/缺陷 | 上游来源 | 本地提交 | 本地处理 |
| --- | --- | --- | --- |
| DNS/TCP 建连可能卡到内核超时 | `66ad405dd` | `cba522417` | HTTP/HTTPS 上游显式 10s DialContext、10s TLS handshake、30s keepalive；SOCKS forward dialer 同样有界 |
| 系统日志 DB 故障重试放大 | `e687ca3e9` | `6982b6900` | 2s 指数退避至 60s；退避期 best-effort 丢弃观测日志，不占满业务连接池；成功后清零 |
| SSE `error → response.failed` 阻断 pre-output failover | `c33c3208e` | `25276cffd` | 可重试 error 帧不再算首输出；后续 failed 可安全切号/同号重试 |
| 中途容量降载被 Codex 当致命错误 | `c33c3208e` | `25276cffd` | 已有输出后把 `server_is_overloaded`/`slow_down` 改写为客户端可重试的 `server_error`，保留原消息和内部分类 |

### B2 已审计但未直接移植

- `7d38e6712` 低流量 transient streak：本地没有该上游“account+model breaker”子系统，单独摘取会制造半套状态机；归入后续调度器专项，不在本轮资金/稳定性 PR 中引入。
- `64090de66` Responses→Anthropic request content block：本地没有上游 `ResponsesToAnthropicRequest` 请求转换路径，精确问题不可复现。
- `c4128580f` OAuth `/input_tokens` fallback：本地没有上游 `ForwardCountTokensAsAnthropic` bridge；当前 count_tokens 走不同实现，精确补丁不适用。

## 6. B3：大型后端能力审计（本轮不做整套搬运）

这些能力与本地平台、数据模型和前端强耦合，必须独立 PR，不能混进本轮生产热修：

1. **上游 response model 审计**（`db0bff82c`）：上游一次改动 76 个文件，新增字段、索引、聚合、各协议 observer 和前端筛选。本地已经记录客户端请求模型、渠道映射链和实际出站 `upstream_model`，但尚未单独持久化“供应商响应体自报模型”。这是可观测性增强，不是当前账单错误；后续可按本地 migration 编号独立落地。
2. **Grok / Antigravity 新能力**：此处旧稿把“兼容模型调用”误判为“本地已有原生 Grok 子系统”。该结论已纠正：`origin/main` 当时并没有 `PlatformGrok`、Grok OAuth/配额/媒体/原生调度。当前准确缺口与决策见 `SUB2API_UPSTREAM_FEATURE_GAPS_V0.1.172.md`；仍然不能用上游 service/schema 整树覆盖，只能按本地架构逐能力回移。
3. **Composite group / profit control / model plaza**：涉及调度、价格、分组、管理 API 和前端产品语义；本地月卡与分组模型不同，必须单独做产品与财务评审。
4. **验证码、Passkey、Live、通用支付提供商**：属于新产品能力而非本轮重大后端 bug 修复，不与 02:00 维护窗口的热修包一起发布。

## 7. 明确不适用或会回归本地产品的上游改动

- `7b6111f2f` 上游订阅窗口按 30 天重写：本地月卡使用精确激活时间、每日午夜、31 天商品周期并把 reset 截断到 expiry；照搬会改变已售商品权益。
- `3deb2f17d` 通用支付 settings PATCH：本地是定制 Alipay 配置，没有同一 visible methods 数据结构。
- `c6f375d3a` 通用订阅支付汇率：本地商品价格直接存 CNY 分，不应再次换算。
- `147c1879d` 复数 validity units：本地计划只存整数 `validity_days`，无同一单位枚举。

## 8. B4：管理端与客户体验策略（已完成审计）

- 不覆盖本地 landing、主题、定价、月卡、资源页、分销和客服体验。
- 本轮没有后端契约要求新增管理端表单；因此不引入上游大面积 frontend diff。
- 客户最直接的体验修复已经在 API 错误契约完成：月卡耗尽由可重试 429 改为明确不可重试 403，并提供下一步操作。
- WebSocket 多轮模型修复同时修正每 turn 的响应模型恢复与账单归属，不要求客户升级客户端。

## 9. 当前分支提交清单

```text
baeece24a fix(gateway): guard upstream URL path segments
9aa45235d fix(security): close OAuth and dependency audit gaps
42fec5862 fix(billing): quantize persisted usage amounts
dca160f72 fix(usage): prevent accounting log drops under load
90e8a9c68 fix(subscription): restore midnight daily quota reset
e78a50030 fix(gateway): bill observed partial Anthropic streams
f39f7abde fix(affiliate): settle only collected payg balance
1686237a8 fix(subscription): make monthly exhaustion actionable
35b56518c fix(billing): absorb upstream cache and image corrections
cba522417 fix(upstream): set explicit TCP dial timeout on upstream transports
6982b6900 fix(ops): 系统日志落库失败后退避重试，避免拖垮数据库连接池
25276cffd fix(gateway): recover streamed capacity shedding
b8fdf3f3c test(failover): assert cache billing only on account switch
47be813b5 fix(openai): track websocket models per turn
```

## 10. 发布门禁

合并不等于上线。当前工作树/分支不会影响生产客户；只有生产 Swarm 服务更新到新镜像时才会生效。

发布前必须：

1. 工作树干净，文档中的上游 tag/hash 与 Git 实际一致。
2. 后端 `go test ./...`、`go test -tags=unit ./...`、关键 Testcontainers 集成测试通过。
3. `govulncheck ./...` 为 0 reachable vulnerability。
4. 前端 typecheck、全量测试、production build、production audit 通过。
5. migration/Ent schema diff 审计无意外 destructive change。
6. 推送分支并通过 PR CI；合并后的发布镜像必须对应精确 `origin/main` commit/digest。
7. 只在约定的北京时间维护窗口执行 start-first Swarm 更新；健康失败立即回滚到旧 digest。

## 11. 2026-08-08 最终发布前验证证据

以下命令均在独立工作树执行，未更新生产 Swarm 服务，因此不会影响在线客户：

- 后端默认测试：`go test ./... -count=1`，通过。
- 后端 unit tag 全量测试：`go test -tags=unit ./... -count=1`，通过。
- 并发竞态检查：WebSocket 多轮模型和 ops log backoff 相关用例在 `go test -race -tags=unit` 下通过。
- Go 漏洞扫描：Go 1.26.5、`govulncheck v1.6.0`、漏洞库更新时间 2026-07-27；0 个可达漏洞，0 个 imported package 漏洞。
- 前端：108 个测试文件、579 项测试通过；`vue-tsc -b` 与 production build 通过；production audit 为 `No known vulnerabilities found`。
- 资金链路：Affiliate Testcontainers 集成测试覆盖“按实际收款结算”和“低余额 clamp”，通过；测试容器已清理。
- 结构审计：相对 `origin/main` 没有新增 migration 或 Ent schema diff；`git diff --check` 通过。
- 基线复核：`origin/main@ce5874e05f099c51978d20475f67ffc0bb5c8afe`、`wei-shaw/v0.1.172^{commit}@155c494964c3ea6ecc31f52679525c1034bf0f16`，两者仍无共同 merge-base；annotated tag object 为 `61ba94d2e85a00ba639fc870b91946b1bd2f990d`。

仍待完成的门禁仅为远端阶段：推送分支、PR CI、合并 main、锁定 CI 发布的不可变镜像 digest，以及在约定维护窗口执行生产更新和健康/回滚检查。
