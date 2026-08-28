# OpenAI Responses

## 接入信息

```text
Base URL: https://api.laoshirenai.com/v1
Endpoint: POST /responses
Authorization: Bearer YOUR_API_KEY
```

模型 ID 必须来自当前 Key 的 [`GET /v1/models`](api-models) 返回结果。

## 最小请求

```bash
curl https://api.laoshirenai.com/v1/responses \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "input": "只回复 OK"
  }'
```

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

结构化输出、工具调用和推理参数是否可用取决于模型。客户端必须校验实际返回，不能只检查请求是否被接受。

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

## 验证

HTTP 200、`status: "completed"`、正文完整且Usage可解析才算成功。只有响应头、空正文或中途断流都不算成功。

## 能力边界

以下能力需要目标模型和上游原生支持，不能默认所有分组都可用：

- `previous_response_id`
- 服务端工具，例如托管搜索
- 加密reasoning跨轮回放
- 图片、文件等多模态输入

不支持时网关应明确返回错误，不会静默删除参数。

## 常见错误

- `400`：模型ID、输入项或模型不支持请求参数。
- `401`：Key无效、停用或缺失。
- `403`：当前Key所属分组没有该模型权限。
- `429`：并发或请求频率达到限制。
- `500`–`504`：记录北京时间、模型和完整错误后重试。
