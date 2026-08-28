# 快速开始

创建 API Key，查询可用模型，再完成一个最小请求。

## 1. 创建 API Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择分组并创建 Key。

> 同一个模型可能由多个分组提供。请根据预算和使用场景选择对应分组；创建 Key 后，该 Key 按所选分组开放模型并按该分组倍率计费。

## 2. 查询可用模型

```bash
curl https://api.laoshirenai.com/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

从返回结果中复制一个模型 ID。不要猜模型名。

## 3. 发起请求

### OpenAI Responses

```bash
curl https://api.laoshirenai.com/v1/responses \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_MODEL_ID","input":"只回复 OK"}'
```

### Anthropic Messages

```bash
curl https://api.laoshirenai.com/v1/messages \
  -H "x-api-key: YOUR_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_MODEL_ID","max_tokens":32,"messages":[{"role":"user","content":"只回复 OK"}]}'
```

### Gemini

```bash
curl "https://api.laoshirenai.com/v1beta/models/YOUR_MODEL_ID:generateContent" \
  -H "x-goog-api-key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"contents":[{"parts":[{"text":"只回复 OK"}]}]}'
```

### Grok / xAI

Grok 分组使用 OpenAI Responses：

```bash
curl https://api.laoshirenai.com/v1/responses \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"YOUR_GROK_MODEL_ID","input":"只回复 OK"}'
```

返回完整回复即表示基础 API 接入成功。配置开发终端请进入[工具集成](integration-claude-code)。

## 安全

- 不要把真实 Key 写入仓库、截图或聊天记录。
- 不要把 Key 放在 URL 查询参数中。
- Key 泄露后立即停用并重新创建。
