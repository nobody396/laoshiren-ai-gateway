# Changelog: 2026-04-06 Phase 4 分佣系统前端实现

## 新增文件

### API 模块
- `frontend/src/api/agent.ts` — 代理商 API 封装（邀请码、Dashboard、用户列表、分佣记录、验证推荐码）

### 代理商视图
- `frontend/src/views/agent/DashboardView.vue` — 代理商总览（分佣统计卡片、邀请码展示、快捷链接）
- `frontend/src/views/agent/UsersView.vue` — 邀请用户列表（日期筛选、消费/分佣统计、分页）
- `frontend/src/views/agent/CommissionsView.vue` — 分佣记录明细（类型/日期筛选、分页）

## 修改文件

### 注册页
- `frontend/src/views/auth/RegisterView.vue`
  - 新增 `referral_code` 输入框（始终展示，可选）
  - 支持 URL `?ref=XXXX` 自动填入推荐码
  - 注册请求和邮箱验证跳转时均传递 `referral_code`

### 个人资料页
- `frontend/src/views/user/ProfileView.vue`
  - 新增"我的邀请码"卡片（调用 `/user/invite-code` 自动生成）
  - 支持一键复制邀请码

### 管理员用户管理
- `frontend/src/views/admin/UsersView.vue`
  - 操作菜单新增"设为代理商/撤销代理商"选项
  - 角色筛选下拉新增 `agent` 选项
- `frontend/src/api/admin/users.ts` — `list` 过滤器 `role` 类型加入 `'agent'`
- `frontend/src/types/index.ts` — `UpdateUserRequest.role` 类型加入 `'agent'`

### 路由
- `frontend/src/router/index.ts`
  - 新增 `/agent`、`/agent/dashboard`、`/agent/users`、`/agent/commissions` 路由
  - 新增 `requiresAgent` meta 守卫（agent 或 admin 可访问）

### 侧边栏导航
- `frontend/src/components/layout/AppSidebar.vue`
  - 新增 `isAgent` 计算属性
  - 新增 `AgentIcon` SVG 组件
  - 新增 `agentNavItems` computed（Dashboard/Users/Commissions）
  - 普通用户视图中：当 `role === 'agent'` 时渲染"代理商中心"菜单区块

### 国际化
- `frontend/src/i18n/locales/en.ts` 新增键：
  - `agent.*`（全套代理商 UI 文本）
  - `nav.agentConsole`、`nav.agentDashboard`、`nav.agentUsers`、`nav.agentCommissions`
  - `auth.referralCodeLabel`、`auth.referralCodePlaceholder`、`auth.referralCodeHint`
  - `profile.myInviteCode`、`profile.inviteCodeHint`
  - `admin.users.agent`、`admin.users.setAsAgent`、`admin.users.removeAgent`、`admin.users.setAsAgentSuccess`、`admin.users.removeAgentSuccess`
  - `common.copy`、`common.prev`、`common.showing`、`common.user`（补充缺失键）
- `frontend/src/i18n/locales/zh.ts` — 同上中文翻译

## 后端补充修改

- `backend/internal/handler/agent_handler.go` — 新增 `ValidateReferralCode` 方法（公开接口，用于注册页验证推荐码）
- `backend/internal/server/routes/auth.go` — 注册 `GET /api/v1/validate-referral-code` 公开路由

## 已执行

- [x] `go generate ./ent/...` — 生成 Ent 代码（CommissionRecord 实体 + User 邀请字段）
- [x] `wire gen ./cmd/server/...` — 重新生成 Wire 依赖注入（补全 Sora 相关缺失 provider）
- [x] 编译验证（`go build ./...`）✅
- [x] 前端构建验证（`pnpm build`）✅ — 修复 TypeScript 错误（`User.role` 加入 `'agent'`、`RegisterRequest` 加入 `referral_code`）

## Phase 5 补丁（本地运行验证中发现）

### Bug 修复

- `frontend/src/i18n/locales/zh.ts` — 修正 `agent.inviteCodeHint` 乱码文字
- `frontend/src/i18n/locales/en.ts` + `zh.ts` — `admin.users.roles` 新增 `agent` 键（修复角色显示为 i18n key 的问题）
- `frontend/src/views/agent/DashboardView.vue` — 日期范围默认为最近 30 天
- `backend/internal/repository/user_repo.go` — 新增 `GetInviteCodeByUserID` 原生 SQL 方法（绕过 Ent `GetByID` 在特定 context 下返回 404 的问题）
- `backend/internal/service/user_service.go` — `UserRepository` 接口新增 `GetInviteCodeByUserID`
- `backend/internal/service/commission_service.go` — `GetOrCreateInviteCode` 改用 `GetInviteCodeByUserID`
- [ ] 前端构建验证（`pnpm build`）
