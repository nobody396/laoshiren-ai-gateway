# 易支付（皮卡丘）支付通道运维手册

迁移 194 引入统一支付网关（`backend/internal/payment`）与皮卡丘易支付
Provider（经典易支付 MD5 协议，默认 API Base `https://pay.hueling.cc`）。
覆盖三条业务线：**余额充值**（与虎皮椒并存、可按支付方式灰度）、
**新人 ¥5→¥10** 与**月卡（Plus/Pro/Max）站内扫码直付**（后两者均见
`native-checkout.md` 的 EasyPay 章节，按 offer 灰度，链动小铺外链保留回退）。

## 配置

管理后台「系统设置 → 易支付（皮卡丘）充值」卡片，对应 settings 键：

| 键 | 说明 |
| --- | --- |
| `easypay_enabled` | 总开关，默认关 |
| `easypay_pid` / `easypay_key` | 商户号与经典协议 MD5 密钥（密钥写一次后只显示已配置） |
| `easypay_api_base` | 留空 = `https://pay.hueling.cc` |
| `topup_alipay_provider` / `topup_wechat_provider` | `xunhu`（默认）或 `easypay`，按支付方式分别灰度 |

回调地址（填进皮卡丘商户后台或下单时自动携带）：
`<frontend_url>/api/v1/pay/notify/easypay`，**GET / POST 都收**，商户后台
「易支付异步回调请求方式」保持默认 GET 即可。回调只负责验签 + 顶起自查，
到账一律以主动查单 / 金额核对后的幂等完成为准。

## 不变量

- `topup_orders.provider` 在下单时写死；切换 `topup_*_provider` 不影响存量
  pending 订单——它们仍由下单时的通道收尾（虎皮椒回调与查单代码保持原样）。
- 到账的唯一入口仍是 `completeOrder`：`CompleteIfPending` 条件更新保证重复
  回调 / 轮询自愈只入账一次；金额、支付方式、订单 provider 不匹配一律拒绝。
- 密钥不进日志；回调验签失败回 `fail`，平台会重试。

## 上线顺序（余额充值）

1. 皮卡丘商户后台：配置通道（推荐「支付宝商家账单」，长期稳定不依赖登录态；
   微信通道依赖 Windows PC 监控工具，先不作为生产主通道）。
2. 管理后台填 PID/密钥并启用 `easypay_enabled`。
3. 先将 `topup_alipay_provider` 切到 `easypay`，微信保持 `xunhu`。
4. 测试账号小额实付（≥¥20）：站内二维码 → 支付 → 轮询/回调自动到账；
   重复回调不重复入账；管理端订单流水与账变记录齐全。
5. 观察 24–48 小时无误后，再评估微信通道。

## 回滚

把对应支付方式的 `topup_*_provider` 改回 `xunhu` 即可，即时生效；
虎皮椒配置不要清空。进行中的 easypay 订单仍按 easypay 协议收尾。

## 皮卡丘平台侧待办（非代码）

- 通道总数当前为 0：必须先配置并上线至少一条通道。
- 资金账户余额 ¥0 的性质（手续费账户？是否需要预存）需与服务方确认。
- 正式全量前完成供应商尽调（主体、条款、备案、口碑）。
