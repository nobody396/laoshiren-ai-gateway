# Gemini

## cURL 请求

```bash
curl "https://api.laoshirenai.com/v1beta/models/YOUR_MODEL_ID:generateContent" \
  -H "x-goog-api-key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"role":"user","parts":[{"text":"只回复 OK"}]}]
  }'
```

Gemini SDK 和 Gemini CLI 会自动追加 `/v1beta/models/...`，Base URL 填 `https://api.laoshirenai.com`。

## 查询模型

```bash
curl https://api.laoshirenai.com/v1beta/models \
  -H "x-goog-api-key: YOUR_API_KEY"
```

不要使用OpenAI或Anthropic分组的Key。

## 常用参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `contents` | 是 | 对话内容，包含 `role` 和 `parts` |
| `systemInstruction` | 否 | 系统指令 |
| `generationConfig` | 否 | 温度、最大输出、停止词等生成设置 |
| `tools` | 否 | Function Calling工具定义 |
| `toolConfig` | 否 | 工具调用模式 |
| `safetySettings` | 否 | 安全策略；是否支持取决于上游 |

## 生成设置

```json
{
  "generationConfig": {
    "temperature": 0.2,
    "maxOutputTokens": 512,
    "responseMimeType": "application/json"
  }
}
```

Gemini 3.1 Pro 已支持 JSON Schema 结构化输出，并返回 `application/json` 文本。

## 流式响应

```bash
curl -N "https://api.laoshirenai.com/v1beta/models/YOUR_MODEL_ID:streamGenerateContent?alt=sse" \
  -H "x-goog-api-key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [
      {"role":"user","parts":[{"text":"用三句话介绍 API 网关"}]}
    ]
  }'
```

客户端逐条读取SSE中的 `candidates`，直到出现完成原因。不要把单个增量当成完整回复。

## 工具调用

```json
{
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "get_time",
          "description": "读取指定时区的当前时间",
          "parameters": {
            "type": "OBJECT",
            "properties": {
              "utc_offset": {"type": "STRING"}
            },
            "required": ["utc_offset"]
          }
        }
      ]
    }
  ]
}
```

Gemini 3.1 Pro 已支持 Function Calling；成功时在候选内容中读取 `functionCall`，应用执行函数后再发送函数响应。该模型也支持 `inlineData` Base64 图片输入。

## 成功响应

```json
{
  "candidates": [
    {
      "content": {
        "role": "model",
        "parts": [{"text": "OK"}]
      },
      "finishReason": "STOP"
    }
  ],
  "usageMetadata": {
    "promptTokenCount": 12,
    "candidatesTokenCount": 1,
    "totalTokenCount": 13
  }
}
```

成功时返回 HTTP 200、完整文本、正常的 `finishReason` 和可解析的 `usageMetadata`。

## 常见错误

- `400`：模型ID、`contents`、工具Schema或生成参数错误。
- `401`：Key无效、停用或缺失。
- `403`：当前Key不是Gemini分组或没有模型权限。
- `429`：并发或请求频率达到限制。
- `500`–`504`：服务暂时不可用，可以稍后重试。
