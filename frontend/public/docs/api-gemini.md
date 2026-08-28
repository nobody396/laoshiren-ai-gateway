# Gemini

## 请求

```bash
curl "https://api.laoshirenai.com/v1beta/models/YOUR_MODEL_ID:generateContent" \
  -H "x-goog-api-key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"parts":[{"text":"只回复 OK"}]}]
  }'
```

流式请求使用：

```text
POST /v1beta/models/{model}:streamGenerateContent?alt=sse
```

模型 ID 必须来自当前 Gemini Key 的模型列表。不要使用 OpenAI 分组的 Key。
