# 营销联盟计划 V2.1 开发方案

- 日期：2026-07-26
- 分支：`feat/affiliate-program-v2-20260726`
- Worktree：`/Users/fujunhao/laoshirenai/worktrees/affiliate-program-v2`
- 状态：设计冻结后分阶段实现；默认不影响生产
- 时区：业务时间统一 `Asia/Shanghai`，数据库时间统一保存 UTC

## 1. 目标与硬约束

1. 所有用户都能推荐新用户，普通推荐保持简单。
2. 达标用户可申请成为代理商，代理商不需要充值押金。
3. 每笔符合条件的消费最多生成 10% 的代理激励池，禁止递归叠加。
4. 所有奖励分配完成后，压力场景毛利率不得低于 35%。
5. 只有计划正式启动后的真实付费与真实消费参与结算；历史数据不追溯发钱。
6. 联盟账务必须来源可追踪、幂等、可冲正、可审计，不使用浮点数结算。
7. 所有开发、测试与 checkpoint 在独立 worktree 完成；未经用户明确批准不得部署生产。

## 2. 资产与展示

- 现金佣金以人民币 `¥` 展示并可申请提现。
- 平台额度使用符号 `⚡`，不可提现、不可转账。
- 用户余额、赠送额度与使用额度使用 `⚡`，不得继续使用 `$` 造成美元误解。
- 官方模型美元成本、上游成本等真实美元语义仍保留 `$`。
- 金额账务使用整数微单位或 `decimal(20,8)`；提现最终按人民币分向下取整。

## 3. 普通用户推荐

- 每个用户拥有唯一邀请码与推荐链接。
- 被邀请人的第一次真实付费产品金额产生一次普通推荐奖励：
  - 邀请人获得首笔真实付费金额的 5% `⚡`，T+0 入账。
  - 若首笔真实付费金额严格大于 ¥50，被邀请人获得固定 `⚡5`，T+1 入账。
  - 首笔金额等于 ¥50 不获得固定 `⚡5`。
- 赠送、补偿、测试、历史卡密和无法归因的旧余额不触发奖励。
- 活跃代理商链接使用代理 10% 激励池，不再叠加普通邀请人的 5%。
- 被邀请人的首笔大于 ¥50 时，固定 `⚡5` 仍可发放。

## 4. 代理商资格

无押金，满足任一路径后成为候选人：

### 路径 A

- 直属有效消费用户不少于 10 人；
- 每人累计确认消费不少于 ¥20；
- 直属团队累计确认消费不少于 ¥1,000。

### 路径 B

- 本人加直属团队累计确认消费不少于 ¥2,000。

候选人提交收款资料，经过唯一主体与风险检查、管理员激活后成为正式代理商。保存 `qualified_at` 与 `activated_at`。激活前消费可用于资格判断，但不得追溯生成现金佣金。

## 5. 动态代理链接与 10% 激励池

- 正式代理商拥有 1 个默认链接，最多另建 5 个活动链接。
- 每个链接可设置客户返还率 `r`，范围 0% 到 10%，步长 1%。
- 每笔直属客户确认消费固定生成 10% 激励池：
  - 客户获得 `r%` 的 `⚡`；
  - 直属代理商获得 `(10-r)%` 的人民币现金佣金。
- 客户绑定时快照：
  - `customer_rebate_rate_snapshot`
  - `agent_commission_rate_snapshot`
  - `link_rate_version`
  - `bound_at`
- 链接费率修改只影响未来绑定用户。
- 已绑定客户费率只能提高，不得降低。
- 暂停链接不影响已经绑定的客户。

## 6. 层级与防套娃原则

- 邀请关系永久绑定，只结算直属关系边。
- 一笔消费只向消费者本人返还客户额度，并向其直属代理商生成剩余现金佣金。
- 不产生上上级抽成、团队级差或递归佣金。
- 用户成为代理商后：
  - 自己消费仍按原直属关系结算给原上级；
  - 自己不能获得自己的现金佣金；
  - 只有其新发展的直属客户消费才给其产生现金佣金。
- 一个自然人或实体只能对应一个代理主体。关联账号不计为独立有效人数，并可冻结或冲正异常佣金。

## 7. 确认消费与资金来源

### 按量付费

仅真实付费余额批次被扣减时产生确认消费。每笔余额必须带来源批次：

- `paid`：参与联盟；
- `referral_bonus`、`customer_rebate`、`commission_conversion`、`compensation`、`test`：不参与联盟；
- 上线时无法归因的旧余额标记为 `legacy_unattributed`，可继续使用但不参与联盟。

### 月卡

每个周期按以下公式确认消费：

`确认消费 = 销售价 × 已使用额度 / 周期额度上限`

同一周期最多确认到该周期的实际销售价。赠送额度和超出可结算上限的额度不产生联盟奖励。

## 8. 收款资料

在现有 `agent_payment_profiles` 基础上重构：

- 支付宝实名；
- 支付宝账号；
- 支付宝收款二维码；
- 联系电话；
- 信息真实性确认。

资料状态：`missing`、`pending_review`、`verified`、`rejected`。只有 `verified` 才可申请提现。已验证资料再次修改后重新进入 `pending_review`。二维码为私有文件，限制 JPG/PNG/WebP、最大 5MB、清除 EXIF、随机文件名并记录访问审计。

## 9. 代理商现金佣金与按需提现

### 9.1 佣金成熟

- 确认消费产生的代理现金佣金 T+0 成熟并进入 `available`。
- 命中风控的佣金进入 `risk_hold`，不计入可提现余额。
- 可提现佣金可部分或全部转换为 `⚡`，默认倍率 1.2；例如 ¥10 转为 `⚡12`。
- 转换不可撤销，不可提现，不参与资格计算，也不继续生成奖励。

### 9.2 提现规则

取消每日、每7日、每15日、每31日和固定结算日。改为：

- 默认最低提现金额 ¥100，可后台配置；
- 达到门槛后可随时申请全部或部分提现；
- 手续费默认 ¥0；
- 同时最多存在 1 笔进行中的申请；
- 正常处理承诺为提交后连续 24 小时内，按北京时间计算；
- 用户提交前一次性完成资格、资料、余额与风险校验；
- 提交成功即锁定余额并直接进入 `processing`；
- 不设置 `submitted`、`pending_review` 等用户状态；
- 不允许用户取消已经进入处理中的提现。

用户端正常流程只展示：

`处理中 → 已到账`

异常付款失败时，后台将锁定金额原子退回 `available`，用户收到“提现未完成，金额已退回”的异常通知；这不是正常流程中的审核状态。

### 9.3 后台付款

- 管理员看到按 SLA 剩余时间排序的处理中申请；
- 使用提交时冻结的支付宝资料快照扫码付款；
- 记录操作人、付款时间、支付宝交易号或付款凭证；
- 确认付款后原子更新为 `paid`；
- 12 小时提醒、剩余 4 小时红色预警、24 小时超时告警；
- 存在处理中提现时禁止修改收款资料。

## 10. 代理商社群通知

后台可配置私有代理商社群：启用状态、群名称、渠道类型、标题、正文、二维码、可选链接、过期时间、客服联系方式和版本号。

代理商激活后收到一次弹窗、站内通知和代理中心常驻卡片。代理商确认已加入后不再重复弹窗，卡片仍保留。配置换版可选择重新通知全部代理商。

## 11. 产品与毛利率门禁

### 按量付费

- ¥20 → `⚡20`
- ¥50 → `⚡50`
- ¥100 → `⚡100`

### 月卡建议目录

| 产品 | LDXP 售价 | 直售价格 | 周期额度 | 日额度 |
| --- | ---: | ---: | ---: | ---: |
| Starter | ¥259 | ¥249 | ⚡2400 | ⚡80 |
| Lite | ¥469 | ¥459 | ⚡4500 | ⚡150 |
| Pro | ¥869 | ¥839 | ⚡8500 | ⚡280 |

### 公开分组倍率建议

- GPT：0.42
- Claude/MAX：2.40
- GLM：2.80
- Grok：0.40
- 外部 Claude：2.60
- Bedrock：6.00
- 官方高成本：8.50

GPT 成本按 0.15 与 0.20 的 3:7 混合成本，即 0.185；LDXP 手续费按 3%。所有可公开销售的产品必须经过压力毛利率门禁，低于 35% 时拒绝启用。既有月卡周期保持原权益，不在开发阶段修改线上价格。

## 12. 程序开关与发布

- `affiliate_program_mode`: `off` / `shadow` / `live`
- `affiliate_program_started_at`: 北京时间输入，UTC 保存
- `affiliate_program_version`: `v2`
- `shadow` 只计算和记录观测结果，不进行任何奖励、现金或余额写入。
- 价格目录与联盟开关启用前必须原子校验 35% 毛利率门禁。
- 首次上线默认 `off`，经过 staging 验收后才能进入 `shadow`。
- 未经用户明确“可以上线”不得合并发布或部署生产。

## 13. 核心数据边界

计划新增或重构：

- `affiliate_program_settings`
- `affiliate_links`
- `affiliate_link_rate_versions`
- `affiliate_bindings`
- `affiliate_reward_entries`
- `balance_lots`
- `balance_lot_consumptions`
- `monthly_entitlement_cycles`
- `affiliate_performance_events`
- `affiliate_qualification_states`
- `agent_principals`
- `agent_cash_commission_entries`
- `agent_withdrawal_requests`
- `agent_withdrawal_events`
- `agent_payment_profiles`（重构）
- `agent_community_configs`
- `agent_community_notice_deliveries`

旧 `commission_records` 与 `agent_settlements` 保持只读兼容，不与 V2 的现金佣金和平台额度语义混用。

## 14. Checkpoint 顺序

每完成一个可独立验证的 feature 就提交一次：

1. `chore(dev): establish affiliate v2 worktree`
2. `feat(affiliate): add v2 settings and ledger schema`
3. `feat(billing): attribute paid balance and monthly consumption`
4. `feat(referral): reward first paid redemptions`
5. `feat(affiliate): add dynamic links and direct-edge settlement`
6. `feat(agent): add qualification and activation workflow`
7. `feat(agent): harden payment profile review`
8. `feat(agent): add immediate-processing withdrawals with 24h SLA`
9. `feat(agent): add community onboarding notice`
10. `feat(ui): use platform credit symbol and affiliate dashboards`
11. `feat(catalog): enforce affiliate margin floor`
12. `test(affiliate): cover ledger and end-to-end invariants`

每个 checkpoint 至少运行相关单元测试；涉及后端时运行编译检查，涉及前端时运行 typecheck/build。最终进行全量回归和 staging 验收，不直接部署生产。
