# Anthropic Messages

## cURL 请求

```bash
curl https://api.laoshirenai.com/v1/messages \
  -H "x-api-key: YOUR_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "max_tokens": 64,
    "messages": [
      {"role": "user", "content": "只回复 OK"}
    ]
  }'
```

`anthropic-version: 2023-06-01` 是 Messages API 的协议版本，不是模型年份。Anthropic 当前官方版本说明仍使用该值；Anthropic SDK 和 Claude Code 会自动发送该请求头并追加 `/v1/messages`，Base URL 填 `https://api.laoshirenai.com`。

## 常用参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `model` | 是 | 当前Key可用的Anthropic兼容模型ID |
| `messages` | 是 | `user`和`assistant`消息数组 |
| `max_tokens` | 是 | 最大输出Token数 |
| `system` | 否 | 系统提示词，字符串或内容块数组 |
| `stream` | 否 | `true`时返回Anthropic SSE事件 |
| `tools` | 否 | 工具定义 |
| `tool_choice` | 否 | 自动、任意或指定工具 |
| `thinking` | 否 | 推理配置，取决于模型 |
| `temperature` | 否 | 采样温度；部分推理模型会忽略或拒绝 |

## 内容块

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "解释什么是 API 网关"}
  ]
}
```

Claude Sonnet 5 已支持文本、Base64 图片输入、工具调用和扩展思考内容块。

## 流式响应

```bash
curl -N https://api.laoshirenai.com/v1/messages \
  -H "x-api-key: YOUR_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_MODEL_ID",
    "max_tokens": 256,
    "stream": true,
    "messages": [{"role":"user","content":"用三句话介绍 API 网关"}]
  }'
```

常见事件顺序：

```text
message_start
content_block_start
content_block_delta
content_block_stop
message_delta
message_stop
```

客户端必须等到 `message_stop`。收到HTTP 200但缺少终止事件，属于不完整响应。

## 工具调用

```json
{
  "model": "YOUR_MODEL_ID",
  "max_tokens": 256,
  "messages": [
    {"role": "user", "content": "查询北京现在几点，请使用工具"}
  ],
  "tools": [
    {
      "name": "get_time",
      "description": "读取指定时区的当前时间",
      "input_schema": {
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

成功时响应 `content` 中出现 `tool_use`。应用执行函数后，使用匹配的 `tool_result` 发送下一轮消息。

## 成功响应

```json
{
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "text", "text": "OK"}
  ],
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 12,
    "output_tokens": 1
  }
}
```

成功时返回 HTTP 200、完整正文、正常的 `stop_reason` 和可解析的 Usage。

## 常见错误

- `400`：模型ID、内容块、工具Schema或推理参数错误。
- `401`：Key无效、停用或缺失。
- `403`：当前Key所属分组没有模型权限。
- `429`：并发或请求频率达到限制。
- `500`–`504`：服务暂时不可用，可以稍后重试。
