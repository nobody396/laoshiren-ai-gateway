# OpenAI Chat Completions

## cURL 请求

```bash
curl https://api.laoshirenai.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "messages": [
      {"role": "user", "content": "只回复 OK"}
    ]
  }'
```

将 `YOUR_API_KEY` 替换为站内创建的 Key；将 `YOUR_MODEL_ID` 替换为该 Key 的 [`GET /v1/models`](api-models) 返回值。

## 常用参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `model` | 是 | 当前 Key 可用的模型 ID |
| `messages` | 是 | 对话消息数组 |
| `stream` | 否 | `true` 时返回 SSE 流 |
| `tools` | 否 | Function Calling 工具定义 |
| `tool_choice` | 否 | `auto`、`none` 或指定工具 |
| `response_format` | 否 | JSON 对象或 JSON Schema；是否生效取决于模型 |
| `temperature` | 否 | 采样温度；部分推理模型会忽略或拒绝 |
| `max_tokens` | 否 | 最大输出 Token 数 |

## 流式响应

使用 `curl -N` 避免本地缓冲：

```bash
curl -N https://api.laoshirenai.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "messages": [{"role":"user","content":"用三句话介绍 API 网关"}],
    "stream": true,
    "stream_options": {"include_usage": true}
  }'
```

客户端应逐条解析 `data:` 事件，并在 `[DONE]` 后结束。

## 工具调用

下面的请求只让模型返回工具参数，不会执行真实函数：

```bash
curl https://api.laoshirenai.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "messages": [{"role":"user","content":"查询北京现在几点，请使用工具"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_time",
        "description": "读取指定时区的当前时间",
        "parameters": {
          "type": "object",
          "properties": {"utc_offset": {"type": "string"}},
          "required": ["utc_offset"],
          "additionalProperties": false
        }
      }
    }],
    "tool_choice": "auto"
  }'
```

成功时读取 `choices[0].message.tool_calls`。应用负责执行函数，并把结果作为 `role: "tool"` 的下一条消息发送。

## 结构化输出

```json
{
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "name": "city",
      "strict": true,
      "schema": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "country": {"type": "string"}
        },
        "required": ["name", "country"],
        "additionalProperties": false
      }
    }
  }
}
```

GPT 5.6 Sol 已支持上述严格 JSON Schema，并返回可直接解析的 JSON。

## 成功响应

```json
{
  "choices": [
    {
      "message": {"role": "assistant", "content": "OK"},
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 1,
    "total_tokens": 13
  }
}
```

成功的普通请求返回 HTTP 200、完整 `choices` 和可解析的 Usage；流式请求以 `[DONE]` 收尾。

## 常见错误

- `400`：模型 ID、消息格式或模型不支持请求参数。
- `401`：Key 无效、停用或缺失。
- `403`：当前 Key 所属分组没有该模型权限。
- `429`：并发或请求频率达到限制。
- `500`–`504`：服务暂时不可用，可以稍后重试。
