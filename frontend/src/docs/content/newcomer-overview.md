# 新手总览：从注册到第一次成功调用

第一次使用老实人AI，不需要先理解所有模型和协议。按照这条路线操作即可：

```text
注册账号 → 选择按量付费或月卡 → 充值/兑换 → 创建 API Key → 选择分组 → 配置工具 → 发出第一次请求
```

本文会先帮你选计费方式，再解释 API Key、分组、Base URL 和模型之间的关系。完整参数请查阅《API参考总览》，不要在新手阶段一次配置所有功能。

---

## 一分钟结论

- 偶尔使用、调用量不确定：选择**按量付费**。
- 每天持续使用 Codex、Claude Code 或 Grok：通常选择**月卡更实惠**。
- 第一次使用 Codex：优先选择站内推荐的 **0.5 Pro** 或对应月卡GPT分组。
- 更看重速度和稳定性：选择**企业级分组**。
- 生成图片：单独创建 **GPT-Image分组**的Key，不要和文本Key混用。
- 一个 API Key 只绑定一个分组；需要使用不同产品时，建议分别创建Key并写清用途。

> 月卡并不是任何情况下都更便宜。使用频率很低时，按量付费可能更省；月卡的优势是固定周期内用得越充分，平均成本越低。

---

## 第一步：注册并登录

打开 [老实人AI](https://laoshirenai.com)，完成注册和登录。

登录后主要会用到以下页面：

| 页面 | 用途 |
|---|---|
| API Keys | 创建、查看和管理API Key |
| 充值/兑换 | 充值按量余额，或兑换月卡、余额卡 |
| 模型与价格 | 查看当前分组、模型和价格 |
| 使用记录 | 查看请求、Token、模型和费用 |
| 文档 | 查找客户端配置和错误处理方式 |

<!-- SCREENSHOT:newcomer-01-dashboard route=/dashboard focus=left-nav redact=balance,email -->

---

## 第二步：按量付费和月卡怎么选

这是新手最容易混淆的地方。

### 按量付费

按量付费的逻辑是：

```text
先充值余额 → 创建公开分组Key → 每次请求按实际用量扣余额
```

适合：

- 偶尔使用；
- 暂时不知道一个月会用多少；
- 需要灵活切换不同价格和服务等级；
- 开发阶段做小规模测试；
- 不希望购买固定周期权益。

特点：

- 用多少扣多少；
- 不同公开分组有不同倍率、速度和稳定性；
- 可以根据任务选择低价混池、Pro或企业级；
- 余额、价格和有效状态以控制台实时显示为准。

### 月卡

月卡的逻辑是：

```text
购买/兑换一个月权益 → 激活GPT、Claude、Grok三个Host → 在周期内使用共享月额度
```

当前在售月卡分为 Plus、Pro、Max。档位越高，月额度越高。每档都包含：

- GPT/Codex Host；
- Claude Host；
- Grok Host。

适合：

- 每天使用Codex、Claude Code或Grok；
- 有稳定开发任务；
- 希望一个月成本更容易预算；
- 能够比较充分地使用月额度；
- 同时需要GPT、Claude和Grok。

特点：

- 固定周期、固定权益；
- 三个Host共享该月卡的月额度；
- 持续、高频使用时，平均成本通常比按量付费更低；
- 到期后需要续费或开启新的周期；
- 未充分使用月额度时，不一定比按量付费划算。

### 一张表看懂区别

| 对比项 | 按量付费 | 月卡 |
|---|---|---|
| 付费方式 | 先充值余额，用多少扣多少 | 购买一个固定周期权益 |
| 适合人群 | 偶尔使用、用量不确定 | 每天使用、用量稳定 |
| 成本特点 | 灵活，低频更省 | 高频使用通常更实惠 |
| 使用期限 | 以账户余额和控制台状态为准 | 有明确月卡周期 |
| 模型入口 | 创建公开分组Key | 兑换后使用月卡专属分组 |
| GPT/Claude/Grok | 分别选择对应公开分组 | 一档月卡包含三个Host |
| 速度选择 | 可选混池、Pro、企业级等 | 使用月卡当前配置的线路池 |
| 适合测试 | 很适合 | 不建议只为一次小测试购买 |

<!-- SCREENSHOT:newcomer-02-pricing route=/pricing focus=payg-vs-monthly redact=none -->

### 简单选择题

**一个月只用几次？** 先用按量付费。

**每天都开着Codex或Claude Code？** 月卡通常更合适。

**不确定是否适合自己？** 先按量充值少量余额测试，确认客户端、模型和速度后再决定月卡。

---

## 第三步：理解API Key、分组、模型和Base URL

### API Key是什么

API Key是客户端调用老实人AI的凭证，通常以 `sk-` 开头。

API Key不是登录密码、订单号、兑换码或月卡卡密，也不是可以公开分享的链接。

不要把完整Key发到群聊、工单、截图、Git仓库或前端网页代码里。

### 分组是什么

创建Key时必须选择分组。分组决定：

- 可以看到哪些模型；
- 使用GPT、Claude、Grok还是生图协议；
- 走哪些上游线路；
- 账号倍率和用户计费；
- 当前速度、稳定性和容量。

一个Key只绑定一个分组。分组选错时，即使Base URL和模型名都正确，也可能返回403或模型不可用。

### 模型是什么

模型是请求里的 `model` 字段，例如：

```text
gpt-5.6-sol
claude-sonnet-5
grok-4.6
gpt-image-2
```

创建Key后应使用该Key调用 `GET /v1/models`，只使用实际返回的模型ID。

### Base URL是什么

Base URL是客户端访问API的地址。不同客户端会不会自动追加 `/v1`，规则不同。

| 客户端/协议 | 推荐Base URL |
|---|---|
| Codex CLI/App | `https://api.laoshirenai.com` |
| Claude Code | `https://api.laoshirenai.com` |
| 普通OpenAI SDK | `https://api.laoshirenai.com/v1` |
| Anthropic SDK | `https://api.laoshirenai.com` |
| GPT-Image | `https://api.laoshirenai.com/gpt-image/v1` |

不要把 `/v1` 重复拼接成 `/v1/v1`。

---

## 第四步：充值或兑换月卡

### 按量付费用户

进入充值页面，充值成功后确认账户余额已经更新，再创建公开分组Key。

### 月卡用户

进入兑换页面，输入月卡卡密。兑换成功后，确认账户已经出现对应的月卡权益和到期时间。

月卡会激活GPT、Claude和Grok三个Host。建议为不同Host分别创建Key，例如：

```text
Codex-月卡
Claude-月卡
Grok-月卡
```

这样出现问题时更容易判断是哪一个工具、分组或模型。

<!-- SCREENSHOT:newcomer-03-redeem route=/redeem focus=redeem-and-success redact=code,balance -->

---

## 第五步：创建第一个API Key

进入 [API Keys](https://laoshirenai.com/keys)，点击创建。

建议填写容易识别的名称，例如 `我的Codex`、`我的Claude-Code`、`测试-OpenAI-SDK` 或 `GPT-Image-生图`，然后选择分组。

### 按量付费常见选择

| 需求 | 建议 |
|---|---|
| 价格优先、接受高峰波动 | 低价混池 |
| 第一次使用Codex、兼顾价格和体验 | 0.5 Pro |
| 速度和稳定性优先 | 企业级 |
| 特定高级模型 | 对应专用分组 |
| 生图 | GPT-Image专用分组 |

### 月卡用户

兑换月卡后，在创建Key时选择对应的月卡GPT、Claude或Grok分组。看不到月卡分组时，先确认：

1. 是否登录了兑换卡密时使用的同一个账号；
2. 月卡是否仍在有效期内；
3. 兑换是否真的成功；
4. 是否需要刷新页面。

<!-- SCREENSHOT:newcomer-04-create-key route=/keys focus=create-dialog-group-selector redact=existing-keys -->

### 创建完成后

完整Key通常只在创建时显示一次。请立即保存到密码管理器或客户端安全配置中。

不要把Key保存在Git仓库、README、网页前端源码、群聊、未打码截图或公开日志里。

<!-- SCREENSHOT:newcomer-05-key-created route=/keys focus=created-key-actions redact=full-key -->

---

## 第六步：配置你的工具

### Codex

优先使用站内一键配置，并阅读《Codex快速开始指南》。核心配置是：

```text
Provider：老实人AI/OpenAI兼容
Base URL：https://api.laoshirenai.com
协议：Responses
API Key：刚创建的完整Key
```

### Claude Code

阅读《Claude Code快速开始指南》。核心配置是：

```text
ANTHROPIC_BASE_URL=https://api.laoshirenai.com
API Key/Token=刚创建的Claude分组Key
```

### OpenAI SDK

普通OpenAI SDK通常使用 `https://api.laoshirenai.com/v1`。不要把Codex根地址和普通SDK的 `/v1` 地址混为一谈。

<!-- SCREENSHOT:newcomer-06-one-click route=/keys focus=copy-use-ccswitch redact=full-key -->

---

## 第七步：发出第一次请求

### 先读取模型列表

```bash
curl 'https://api.laoshirenai.com/v1/models' \
  -H 'Authorization: Bearer YOUR_API_KEY'
```

### 再发送一个最小Responses请求

```bash
curl 'https://api.laoshirenai.com/v1/responses' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-5.6-sol",
    "input": "只回复 OK",
    "max_output_tokens": 64,
    "stream": false
  }'
```

如果模型列表里没有 `gpt-5.6-sol`，请换成列表中实际返回的模型。

### 什么算调用成功

- HTTP状态码为200；
- 返回内容不是错误JSON；
- 流式请求最终收到完成事件；
- 使用记录中出现一条对应请求；
- 余额或月额度按照用量变化。

只看到客户端开始转圈，不代表请求已经成功完成。

---

## 第八步：查看使用记录和费用

进入使用记录页面，可以核对请求时间、API Key、分组、模型、Token、费用和请求状态。

如果费用和预期不同，先确认是否存在长上下文、大量输出、缓存创建/命中、Fast/Priority模式、图片视频能力，或者选错分组和模型。

<!-- SCREENSHOT:newcomer-07-usage route=/usage focus=request-row-and-details redact=email,key,request-content -->

---

## 第九步：文本和生图必须分开

文本模型Key用于Codex、Chat Completions、Responses或Claude等文本请求。

生图请单独创建GPT-Image分组Key，并使用：

```text
https://api.laoshirenai.com/gpt-image/v1
```

模型为 `gpt-image-2`。

不要依赖普通聊天客户端把文本请求自动变成生图请求。需要生图时，应使用支持Images API的客户端或站内生图Skill。

<!-- SCREENSHOT:newcomer-08-image-group route=/keys focus=gpt-image-group-and-base-url redact=existing-keys -->

---

## 第十步：最常见的错误

### 401：API Key错误

检查Key是否完整、是否多了空格或引号、客户端是否仍在读取旧环境变量，以及Key是否已被删除或停用。

### 403：分组或模型无权限

检查Key绑定的分组、`GET /v1/models`是否包含目标模型、文本Key是否错误调用了生图，以及月卡是否到期。

### 404：路径或模型不存在

检查Base URL是否重复 `/v1`，模型ID是否拼错。

### 429：请求过多

降低并发，读取 `Retry-After`，使用指数退避。不要立即无限重试。

### 500 / 502 / 503 / 529

可能是网关或上游暂时异常。保留请求时间、模型、状态码和请求ID，有限重试1～2次后再反馈。

更多内容请阅读《常见API报错排查》。

---

## 新手选择流程

```text
我只是偶尔用？
├─ 是 → 按量付费
└─ 否 → 每天持续使用？
   ├─ 是 → 月卡通常更实惠
   └─ 不确定 → 先少量按量测试

我要用什么？
├─ Codex/GPT → GPT公开分组或月卡GPT Host
├─ Claude Code → Claude公开分组或月卡Claude Host
├─ Grok → Grok公开分组或月卡Grok Host
└─ 生成图片 → GPT-Image专用分组

我优先考虑什么？
├─ 价格 → 低价混池
├─ 平衡 → 0.5 Pro
└─ 速度和稳定性 → 企业级
```

---

## 下一篇应该看什么

| 目标 | 推荐文档 |
|---|---|
| 配置Codex | Codex快速开始指南 |
| 配置Claude Code | Claude Code快速开始指南 |
| 不知道Base URL怎么填 | Base URL填写总指南 |
| 开发自己的程序 | API参考总览 |
| 遇到报错 | 常见API报错排查 |
| 使用生图 | GPT-Image-2使用指南 |
| 不理解Key和分组 | API Key与分组选择指南 |

如果需要客服排查，请提供请求时间、客户端、协议、模型、状态码和请求ID。不要发送完整API Key。
