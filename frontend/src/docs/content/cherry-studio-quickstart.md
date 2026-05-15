# 老实人 AI × Cherry Studio 快速开始指南

## 适用场景

这篇文档只解决一件事：把 Cherry Studio 接到 老实人 AI 中转站，并完成一次可用性验证。

---

## 一句话快速开始

只需要填这 3 项就能用：

- **Base URL**：`https://api.laoshirenai.com`
- **API Key**：你在 老实人 AI 获取的 key
- **模型 ID**：例如 `claude-opus-4-6`

不会填？按下面的步骤一步一步来。

---

## 1. 准备工作

在开始前，请确认：

- 已安装 Cherry Studio
- 已注册 [老实人 AI](https://laoshirenai.com) 账号
- 已获取 API Key（形如 `sk-xxxx`）

### 创建 API 密钥

登录 老实人 AI 控制台，左边栏进入 **API 密钥** → 点击 **创建密钥**。

- **名称**：可以随便填，比如 `laoshirenai`
- **分组**：根据你要使用的工具选择
  - 使用 **Codex**（OpenAI 工具）→ 选 `codex` 分组
  - 使用 **Claude Code**（Anthropic 工具）→ 选 Claude 对应分组
- 其他字段可保持默认
- 点击创建

![创建密钥](https://assets.example.com/dragoncode-cherry-studio/page2_img1.png)

创建完成后，在列表中点击复制按钮拿到完整的 API Key。

![复制 API Key](https://assets.example.com/dragoncode-cherry-studio/page3_img1.png)

### 如何确认 API Key 正确

- 能在 老实人 AI 后台看到该 Key
- 复制后是完整的一串字符，没有空格或截断

### 常见卡点

- 没有 API Key → 无法使用
- 复制少一位 → 会报 `401`
- Base URL 写错 → 会报 `404` / `400`

---

## 2. 打开 Cherry Studio 设置

1. 打开 Cherry Studio
2. 点击左下角/右上角 **设置**

![Cherry Studio 设置入口](https://assets.example.com/dragoncode-cherry-studio/page4_img1.png)

---

## 3. 添加 老实人 AI 提供商

1. 在模型服务中点击 **+ 添加**

   ![添加提供商入口](https://assets.example.com/dragoncode-cherry-studio/page5_img1.png)

   提供商名称可以随便填，例如 `laoshirenai`。

   ![填写提供商名称](https://assets.example.com/dragoncode-cherry-studio/page5_img2.png)

2. 选择提供商类型

   Cherry Studio 支持 **OpenAI** 和 **Anthropic** 两种类型，按你要用的模型系列选择。

   ![选择提供商类型](https://assets.example.com/dragoncode-cherry-studio/page6_img1.png)

3. 填写 API 密钥和 API 地址（下图以 OpenAI 为例）

   - **API 密钥**：你在 老实人 AI 创建的 API Key
   - **API 地址**：`https://api.laoshirenai.com`

   ![填写 API 密钥和地址](https://assets.example.com/dragoncode-cherry-studio/page7_img1.png)

   > **注意**：API Key 要和分组匹配 —— OpenAI 类型只能选 `codex` 分组创建的 Key，Anthropic 类型只能选 Claude 分组创建的 Key。

4. 添加模型（OpenAI 和 Anthropic 类型要填各自支持的模型 ID）

---

## 支持的模型

### OpenAI 兼容模型

| 模型名 |
|---|
| `gpt-5.4` |
| `gpt-5.2` |
| `gpt-5.3-codex` |

### Anthropic 兼容模型

| 模型名 |
|---|
| `claude-opus-4-6` |
| `claude-sonnet-4-6` |

> 模型 ID 必须和 老实人 AI 支持的**完全一致**，大小写、连字符都不能差。

### 添加模型

![添加模型](https://assets.example.com/dragoncode-cherry-studio/page8_img1.png)

添加完成后点击 **检测**，提示成功即可进入下一步。

![检测模型](https://assets.example.com/dragoncode-cherry-studio/page9_img1.png)

---

## 4. 开始聊天（验证接入）

1. 新建对话
2. 选择刚刚添加的模型

   ![选择模型](https://assets.example.com/dragoncode-cherry-studio/page10_img1.png)

3. 输入：`你好`

### 成功表现

模型正常回复内容即表示接入成功。

![成功对话](https://assets.example.com/dragoncode-cherry-studio/page11_img1.png)

如果没有回复或报错，参考下面的排查表。

---

## 5. 报错排查

看到什么报错，就对照下面这一条。

### 503 Service temporarily unavailable

- **含义**：当前模型通道不可用或太拥堵
- **解决**：重试；或切换到其他模型（推荐）

### 502 Bad gateway

- **含义**：老实人 AI 连接上游 Claude / GPT 失败
- **解决**：等 1~2 分钟再试；或切换模型

### AI_RetryError

- **含义**：已自动重试 3 次仍失败（本质通常是 502 / 503）
- **解决**：同上，切换模型或稍后重试

### 401 / 403

- **含义**：API Key 错误或分组不匹配
- **解决**：重新复制 Key；确认提供商类型与 Key 分组一致

### 400

- **含义**：模型 ID 或 URL 写错
- **解决**：检查
  - API 地址是否为 `https://api.laoshirenai.com`
  - 模型 ID 是否在[支持的模型](#支持的模型)列表中

### 一直转圈 / 卡住

- **含义**：网络、流式连接或模型异常
- **解决**：新开对话；重启 Cherry Studio；切换模型或分组

---

## 6. 一分钟自检

90% 的问题都出在这张表里。

| 项目 | 正确写法 |
|---|---|
| Base URL | `https://api.laoshirenai.com` |
| API Key | `sk-xxxx`（完整、无空格） |
| 模型 ID | 必须在支持列表中 |
| 分组与类型 | OpenAI ↔ `codex` 分组；Anthropic ↔ Claude 分组 |
