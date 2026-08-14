# 链动小铺新人卡手动销售运行手册

## 当前方案

原生结账已放弃并保持关闭。用户流程固定为：

1. 已登录用户在 `/get-subscription` 选择“新人特惠 · 10 元余额包”。
2. 点击“去卡密商城购买”，打开 `https://pay.ldxp.cn/item/oc3w4r`。
3. 用户在链动小铺自行完成 ¥5 支付并取得卡密。
4. 用户回到老实人AI `/redeem` 输入卡密，到账 ¥10 余额。

链动商品 ID 为 `746430`，goods key 为 `oc3w4r`。站内原生 offer
`newcomer-balance-5-to-10` 必须始终保持 `enabled=false`，且
`native_checkout_offer_testers` 不得保留测试账号。

## 商业与兑换不变量

- 实付 ¥5，到账 ¥10；用户侧不展示内部“纯赠送余额”分类。
- 每个老实人AI账号终身仅可兑换一次该新人商品。链动匿名购买无法在付款前可靠识别老实人AI账号，因此页面必须明确提示“每个账号仅可兑换 1 次，请勿重复购买”。
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
