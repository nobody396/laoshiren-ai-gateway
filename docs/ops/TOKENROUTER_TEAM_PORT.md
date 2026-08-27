# TokenRouter Team 本地化迁移说明

## 上游基线

- 仓库：`TokenFlux/TokenRouter`
- 审计提交：`521b19f35ae4d64deac376aef0e23dc70b7bfadd`
- Team 初始实现：`4e7aa4243373df9555f45ea97c214dc06901dd06`
- 后续关键修复：`752139a6`、`94bb18f9`、`e652943c`、`247f5231`、`766d4273`
- 上游许可：LGPL-3.0-or-later；移植代码必须保留来源和许可说明。

## 要迁移的产品能力

1. 一个用户同一时间只能属于一个团队；团队只有一个 Owner。
2. Owner 创建团队、邀请成员、设置成员上限和日/周/月消费限额。
3. 成员创建独立的 Team Key；权限、分组、订阅和余额来自当前 Owner。
4. 用量保留两个身份：实际调用成员 `actor_user_id` 与付款用户
   `billing_user_id`，并持久化 `team_id`。
5. 移除成员、暂停或解散团队后，相关 Team Key 立即失效。
6. 支持成员离队、邀请撤销/重发、所有权转让，以及管理员兜底管理。
7. 团队页面展示成员、Key、限额和按成员归因的用量，不泄露完整 Key。

## 本地化边界

- **不直接 cherry-pick**：TokenRouter 与本仓库没有可用 merge-base，初始 Team
  提交同时改动 195 个文件；本仓库还包含月卡、余额批次、Reliability、
  universal routing 等本地合同。
- **保持现有计费语义**：Team Key 只改变付款主体，不改变 31 天月卡、余额批次
  FIFO 归因、分组倍率、账户倍率、AccountingCommand 或供应商结算规则。
- **失败关闭**：找不到有效团队、成员、Owner，或团队已暂停时，Team Key 必须
  拒绝请求，不能回退为成员个人计费。
- **渐进开放**：功能由 `team.enabled` 和 `team.self_service_enabled` 控制；上线
  前可先关闭用户自助创建，由管理员建立试点团队。
- **邮件边界**：邀请属于系统事件触发的交易通知；只复用
  `老实人AI <no-reply@laoshirenai.com>` 的现有邮件通道，不增加备用发件人。
- **不在本任务部署生产**：本分支只完成代码、迁移、测试、构建和审查证据。

## 验证门槛

- 数据库迁移可在空库和已有数据上重复执行，旧个人 Key/用量回填正确。
- 单元/集成测试覆盖邀请并发、成员上限、Owner 转让、暂停/解散、Key 生命周期、
  Owner 计费、成员用量归因和缓存失效。
- 现有个人 Key、余额、订阅、月卡、universal routing 和 Reliability 测试不回归。
- 前端类型检查、生产构建和 Team 页面关键交互测试通过。
