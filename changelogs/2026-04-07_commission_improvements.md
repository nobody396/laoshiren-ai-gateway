# Changelog: 2026-04-07 分佣系统完善与用户邀请看板

## Bug 修复

### Backend

- `backend/internal/service/commission_service.go`
  - `isNotFound()` 改用 `errors.Is(err, ErrUserNotFound)`，修复邀请码 API 返回 404 的问题
  - 根因：`translatePersistenceError` 返回 `WithCause()` 克隆对象，指针不同导致 `==` 比较永远为 false

- `backend/internal/service/commission.go`
  - `AgentDashboard`、`InvitedUserStat`、`CommissionRecord` 三个结构体补全 JSON tag（snake_case）
  - 修复代理商总览页统计卡片数据为空的问题（Go 默认序列化 PascalCase，前端期望 snake_case）

### Frontend

- `frontend/src/i18n/locales/zh.ts` + `en.ts`
  - `auth.referralCodeLabel`：「推荐码」→「邀请码」，文案统一
  - `auth.referralCodePlaceholder`、`auth.referralCodeHint` 同步更新

- `frontend/src/views/agent/DashboardView.vue`
  - 已于 4-06 Phase 5 修复（日期默认近 30 天）

---

## 新功能：邀请机制完善

### 邀请码绑定幂等保护

- `backend/internal/repository/user_repo.go`
  - `SetInviterAndAgent` 改为条件 UPDATE：`WHERE inviter_id IS NULL`
  - 防止已绑定用户的邀请关系被重复覆盖

### 注册页 `?aff=` affiliate 链接支持

- `frontend/src/views/auth/RegisterView.vue`
  - `onMounted` 同时读取 `?aff=` 和 `?ref=` 参数，`?aff=` 优先级更高
  - 满足代理商分享 affiliate 链接的常见场景

### 邀请用户与分佣记录页日期默认值

- `frontend/src/views/agent/UsersView.vue`
- `frontend/src/views/agent/CommissionsView.vue`
  - `startDate`/`endDate` 初始化为近 30 天（与 DashboardView 保持一致）
  - 重置按钮恢复到近 30 天而非清空

---

## 新功能：用户邀请看板

### Backend

- `backend/internal/handler/dto/types.go`
  - `dto.User` 新增三个字段：`inviter_id`、`agent_id`、`first_recharged`
  - `GET /user/profile` 与 `GET /auth/me` 均可返回邀请绑定状态

- `backend/internal/handler/dto/mappers.go`
  - `UserFromServiceShallow` 补全三个新字段的映射

- `backend/internal/service/commission.go`
  - 新增 `UserReferralDashboard` 结构体（普通用户邀请统计）
  - `CommissionRepository` 接口新增 `SumByBeneficiaryTypeAndPeriod`（按类型统计佣金）

- `backend/internal/service/user_service.go`
  - `UserRepository` 接口新增 `CountInvitedByInviterID`

- `backend/internal/service/commission_service.go`
  - 新增 `GetUserReferralDashboard` 方法（邀请用户数 + 按类型统计佣金）

- `backend/internal/repository/commission_repo.go`
  - 实现 `SumByBeneficiaryTypeAndPeriod`（带类型过滤的佣金求和）

- `backend/internal/repository/user_repo.go`
  - 实现 `CountInvitedByInviterID`（按 inviter_id 统计被邀请用户数）

- `backend/internal/handler/user_handler.go`
  - 注入 `commissionService`，新增 `GetReferralDashboard` handler

- `backend/internal/server/routes/user.go`
  - 注册新路由：`GET /api/v1/user/referral/dashboard`

- `backend/cmd/server/wire_gen.go`
  - `NewUserHandler` 调用更新，传入 `commissionService`

### Frontend

- `frontend/src/types/index.ts`
  - `User` interface 新增 `inviter_id`、`agent_id`、`first_recharged`

- `frontend/src/api/user.ts`
  - 新增 `UserReferralDashboard` interface
  - 新增 `getUserReferralDashboard()` 方法（`GET /user/referral/dashboard`）

- `frontend/src/views/user/DashboardView.vue`
  - 统计卡片区下方新增"邀请与返利"看板区块：
    - 绑定状态卡片（显示是否绑定邀请码、邀请人类型、首充奖励状态）
    - 三个统计卡片（已邀请用户数、累计佣金、本月佣金）
    - 邀请码展示 + 一键复制注册链接

- `frontend/src/i18n/locales/zh.ts` + `en.ts`
  - 新增 `user.referral.*` 键（邀请看板全套文案）

---

## 测试记录

完整测试用例见 `plans_and_memory/2026_4_6_TEST.MD`

| ID | 测试项 | 结果 |
|----|--------|------|
| T-01 | 注册与邀请码绑定 | ✅ 通过 |
| T-02 | 代理商邀请码生成 | ✅ 通过 |
| T-03 | 代理商邀请用户注册（?ref=） | ✅ 通过 |
| T-04 | 普通用户邀请注册 | ✅ 通过 |
| T-05 | 首充双向奖励（代理商邀请） | ✅ 通过 |
| T-06 | 首充奖励（普通用户邀请） | ✅ 通过 |
| T-07 | 消耗分佣（DB 模拟） | ✅ 通过 |
| T-08 | 代理商看板数据验证 | ✅ 通过 |
| T-09 | 首充幂等性 | ✅ 通过 |
| T-10 | 代理商分佣记录明细 | ✅ 通过 |

## 已执行

- [x] 编译验证（`go build ./...`）✅
- [x] 前端 TypeScript 检查（0 errors）✅
- [x] 后端重启验证（PID 130107 → 139177）✅
