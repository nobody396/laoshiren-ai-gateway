# OpenAI Chat Completions

## 请求

```bash
curl https://api.laoshirenai.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "messages": [{"role":"user","content":"只回复 OK"}]
  }'
```

## 选择端点

- Codex 和需要 Responses 工具语义的 Agent 使用 [`/v1/responses`](api-responses)。
- 只支持 OpenAI Chat Completions 的客户端使用本端点。

不要把 Chat Completions 成功等同于 Responses、工具调用或图片能力成功。
