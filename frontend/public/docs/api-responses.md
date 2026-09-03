# OpenAI Responses

## cURL 请求

```bash
curl https://api.laoshirenai.com/v1/responses \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "input": "只回复 OK"
  }'
```

将 `YOUR_API_KEY` 替换为站内创建的 Key；将 `YOUR_MODEL_ID` 替换为该 Key 的 [`GET /v1/models`](api-models) 返回值。

## 常用参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `model` | 是 | 当前 Key 可用的模型 ID |
| `input` | 是 | 字符串或输入项数组 |
| `instructions` | 否 | 全局指令 |
| `stream` | 否 | `true` 时返回 Responses SSE 事件 |
| `tools` | 否 | Function工具或模型支持的原生工具 |
| `tool_choice` | 否 | 自动或指定工具 |
| `reasoning` | 否 | 推理强度等参数，取决于模型 |
| `max_output_tokens` | 否 | 最大输出Token数 |

## 消息数组

```json
{
  "model": "YOUR_MODEL_ID",
  "instructions": "回答要简洁",
  "input": [
    {
      "role": "user",
      "content": [
        {"type": "input_text", "text": "解释什么是 API 网关"}
      ]
    }
  ]
}
```

## 流式响应

```bash
curl -N https://api.laoshirenai.com/v1/responses \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "input": "用三句话介绍 API 网关",
    "stream": true
  }'
```

常见事件：

```text
response.created
response.output_text.delta
response.output_item.added
response.function_call_arguments.delta
response.completed
response.failed
```

客户端必须按事件类型解析，直到 `response.completed` 或 `response.failed`。不要用Chat Completions的结束格式替代Responses事件状态。

## 工具调用

```json
{
  "model": "YOUR_MODEL_ID",
  "input": "查询北京现在几点，请使用工具",
  "tools": [
    {
      "type": "function",
      "name": "get_time",
      "description": "读取指定时区的当前时间",
      "parameters": {
        "type": "object",
        "properties": {
          "utc_offset": {"type": "string"}
        },
        "required": ["utc_offset"],
        "additionalProperties": false
      }
    }
  ]
}
```

成功时在 `output` 数组中读取 `function_call` 项。应用执行函数后，把对应结果作为下一轮输入发送。

## 结构化输出

```json
{
  "text": {
    "format": {
      "type": "json_schema",
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

GPT 5.6 Sol 已支持上述 JSON Schema 结构化输出。

## 成功响应

```json
{
  "status": "completed",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [
        {"type": "output_text", "text": "OK"}
      ]
    }
  ],
  "usage": {
    "input_tokens": 12,
    "output_tokens": 1,
    "total_tokens": 13
  }
}
```

## 成功响应

成功时返回 HTTP 200、`status: "completed"`、完整正文和可解析的 Usage；空正文或中途断流不是完整响应。

## 支持能力

GPT 5.6 Sol 已支持：

- Function 工具调用；
- JSON Schema 结构化输出；
- `reasoning.effort`；
- `web_search` 服务端工具；
- `input_image` 图片输入；
- 流式 Responses SSE 与完整 Usage。

HTTP `/v1/responses` 当前不接受 `previous_response_id`，会返回 `400 previous_response_id is only supported on Responses WebSocket v2`。HTTP 多轮请求请发送完整上下文；需要 `previous_response_id` 时使用 Responses WebSocket v2。

## Chat → Responses 兼容桥边界

部分只提供 Chat Completions 的上游可以由网关转换成 Responses 外形。当前桥接已覆盖普通文本、SSE、Function Calling 和基础结构化输出，但它不是原生 Responses 的等价替代：

- `web_search`、`image_generation` 等服务端工具在 Chat 上游没有对应能力，会被拒绝或无法执行；
- `previous_response_id` 等 Responses 状态续接不能由无状态 Chat 请求完整还原；
- Responses `custom` 工具的自由文本输入与 grammar 约束只能降级成普通 Function 参数；
- namespace 工具名在 Chat 中需要摊平，发生同名歧义时网关会返回 400，避免调用错误工具；
- Chat 没有对应类型的 Responses item 和事件不能凭空生成。

因此 GLM 5.3、Kimi、MiniMax 等原生 Chat 模型应直接使用 Chat Completions。只有模型与上游均完成原生 Responses 验收时，模型卡才会标记 Responses。

## 常见错误

- `400`：模型ID、输入项或模型不支持请求参数。
- `401`：Key无效、停用或缺失。
- `403`：当前Key所属分组没有该模型权限。
- `429`：并发或请求频率达到限制。
- `500`–`504`：记录北京时间、模型和完整错误后重试。
