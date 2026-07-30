# 合伙人本人消费返佣上线与回滚门禁

`PARTNER_SELF_USAGE` 是前向兼容但不能向旧镜像静默回滚的资金策略。它只允许：

- 消费用户与合伙人是同一人；
- 客户平台返利为 `0%`；
- 合伙人现金返佣固定为 `10%`；
- 仅后台逐人开启后新购买的真实付费余额/月卡参与。

## 强制两阶段发布

### 阶段一：兼容代码上线，全部关闭

1. 备份数据库并记录目标、当前和上一版镜像 digest。
2. 先运行 migration `169_add_affiliate_self_commission_policies.sql`，再启动支持
   `PARTNER_SELF_USAGE` 的应用镜像。
3. 确认所有策略默认关闭，并验证普通邀请、普通合伙人分润、提现、佣金转额度、
   风控释放和冲正均正常。
4. 此阶段不得给任何生产账号开启本人消费返佣。

若阶段一失败，可以回滚到上一版应用镜像；此时数据库中不得已有任何
`PARTNER_SELF_USAGE` 余额批次、月卡周期或消费事件。

### 阶段二：单账号开启，再逐步扩大

1. 只选择一个状态正常、无任何上级关系的自有测试合伙人。
2. 后台开启后，重新购买真实付费额度；历史余额和历史月卡不得补算。
3. 完成部分消费、重复请求、风险冻结/释放、冲正、提现与转额度核对。
4. 资金账、余额批次、消费事件、现金钱包和页面金额完全一致后，才允许逐人开启。

## 最低回滚版本

首次产生 `PARTNER_SELF_USAGE` 购买批次后，**最低可回滚镜像必须仍支持该策略**。
关闭后台开关只阻止未来购买，不能改变已购买批次，因此不能恢复对旧镜像的兼容性。

上线或回滚前必须执行：

```sql
SELECT
  (SELECT COUNT(*) FROM affiliate_agent_self_commission_policies WHERE enabled) AS enabled_policies,
  (SELECT COUNT(*) FROM balance_lots WHERE affiliate_policy = 'PARTNER_SELF_USAGE') AS balance_lots,
  (SELECT COUNT(*) FROM monthly_entitlement_cycles WHERE affiliate_policy = 'PARTNER_SELF_USAGE') AS monthly_cycles,
  (SELECT COUNT(*) FROM affiliate_performance_events WHERE affiliate_policy = 'PARTNER_SELF_USAGE') AS performance_events;
```

只要后三项任意一项大于 `0`，就禁止回滚到不支持本策略的镜像，并在发布记录中保存
最低兼容 commit 与 digest。

migration 169 还要求本人消费事件明确携带
`metadata.attribution_policy = PARTNER_SELF_USAGE`。旧镜像缺少该元数据时，数据库会
拒绝整笔消费事务，避免“事件已占幂等键但现金未入账”的静默资金损失。出现该错误时
应立即恢复最低兼容镜像，不得删除事件、批次或放宽约束。

## 回滚规则

- 开启前：允许回滚至已验证的上一版 digest。
- 开启后：只允许回滚至支持 `PARTNER_SELF_USAGE` 的已验证 digest。
- 不使用移动 tag，不使用模糊的 `docker service rollback`。
- 不删除 migration 169 的表、事件或约束。
- 不通过手工改余额补偿；所有退款和冲正必须走来源明确的资金路径。
