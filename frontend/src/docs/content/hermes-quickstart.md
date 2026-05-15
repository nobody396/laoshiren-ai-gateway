# 老实人 AI × Hermes 快速开始指南

## 适用场景

这篇文档只解决一件事：把 老实人 AI 的 Anthropic 兼容接口接到 Hermes，并完成一次最小可用验证。

如果你当前用的是 Hermes，而不是 OpenClaw、Cherry Studio 或其他客户端，这篇文档就是对应的接入方式。

---

## 1. 准备事项

开始前请先确认以下几点：

- 你已经在 [老实人 AI 控制台](https://laoshirenai.com/keys) 创建好一枚 API Key
- 你知道 Hermes 当前要调用的实际模型名
- 你可以编辑 Hermes 的配置文件 `config.yaml`
- 你本次要接的是 老实人 AI 的 Anthropic 兼容端点，不是 OpenAI 兼容端点

---

## 2. 支持的模型

当前这篇文档优先覆盖 老实人 AI 的 Anthropic 兼容模型接入，常见模型名如下：

| 模型名 |
|---|
| `claude-opus-4-6` |
| `claude-sonnet-4-6` |
| `claude-haiku-4-5-20251001` |

如果你不确定自己账号当前实际可用的模型，以控制台或你现有可调用记录为准。

---

## 3. 推荐路径：通过 Hermes 的 `custom_providers` 接入

Hermes 接 老实人 AI 时，核心思路是：

1. 在 `custom_providers` 中声明一个 老实人 AI provider
2. 在 `model` 块中把默认模型指向这个 provider
3. 关闭 `smart_model_routing`，避免 Hermes 自动切到别的模型

### 第一步：定义自定义 provider

```yaml
custom_providers:
  - name: custom-laoshirenai-codes-aws
    base_url: https://api.laoshirenai.com
    api_key: <your-key>
    api_mode: anthropic_messages
    models:
      - claude-opus-4-6
```

这里最关键的是两点：

- `base_url` 写 `https://api.laoshirenai.com`
- `api_mode` 必须写 `anthropic_messages`

### 第二步：把默认模型指向这个 provider

```yaml
model:
  default: claude-opus-4-6
  provider: custom-laoshirenai-codes-aws
  base_url: https://api.laoshirenai.com
  api_key: <your-key>
  api_mode: anthropic_messages
```

这里的 `provider` 必须和上面 `custom_providers` 里的 `name` 对应上，否则 Hermes 不会走到你定义的 老实人 AI provider。

### 第三步：关闭 Smart Model Routing

```yaml
smart_model_routing:
  enabled: false
```

如果不关，Hermes 可能会因为消息较短而自动切到更便宜的模型，导致你明明配置了 `claude-opus-4-6`，实际跑的却不是它。

---

## 4. 可直接参考的最小可用配置

下面是一份合并后的最小可用示例。把 `<your-key>` 换成你自己的 老实人 AI API Key 即可：

```yaml
model:
  default: claude-opus-4-6
  provider: custom-laoshirenai-codes-aws
  base_url: https://api.laoshirenai.com
  api_key: <your-key>
  api_mode: anthropic_messages

custom_providers:
  - name: custom-laoshirenai-codes-aws
    base_url: https://api.laoshirenai.com
    api_key: <your-key>
    api_mode: anthropic_messages
    models:
      - claude-opus-4-6

smart_model_routing:
  enabled: false

fallback_providers:
  - provider: minimax-cn
    model: MiniMax-M2.7
```

如果你想切换到别的 Anthropic 兼容模型，只需要把 `model.default` 和 `custom_providers[].models` 中的模型名一起改掉。

---

## 5. 成功标准

满足下面几项，基本就说明 Hermes 和 老实人 AI 已经接通：

- `base_url` 填写的是 `https://api.laoshirenai.com`
- `api_mode` 填写的是 `anthropic_messages`
- `model.provider` 指向的是你在 `custom_providers` 中定义的 provider 名
- `smart_model_routing.enabled` 已关闭
- Hermes 发起消息后，实际能正常收到模型回复

---

## 6. 常见问题

### 为什么不能按 OpenAI 兼容方式接？

这次 Hermes 接 老实人 AI，应该走 Anthropic 原生消息格式，也就是 `/v1/messages`。

已知结论是：

- `/v1/messages` 可用
- `/v1/chat/completions` 不适合作为这篇 Hermes 配置方案的主路径

所以 `api_mode` 必须写成 `anthropic_messages`，不要按 OpenAI 兼容模式去配。

### 为什么我明明选了 `claude-opus-4-6`，实际却跑了别的模型？

优先检查 `smart_model_routing.enabled` 是否已经关闭。

如果 Hermes 开着智能路由，短消息可能会被自动切到便宜模型，导致你的界面选择和真实请求模型不一致。

### `api_key_env` 能不能用？

基于当前这份配置记录，Hermes v0.10.0 下的 `api_key_env` 在 `model` 和 `custom_providers` 块里存在不稳定情况，可能导致环境变量没有正确展开，最后返回 401。

如果你已经确认是这个问题，可以先退回到直接填写 `api_key` 的方式，先把链路打通，再决定是否继续排查环境变量方案。

### 为什么返回 401？

通常先查这几项：

- API Key 是否有效
- `api_key` 是否填到了 `model` 和 `custom_providers` 需要的位置
- Hermes 当前是否真的加载到了你修改后的配置文件

### 为什么返回 503 或服务不可用？

如果你当前配置走的是 OpenAI 兼容路径，先回头检查是不是把 Hermes 配成了错误的 `api_mode`。

这篇文档的推荐路径是 Anthropic 兼容接法，不建议把它和 OpenAI 兼容写法混用。

---

## 7. API 验证示例

如果你想先绕过 Hermes，直接验证 老实人 AI 端点本身是否可用，可以先调用 Anthropic 原生端点：

```bash
curl -X POST "https://api.laoshirenai.com/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: <your-key>" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-opus-4-6","max_tokens":50,"messages":[{"role":"user","content":"hi"}]}'
```

如果这个请求能正常返回，再回头检查 Hermes 配置，排障会更直接。

---

## 8. 下一步

- 想继续接其他客户端：查看 `OpenClaw 快速开始指南`、`Cherry Studio 快速开始指南`
- 还没创建 Key：回到 [老实人 AI 控制台](https://laoshirenai.com/keys) 先创建 API Key
- 想统一整理开发工具配置：继续查看站内其他快速接入文档
