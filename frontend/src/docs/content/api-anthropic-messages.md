# Anthropic Messages

## 请求

```bash
curl https://api.laoshirenai.com/v1/messages \
  -H "x-api-key: YOUR_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "max_tokens": 32,
    "messages": [{"role":"user","content":"只回复 OK"}]
  }'
```

## Base URL

Anthropic SDK 和 Claude Code 使用：

```text
https://api.laoshirenai.com
```

不要填写 `/v1`。客户端会自行请求 `/v1/messages`。

## 验证

HTTP 200、正文完整、`stop_reason` 正常结束才算成功。
