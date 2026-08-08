# Sub2API v0.1.172 上游吸收计划

更新时间：2026-08-08（北京时间）

## 结论

不能把 `Wei-Shaw/sub2api` 直接 merge 进本仓库，也不能用上游目录整体覆盖。

- 本仓库来自 `bozhouDev/DragonCode-sub2api` 的独立快照历史；它与
  `Wei-Shaw/sub2api` **没有共同 Git 祖先**。
- 当前私有仓库从初始快照到 `origin/main@ce5874e05` 有 367 个非 merge 提交；
  其中 218 个触及 backend，211 个触及 frontend。净变化包含 580 个 backend
  文件和 406 个 frontend 文件。因此本仓库不是“只改了前端”的轻量 fork。
- 已跟随的最后正式版本为 v0.1.136。上游最新正式版本为 v0.1.172
  （北京时间 2026-08-07 23:38 发布）；中间有 32 个正式 Release，
  v0.1.136..v0.1.172 含 1,075 个非 merge 提交、1,939 个变更文件。

正确方式是：以正式 Release 为边界，按风险域逐项回移（backport），保留本地
后端、数据模型、计费、月卡、分销、运维和前端产品层，并为每个上游修复记录来源
commit 和本地验证证据。

## 上游来源

| 名称 | 仓库 | 用途 | push |
| --- | --- | --- | --- |
| `origin` | `nobody396/laoshiren-ai-gateway` | 本项目私有仓库 | 允许走分支/PR |
| `upstream` | `bozhouDev/DragonCode-sub2api` | 私有初始分叉来源 | 禁止 |
| `wei-shaw` | `Wei-Shaw/sub2api` | 正式 Release 与上游补丁来源 | 禁止 |

本轮基线：

- 本地：`origin/main@ce5874e05f099c51978d20475f67ffc0bb5c8afe`
- 上游 Release：`wei-shaw/v0.1.172@155c494964c3ea6ecc31f52679525c1034bf0f16`
- 工作分支：`upgrade/sub2api-v0.1.172-p0`
- 工作树：`/Users/fujunhao/laoshirenai/worktrees/sub2api-v0.1.172-p0-20260808`

`wei-shaw/main` 在 v0.1.172 后仍有未发布提交。本轮默认不把未发布 main 当作
升级基线；只有独立确认的紧急安全修复才可例外进入单独补丁。

## P0 适用性核对

### GHSA-vrxq-qm4h-6hgg：适用，已在本分支回移

上游 v0.1.169 修复了客户端可控字符串直接拼接上游 URL path 的问题，来源 commit：
`017f6bbd5edffea0639ef3c84c0391161983f1f3`。

本地审计确认仍存在相同风险点：

- `/v1/responses/*subpath` 与 `/responses/*subpath` 直接进入转发；
- Responses 子路径直接追加到 OpenAI 上游 URL；
- Gemini 模型名直接拼接到 `/v1beta/models/{model}:{action}`；
- Gemini 模型查询 path 直接追加到 AI Studio base URL。

本分支已回移闭集 path segment 护栏、Responses 入口拒绝、上游 URL 构造兜底、
Gemini 统一 URL 构造及回归测试。上游 Grok video `request_id` 补丁未移植，因为
本地当前没有对应的 xAI video URL 构造实现。

### v0.1.172 OAuth 账号接管：上游精确漏洞路径不适用

上游修复来源 commit：`02e50cc22d038dabf3c6af92dbb92d1e0321f8d5`。

本地没有上游漏洞依赖的
`/auth/oauth/pending/create-account`、`/auth/oauth/pending/exchange`、
`choose_account_action_required` 或 `auth_oauth_pending_flow.go`。本地完成注册时使用
服务端 pending session 中的 provider identity，不接受攻击者补填任意目标邮箱，
因此不能复现该精确攻击链。

本地 OAuth 全链路复核结果：

- LinuxDo 不用上游返回邮箱匹配本地账号，而是按稳定 provider subject 生成
  `@linuxdo-connect.invalid` 合成邮箱；原始邮箱只保留在 provider claims 中。
- 已绑定 identity 的登录按 `(provider, provider_user_id)` 找用户；绑定入口要求已认证
  用户，pending session 固化目标 `user_id`，不会由回调参数改写。
- GitHub 只接受 primary verified email；Google/OIDC 可以由管理员关闭
  `require_email_verified`。此前该可选配置会让未验证的 provider email 参与本地邮箱
  匹配，存在撞中已有账号的接管风险。

本分支新增 provider email ownership 边界：外部 OAuth 将 `email_verified` 传入
AuthService；未验证 claim 可以遵循现有配置创建新账号，但不能仅凭同邮箱登录已有
账号，也不能在并发创建冲突后接管胜出的账号。已有兼容调用仍按“可信/已验证”处理，
LinuxDo 因使用稳定合成邮箱而保持原有行为。新增三项回归测试覆盖未验证撞号拒绝、
已验证邮箱兼容登录及并发唯一键冲突拒绝。

### 依赖安全审计：已清除可达 Go 漏洞与前端生产依赖告警

`govulncheck ./...` 首次确认 5 个代码可达漏洞：`GO-2026-5970`
（`x/text`）、`GO-2026-5960`（`excelize`）、`GO-2026-5764`
（AWS EventStream/S3）、`GO-2026-5061` 与 `GO-2026-4961`（`x/image`）。
本分支升级到官方修复版本并运行全量 `go test ./...`；复扫结果为
`Your code is affected by 0 vulnerabilities`。

`pnpm audit --prod` 首次报告 20 个 high（无 critical），涉及 axios、form-data、
linkify-it、postcss、nanoid 与旧 `xlsx`。前五类升级或用 pnpm workspace override
锁定到修复版本。旧 npm `xlsx@0.18.5` 没有可用的 patched npm 版本，因此没有
忽略告警，而是将管理员用量 XLSX 导出改为只负责生成文件的
`write-excel-file`，保持文件名、工作表和字段不变，同时移除 `xlsx` 与
`file-saver`。DOMPurify、Markdown-It、Mermaid、UUID、YAML 的中等级传递依赖也
一并锁定到修复版本；最终 `pnpm audit --prod --audit-level moderate` 返回
`No known vulnerabilities found`，frontend typecheck 与 578 项测试通过。

## 分批吸收顺序

### B0：安全热修

1. URL path guard（本分支已完成）。
2. OAuth 本地实现专项审计与绑定回归测试。
3. Go/前端依赖安全审计；依赖升级与业务回移分开提交。

### B1：资金与配额正确性

优先核对并回移：计费金额 NUMERIC(20,8) 量化、用量队列不丢记录、Anthropic
断流部分用量、WebSocket 多轮模型计费、优雅关停刷盘、订阅日额度午夜刷新。

这些补丁必须与本地月卡、余额、返佣、支付幂等和财务流水实现逐项对照；禁止直接
替换 service/repository/schema 或照搬上游 migration 编号。

### B2：网关稳定性

优先核对并回移：TCP/TLS 显式连接超时、输出前 failover、容量降载错误改写、
系统日志失败退避、低流量故障计数、count_tokens HTML 403 处理、
Responses→Anthropic 非法 content block 修复。

### B3：后端能力

按实际业务价值选择：上游响应模型审计、Grok/Antigravity 能力、组合分组、模型目录
与协议兼容。每个能力独立 PR，不与资金或安全热修混合。

### B4：管理端体验

只吸收后端能力所必需的管理端界面和明显可用性修复。保留本项目 landing、主题、
定价、月卡、资源页、分销和客户服务体验，不整体覆盖 upstream frontend。

## 每批退出条件

1. 写明上游 Release、commit/PR、适用性判断和本地改写点。
2. 新增能在修复前失败、修复后通过的回归测试。
3. 通过受影响 backend 包测试和 server build。
4. 涉及计费、月卡、支付、返佣或 migration 时，额外跑对应集成/迁移验证。
5. 涉及 frontend 时通过 typecheck、相关测试和 production build。
6. 独立 PR review 后才能合入 `main`；合并不等于上线。
7. 未收到明确“上线/部署到生产”指令前，不改生产环境。
