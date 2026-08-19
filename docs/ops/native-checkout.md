# 链动小铺新人卡手动销售运行手册

## 当前方案

原生结账已放弃并保持关闭。用户流程固定为：

1. 已登录用户在 `/get-subscription` 选择“新人特惠 · 10 元余额包”。
2. 页面先按当前登录账号查询领取状态；已领取时整张新人商品卡隐藏。
3. 未领取账号点击“去卡密商城购买”时，服务端再次核对领取状态，再返回
   `https://pay.ldxp.cn/item/oc3w4r`。
4. 用户在链动小铺自行完成 ¥5 支付并取得卡密。
5. 用户回到老实人AI `/redeem` 输入卡密，到账 ¥10 余额。

链动商品 ID 为 `746430`，goods key 为 `oc3w4r`。站内原生 offer
`newcomer-balance-5-to-10` 必须始终保持 `enabled=false`，且
`native_checkout_offer_testers` 不得保留测试账号。

## 商业与兑换不变量

- 实付 ¥5，到账 ¥10；用户侧不展示内部“纯赠送余额”分类。
- 每个老实人AI账号终身仅可兑换一次该新人商品。链动匿名购买无法在付款前可靠识别老实人AI账号，因此页面必须明确提示“每个账号仅可兑换 1 次，请勿重复购买”。
- 充值页和充值弹窗必须在领取状态未确认、查询失败或已领取时隐藏新人商品；点击购买前必须再次查询，禁止直接信任公开设置中的外链。
- 用户绕过站内页面直接重复购买仍无法由匿名链动订单拦截；其第二张卡在本站兑换时必须返回 `REDEEM_OFFER_ALREADY_CLAIMED`，且卡保持未使用。
- 终身限购的最终权威是 `native_checkout_manual_claims` 的 `(offer_code, user_id)` 主键。服务层会提前返回友好冲突；数据库触发器负责阻止两个不同卡密的并发绕过。
- 新人库存仍登记在 `native_checkout_redeem_inventory`，并继续由 offer 行校验卡密语义：`type=balance`、`value=10`、`paid_value=0`、`purpose=gift`、`sales_status=gifted`、`validity_days=0`。
- 只有 `manual_redeem_enabled=true` 的库存允许公开手动兑换；其他已登记库存仍被公共兑换接口拒绝。
- 卡密使用、终身 claim、余额到账和账变记录必须在同一数据库事务内提交；失败时一起回滚。

## 上架顺序

1. 保持链动商品下架，部署并验证迁移 190。
2. 回读 offer：`enabled=false`、`manual_redeem_enabled=true`、`once_per_user=true`、实付 500 分、到账 1000 分。
3. 确认生产数据库与链动商品仍是同一批 10 张未使用卡，卡密正文不得进入日志或聊天。
4. 在 `card_shop_products` 中保留已有商品，只新增或更新：
   - `id=newcomer-5-to-10`
   - `label=新人特惠 · 10 元余额包`
   - `amount_cny=5`
   - `url=https://pay.ldxp.cn/item/oc3w4r`
   - `enabled=true`
5. 将链动商品设为销售中，保持隐藏目录（`show=0`），精确回读价格 ¥5、库存和至少一个可用支付通道。
6. 验证 `/get-subscription` 首张卡显示“实付 ¥5 · 到账 ¥10”，按钮打开正确链接；`/redeem` 可完成一次兑换。
7. 用同一账号的第二张同批卡验证返回 `REDEEM_OFFER_ALREADY_CLAIMED`，且第二张卡仍为未使用、余额不增加。

## 回滚

任一门禁失败时：

1. 先将链动商品设为下架。
2. 从 `card_shop_products` 删除或禁用 `newcomer-5-to-10`，保留其他商品。
3. 将 offer 的 `manual_redeem_enabled=false`，同时确认 `enabled=false`、tester 数量为 0。
4. 不删除订单、库存、claim 或已使用卡密记录。

## EasyPay 站内扫码模式（迁移 194/195 之后）

代码已支持 offer `provider='easypay'` 的原生扫码流程：站内下单（支付宝/微信可选）
→ 皮卡丘易支付回调 `POST|GET /api/v1/pay/notify/easypay` 顶起 reconcile worker
→ worker 主动查单确认支付 → 按订单快照内部铸造等额兑换码
（`external_order_no=<平台单号>`，幂等；迁移 195 的部分唯一索引兜底）
→ 走与库存卡完全相同的 ClaimFulfillment → 兑换 → 完成链路，余额、账变、
终身 claim 在同一事务语义下提交。终身限购权威仍是
`native_checkout_manual_claims`，手工卡与原生扫码共用。

### 上线（从手动模式切到 EasyPay 原生模式）

前置：皮卡丘商户后台至少一条通道在线（推荐支付宝商家账单），管理后台
「易支付（皮卡丘）」已填 PID/密钥并启用。

```sql
UPDATE native_checkout_offers
SET provider = 'easypay',
    provider_goods_key = 'newcomer-balance-5-to-10',
    enabled = TRUE,
    manual_redeem_enabled = TRUE,  -- 保留存量手工卡可兑换
    updated_at = NOW()
WHERE code = 'newcomer-balance-5-to-10';
```

上线后前端自动切到原生扫码（offer 可见且 provider=easypay）；手动购买入口
自动隐藏。验证：测试账号实付 ¥5 → 站内二维码 → 到账 ¥10 → 第二单被
`NATIVE_CHECKOUT_ALREADY_CLAIMED` 拒绝；再用一张存量手工卡确认仍返回
`REDEEM_OFFER_ALREADY_CLAIMED`。

### 回滚（退回手动 LDXP 模式）

```sql
UPDATE native_checkout_offers
SET provider = 'ldxp',
    provider_goods_key = 'oc3w4r',
    enabled = FALSE,
    manual_redeem_enabled = TRUE,
    updated_at = NOW()
WHERE code = 'newcomer-balance-5-to-10';
```

进行中的 easypay 订单会继续由 worker 按 easypay 协议收尾（订单自身记录了
provider）；新的购买回到手动卡密流程。

## 月卡 EasyPay 直付

月卡（Plus/Pro/Max 订阅卡）复用同一套原生结账引擎：站内选择套餐 → 支付宝/
微信扫码 → 回调顶起 worker → 查单确认 → 按订单快照铸造**订阅兑换码**
（`type=subscription`，带分组与有效期）→ 同一兑换链自动开通订阅。用户全程
不跳转、不见卡密；链动小铺外链保持为回退通道（offer 不可见时前端自动回到
外链）。

### 约定

- **offer code 必须等于前端套餐 id**（`plus` / `pro` / `max`，见
  `frontend/src/constants/monthlyCreditCards.ts`）。前端按此约定把选中套餐
  映射到站内扫码卡片；不一致则静默回退外链。
- `provider_goods_key` 填与 code 相同即可（easypay 不做上游商品校验，仅作
  对账标识）。
- `once_per_user=false`：月卡可复购续期。进行中的订单仍被
  `uq_native_checkout_orders_active_per_user` 部分唯一索引拦截，不会并发重复
  下单；已完成的订单不拦截复购。
- 铸码语义由订单快照决定并被库存触发器按 offer 行复核：
  `type=subscription`、`value=售价（元）`、`paid_value=0`、
  `purpose=sale_recharge`、`sales_status=sold`（实付销售，参与联盟归因）、
  `group_ids=套餐订阅分组`、`validity_days=31`。
- `benefit_amount_cny_fen` 约定填名义面值分（= `redeem_value * 100`）；订阅
  类前端不把它当“到账余额”展示。

### 上架（每个套餐一条，默认关闭）

分组 id 以生产 `groups` 表 / 月卡 host catalog 为准（Plus 示例）：

```sql
INSERT INTO native_checkout_offers (
    code, provider, provider_goods_key, name, description, product_kind,
    pay_amount_cny_fen, benefit_amount_cny_fen, redeem_type,
    redeem_value, redeem_paid_value, redeem_purpose, redeem_sales_status,
    redeem_group_ids, redeem_validity_days, once_per_user, enabled, sort_order
) VALUES (
    'plus',                 -- == 前端套餐 id；pro / max 同理各插一条
    'easypay',
    'plus',                 -- provider_goods_key：对账标识，与 code 保持一致
    'Plus 月卡',
    '31 天开发额度，站内扫码自动开通',
    'subscription',
    25900,                  -- pay_amount_cny_fen：实付 ¥259
    25900,                  -- benefit_amount_cny_fen：名义面值 = redeem_value*100
    'subscription',
    259,                    -- redeem_value：售价（元），用于联盟归因
    0,                      -- redeem_paid_value：订阅码固定为 0
    'sale_recharge',
    'sold',
    '[<gpt_group_id>, <claude_group_id>]'::jsonb,  -- 替换为该套餐的订阅分组 id
    31,
    FALSE,                  -- once_per_user：月卡可复购
    FALSE,                  -- enabled：先黑暗入库，验收后再开
    10
);
```

验收顺序：先保持 `enabled=false`，用 `native_checkout_offer_testers` 放行测试
账号实付一单（站内扫码 → 自动开通 → 订阅页可见 31 天有效期 → 订单
completed），再对该套餐 `UPDATE native_checkout_offers SET enabled = TRUE,
updated_at = NOW() WHERE code = 'plus';` 正式可见。

### 回滚

```sql
UPDATE native_checkout_offers SET enabled = FALSE, updated_at = NOW()
WHERE code IN ('plus', 'pro', 'max');
```

offer 不可见后前端立即回到链动小铺外链；进行中的 easypay 月卡订单仍由 worker
按协议收尾并完成开通。不删除订单与已铸造/已兑换的卡密记录。
