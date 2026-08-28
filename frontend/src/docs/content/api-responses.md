# OpenAI Responses

## 请求

```bash
curl https://api.laoshirenai.com/v1/responses \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "input": "只回复 OK"
  }'
```

模型 ID 必须来自当前 Key 的 [`GET /v1/models`](api-models) 返回结果。

## 流式响应

在请求体增加：

```json
{"stream": true}
```

客户端必须持续读取 SSE，直到完成事件结束。

## 验证

HTTP 200 且收到完整文本才算成功。只有响应头、空正文或中途断流都不算成功。
