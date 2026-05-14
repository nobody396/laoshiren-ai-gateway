# Changelog: 2026-04-06 Phase 1-3 分佣系统后端实现

## 新增文件

### 数据层
- `backend/ent/schema/commission_record.go` — CommissionRecord Ent Schema（commission_records 表）
- `backend/migrations/078_add_commission_system.sql` — 数据库迁移：users 表新增4字段 + commission_records 新表
- `backend/internal/service/commission.go` — CommissionRecord model + CommissionRepository 接口 + AgentDashboard/InvitedUserStat DTO
- `backend/internal/repository/commission_repo.go` — CommissionRepository 实现（Create、ListByBeneficiary、SumByBeneficiaryAndPeriod、ListInvitedUsersWithStats）

### 服务层
- `backend/internal/service/commission_service.go` — CommissionService 核心业务（分佣处理、邀请码生成、统计查询）

### 接口层
- `backend/internal/handler/agent_handler.go` — AgentHandler（邀请码、Dashboard、用户列表、分佣记录）
- `backend/internal/server/middleware/agent_only.go` — AgentOrAdmin 角色中间件

## 修改文件

### 数据层
- `backend/ent/schema/user.go` — 新增 invite_code、inviter_id、agent_id、first_recharged 字段
- `backend/internal/service/user.go` — User model 新增 InviteCode、InviterID、AgentID、FirstRecharged
- `backend/internal/service/user_service.go` — UserRepository 接口新增4个邀请系统方法
- `backend/internal/repository/user_repo.go` — 实现 GetByInviteCode、SetInviteCode、SetInviterAndAgent、SetFirstRecharged
- `backend/internal/repository/api_key_repo.go` — userEntityToService 函数新增邀请字段映射
- `backend/internal/repository/wire.go` — 注册 NewCommissionRepository

### 服务层
- `backend/internal/domain/constants.go` — 新增 RoleAgent + 4个 CommissionType 常量
- `backend/internal/service/domain_constants.go` — 暴露 RoleAgent + CommissionType 常量
- `backend/internal/service/auth_service.go` — AuthService 新增 commissionService 字段；RegisterWithVerification 新增 referralCode 参数；注册时绑定邀请关系
- `backend/internal/service/redeem_service.go` — RedeemService 新增 commissionService 字段；balance 兑换后异步触发首充奖励
- `backend/internal/service/gateway_service.go` — GatewayService/billingDeps 新增 commissionService；postUsageBillingParams 新增 UsageLogID；DeductBalance 后异步触发消耗分佣
- `backend/internal/service/wire.go` — 注册 NewCommissionService

### 接口层
- `backend/internal/handler/auth_handler.go` — RegisterRequest 新增 ReferralCode 字段；Register 调用传入 referralCode
- `backend/internal/handler/handler.go` — Handlers 新增 Agent 字段
- `backend/internal/handler/wire.go` — 注册 NewAgentHandler；ProvideHandlers 新增 agentHandler 参数
- `backend/internal/server/routes/user.go` — 新增 /user/invite-code 和 /agent/* 路由组

## 待完成

- [ ] Phase 4 前端实现（Register 页面邀请码输入框、代理商控制台页面）
- [ ] `go generate ./ent/...` — 生成 Ent 代码（含 CommissionRecord 实体和 InviteCode/InviterID/AgentID 字段）
- [ ] `wire gen ./cmd/server/...` — 重新生成 Wire 依赖注入代码（因为多个 service/handler 构造函数签名变更）
- [ ] 编译验证与测试

## 注意事项（开发者）

1. **必须先运行 Ent 代码生成**再编译：`go generate ./ent/...`
2. **Wire 重新生成**：`NewAuthService`、`NewRedeemService`、`NewGatewayService` 构造函数签名均已变更，Wire gen 必须重新执行
3. commission_repo.go 依赖的 `ent/commissionrecord` 包在 Ent 生成后才存在
4. `user.InviteCode/InviterID/AgentID` 字段在 Ent 生成后才能被 user_repo.go 中的 Ent builder 使用（当前代码使用 SetInviteCode 等方法，Ent 生成后会自动提供）
