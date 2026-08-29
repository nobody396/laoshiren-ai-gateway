# API 参考

本文面向需要直接调用老实人AI API 的开发者，集中说明鉴权、Base URL、模型发现、Responses、Chat Completions、Anthropic Messages、Gemini GenerateContent、流式响应、错误处理和图片接口。

如果你只是第一次配置 Codex、Claude Code 或 Cherry Studio，请先阅读对应的快速开始；如果你正在开发服务端、Agent 或内部网关，再使用本文逐项核对协议和参数。

---

## 1. 接入总览

### 1.1 控制台与 API 地址

| 用途 | 地址 | 说明 |
|---|---|---|
| 网站与控制台 | `https://laoshirenai.com` | 注册、充值、创建 API Key、选择分组、查看用量 |
| OpenAI 兼容 API | `https://api.laoshirenai.com/v1` | Models、Responses、Chat Completions |
| Codex Responses 根地址 | `https://api.laoshirenai.com` | Codex 会自行追加 `/responses` 等路径 |
| Anthropic Messages 根地址 | `https://api.laoshirenai.com` | Anthropic SDK 和 Claude Code 会追加 `/v1/messages` |
| Gemini 原生兼容根地址 | `https://api.laoshirenai.com` | Gemini SDK 会追加 `/v1beta/models/...` |
| GPT-Image 专用 API | `https://api.laoshirenai.com/gpt-image/v1` | 独立生图分组、生成任务和结果查询 |

> Base URL 是否包含 `/v1` 取决于客户端会不会自行拼接路径。不要组成 `/v1/v1/responses` 或 `/v1/v1/messages`。

### 1.2 支持的主要协议

| 协议 | 入口 | 典型客户端 |
|---|---|---|
| OpenAI Responses | `POST /v1/responses` | Codex、自研 Agent、OpenAI Responses SDK |
| Responses WebSocket | `GET /v1/responses` 并升级 WebSocket | 支持 Responses WebSocket 的 Codex 客户端 |
| OpenAI Chat Completions | `POST /v1/chat/completions` | Cherry Studio、Open WebUI、传统 OpenAI SDK |
| Anthropic Messages | `POST /v1/messages` | Claude Code、Anthropic SDK |
| Gemini GenerateContent | `POST /v1beta/models/{model}:generateContent` | Gemini SDK、Gemini CLI |
| GPT-Image | `POST /gpt-image/v1/images/generations` | OpenAI Images 兼容客户端、自研生图服务 |

同一个模型不一定对所有协议都可用。API Key 的分组、模型映射、上游能力和请求协议必须同时匹配。

---

## 2. 鉴权与请求头

### 2.1 Bearer Token

OpenAI、Responses、Grok、Gemini兼容入口通常使用：

```http
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json
```

### 2.2 Anthropic兼容请求

Claude Code 和 Anthropic SDK 常用：

```http
x-api-key: YOUR_API_KEY
anthropic-version: 2023-06-01
Content-Type: application/json
```

部分 Anthropic SDK 也会发送 Bearer Token。请优先使用客户端官方的 `api_key` 或 `ANTHROPIC_AUTH_TOKEN` 配置，不要同时写入多个不同 Key。

### 2.3 请求追踪

客户端可以发送：

```http
X-Request-ID: your-unique-request-id
```

建议每次请求生成唯一 ID，并记录请求时间、模型、HTTP 状态码和响应头中的请求标识。向客服反馈时不要发送完整 API Key、提示词或私有文件。

---

## 3. 获取当前模型列表

模型和分组会变化，正式调用前应使用目标 API Key 读取模型列表：

```bash
curl 'https://api.laoshirenai.com/v1/models' \
  -H 'Authorization: Bearer YOUR_API_KEY'
```

响应示例：

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-5.6-sol",
      "object": "model",
      "owned_by": "laoshirenai"
    }
  ]
}
```

只使用列表中实际返回的模型 ID。网页价格页用于了解产品，`GET /v1/models` 才表示当前 Key 可以发现的模型。

---

## 4. OpenAI Responses API

### 4.1 请求地址

```http
POST /v1/responses
```

Codex 使用根地址时也可能请求：

```http
POST /responses
```

两种入口进入同一套鉴权和分组路由。普通 SDK 推荐使用带 `/v1` 的标准地址；Codex 按站内一键配置使用根地址。

### 4.2 基础请求

```bash
curl 'https://api.laoshirenai.com/v1/responses' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-5.6-sol",
    "input": "只回复 OK",
    "max_output_tokens": 64,
    "stream": false
  }'
```

### 4.3 常用参数

| 参数 | 类型 | 必填 | 说明 |
|---|---|---:|---|
| `model` | string | 是 | 从 `GET /v1/models` 获取的模型 ID |
| `input` | string / array | 是 | 字符串，或包含 role/content 的输入项数组 |
| `instructions` | string | 否 | 本轮系统级指令；不要放 API Key 等秘密 |
| `stream` | boolean | 否 | `true` 返回 SSE；Codex 和长响应建议开启 |
| `max_output_tokens` | integer | 否 | 输出 Token 上限；过小可能导致响应截断 |
| `tools` | array | 否 | 函数工具或模型支持的服务端工具 |
| `tool_choice` | string / object | 否 | `auto`、`required` 或指定工具；是否支持取决于模型和上游 |
| `parallel_tool_calls` | boolean | 否 | 是否允许并行函数调用 |
| `previous_response_id` | string | 否 | 续接上一轮响应；切换模型或线路时可能不兼容 |
| `prompt_cache_key` | string | 否 | 会话和缓存路由提示；应稳定且不包含敏感信息 |
| `reasoning` | object | 否 | 推理配置；可用字段取决于模型 |
| `text` | object | 否 | 文本输出与结构化输出配置 |
| `temperature` | number | 否 | 采样温度；部分推理模型忽略或拒绝该参数 |
| `top_p` | number | 否 | 核采样；通常不要与 temperature 同时大幅调整 |
| `metadata` | object | 否 | 客户端元数据；不要包含个人敏感信息 |

网关会兼容和透传常用 Responses 字段，但最终能力由目标模型、Key 分组和上游协议共同决定。生产接入前应使用真实目标模型验证工具调用、结构化输出、长上下文和 usage。

### 4.4 流式响应

```bash
curl --no-buffer 'https://api.laoshirenai.com/v1/responses' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Accept: text/event-stream' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-5.6-sol",
    "input": "用三句话解释缓存",
    "stream": true
  }'
```

常见 SSE 事件：

| 事件 | 含义 |
|---|---|
| `response.created` | 响应已创建 |
| `response.output_text.delta` | 文本增量 |
| `response.function_call_arguments.delta` | 工具参数增量 |
| `response.completed` | 完整成功；读取最终 usage |
| `response.failed` | 响应失败；不要把连接关闭误判成成功 |

客户端必须读到 `response.completed` 才能把请求视为完整成功。不要因为已收到几个文本片段就提前释放业务状态。

### 4.5 函数工具示例

```json
{
  "model": "gpt-5.6-sol",
  "input": "查询订单 A100 的状态",
  "tools": [
    {
      "type": "function",
      "name": "get_order",
      "description": "按订单号查询订单",
      "parameters": {
        "type": "object",
        "properties": {
          "order_id": { "type": "string" }
        },
        "required": ["order_id"],
        "additionalProperties": false
      },
      "strict": true
    }
  ],
  "tool_choice": "auto",
  "stream": true
}
```

模型返回函数调用后，客户端负责执行本地函数，再把工具结果作为下一轮 input 发送。服务端工具（例如部分模型的联网搜索）必须由目标上游原生支持，不能只凭模型名称判断。

### 4.6 结构化输出

```json
{
  "model": "gpt-5.6-sol",
  "input": "提取订单号和金额：订单 A100，金额 299 元",
  "text": {
    "format": {
      "type": "json_schema",
      "name": "order",
      "strict": true,
      "schema": {
        "type": "object",
        "properties": {
          "order_id": { "type": "string" },
          "amount": { "type": "number" }
        },
        "required": ["order_id", "amount"],
        "additionalProperties": false
      }
    }
  }
}
```

请在客户端再次进行 JSON Schema 校验。模型或兼容上游不支持严格结构化输出时，可能返回 400，不能自动降级后仍声称满足 schema。

### 4.7 Compact 与输入 Token 预检

网关提供以下 Responses 子路径：

```http
POST /v1/responses/compact
POST /v1/responses/input_tokens
```

`compact` 用于兼容支持响应压缩的客户端；`input_tokens` 用于估算 Responses 输入。并非所有模型或上游都支持 compact。不要自行构造未记录的 `/responses/*` 路径。

---

## 5. Chat Completions API

### 5.1 请求地址

```http
POST /v1/chat/completions
```

```bash
curl 'https://api.laoshirenai.com/v1/chat/completions' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-5.6-sol",
    "messages": [
      {"role": "system", "content": "回答要简洁"},
      {"role": "user", "content": "解释什么是 API 网关"}
    ],
    "stream": true,
    "max_tokens": 256
  }'
```

### 5.2 常用参数

| 参数 | 类型 | 必填 | 说明 |
|---|---|---:|---|
| `model` | string | 是 | 模型 ID |
| `messages` | array | 是 | `system`、`user`、`assistant`、`tool` 消息 |
| `stream` | boolean | 否 | 是否返回 SSE 增量 |
| `max_tokens` | integer | 否 | 兼容参数；部分客户端使用 `max_completion_tokens` |
| `temperature` | number | 否 | 部分推理模型不支持 |
| `top_p` | number | 否 | 核采样参数 |
| `tools` | array | 否 | Chat Completions 函数工具定义 |
| `tool_choice` | string / object | 否 | 自动、强制或指定函数 |
| `response_format` | object | 否 | JSON object 或 JSON Schema；取决于上游支持 |
| `stream_options` | object | 否 | 常用于请求流尾 usage，例如 `include_usage` |

Chat Completions 和 Responses 不是同一个请求格式。不要把 `input` 直接发给 `/chat/completions`，也不要把 `messages` 原样当成 Responses 请求体。

---

## 6. Anthropic Messages API

### 6.1 请求地址

```http
POST /v1/messages
```

```bash
curl 'https://api.laoshirenai.com/v1/messages' \
  -H 'x-api-key: YOUR_API_KEY' \
  -H 'anthropic-version: 2023-06-01' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "claude-sonnet-5",
    "max_tokens": 512,
    "system": "回答要简洁",
    "messages": [
      {"role": "user", "content": "解释什么是 Agent"}
    ],
    "stream": true
  }'
```

### 6.2 常用参数

| 参数 | 类型 | 必填 | 说明 |
|---|---|---:|---|
| `model` | string | 是 | Claude或当前分组公开的兼容模型 |
| `messages` | array | 是 | `user` 和 `assistant` 消息 |
| `max_tokens` | integer | 是 | 最大输出 Token |
| `system` | string / array | 否 | 系统提示词 |
| `stream` | boolean | 否 | 是否返回 Anthropic SSE 事件 |
| `tools` | array | 否 | Anthropic 工具定义 |
| `tool_choice` | object | 否 | 自动、任意或指定工具 |
| `thinking` | object | 否 | 扩展思考配置；预算必须满足模型要求 |
| `temperature` | number | 否 | 采样温度 |
| `top_p` / `top_k` | number | 否 | 采样参数 |
| `metadata` | object | 否 | 请求元数据，不要包含秘密 |

### 6.3 Token 计数

Claude或Grok兼容分组可使用：

```http
POST /v1/messages/count_tokens
```

OpenAI平台分组不提供 Anthropic Token Counting，调用时会返回不支持。

---

## 7. Gemini GenerateContent

### 7.1 请求地址

```http
POST /v1beta/models/{model}:generateContent
POST /v1beta/models/{model}:streamGenerateContent
```

```bash
curl 'https://api.laoshirenai.com/v1beta/models/gemini-3.1-pro:generateContent' \
  -H 'x-goog-api-key: YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [
      {
        "role": "user",
        "parts": [{"text": "只回复 OK"}]
      }
    ],
    "generationConfig": {
      "maxOutputTokens": 64,
      "temperature": 0.2
    }
  }'
```

常用字段包括 `contents`、`systemInstruction`、`generationConfig`、`tools`、`toolConfig` 和 `safetySettings`。具体可用性取决于模型和分组。

---

## 8. GPT-Image 专用接口

生产生图请创建 **GPT-Image 专用分组**的 API Key，并使用：

```text
https://api.laoshirenai.com/gpt-image/v1
```

文生图：

```http
POST /gpt-image/v1/images/generations
```

查询任务：

```http
GET /gpt-image/v1/tasks/{task_id}
```

模型列表：

```http
GET /gpt-image/v1/models
```

```bash
curl 'https://api.laoshirenai.com/gpt-image/v1/images/generations' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一张极简风格的产品海报",
    "n": 1,
    "size": "1:1",
    "resolution": "2k"
  }'
```

文本模型分组与生图分组应分开创建 Key。不要依赖聊天客户端把普通文本请求自动转换为 Images API。完整尺寸、图生图、任务轮询和结果格式参见《GPT-Image-2 使用指南》。

---

## 9. 用量与计费字段

Responses 常见 usage：

```json
{
  "usage": {
    "input_tokens": 1200,
    "output_tokens": 120,
    "total_tokens": 1320,
    "input_tokens_details": {
      "cached_tokens": 900
    }
  }
}
```

Anthropic Messages 常见 usage：

```json
{
  "usage": {
    "input_tokens": 300,
    "output_tokens": 80,
    "cache_creation_input_tokens": 0,
    "cache_read_input_tokens": 200
  }
}
```

账单以服务端最终记录为准。缓存命中、长上下文、Fast/Priority、图片和视频可能使用独立价格。客户端不要用 `total_tokens × 单一价格` 估算全部费用。

---

## 10. HTTP 状态码与重试

| 状态码 | 常见含义 | 建议 |
|---:|---|---|
| 200 | 请求成功 | 流式请求仍需等待完成事件 |
| 202 | 图片任务仍在处理 | 保存任务 ID，按返回间隔轮询 |
| 400 | 参数、协议、工具或模型能力不匹配 | 修改请求；不要原样无限重试 |
| 401 | API Key 无效、缺失或已停用 | 检查鉴权头和 Key |
| 402 | 余额、月额度或可用权益不足 | 查看余额和订阅 |
| 403 | 分组无权访问该模型或能力 | 检查 Key 分组和模型列表 |
| 404 | 路径、模型或任务不存在 | 检查 Base URL、路径和模型 ID |
| 408 / 504 | 请求或上游超时 | 查询是否已产生任务/账单，再决定重试 |
| 409 | 任务状态冲突或结果尚未就绪 | 重新查询状态 |
| 429 | RPM、TPM、并发或上游限流 | 读取 `Retry-After`，指数退避 |
| 500 / 502 / 503 | 网关或上游临时异常 | 对幂等请求有限重试，并保存请求 ID |
| 529 | 上游过载 | 降低并发，指数退避后重试 |

### 推荐重试策略

- 400、401、402、403、404：修正原因后再请求，不要自动重试。
- 408、429、500、502、503、504、529：指数退避并加入随机抖动。
- 流式连接中断：先确认是否已出现 `response.completed`、账单或图片任务 ID。
- 非幂等任务：不要无限重发 POST，否则可能重复生成和重复计费。
- 建议最多自动重试 1～2 次，再转人工或降级。

---

## 11. Python Responses 示例

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="https://api.laoshirenai.com/v1",
)

response = client.responses.create(
    model="gpt-5.6-sol",
    input="只回复 OK",
    max_output_tokens=64,
)

print(response.output_text)
```

不要把真实 API Key 写入源码。生产环境使用环境变量、Secret Manager 或受控配置中心。

---

## 12. JavaScript流式示例

```javascript
const response = await fetch("https://api.laoshirenai.com/v1/responses", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${process.env.LAOSHIRENAI_API_KEY}`,
    "Content-Type": "application/json",
    Accept: "text/event-stream"
  },
  body: JSON.stringify({
    model: "gpt-5.6-sol",
    input: "用三句话解释 API 网关",
    stream: true,
    max_output_tokens: 256
  })
});

if (!response.ok) {
  throw new Error(`HTTP ${response.status}: ${await response.text()}`);
}

const reader = response.body.getReader();
const decoder = new TextDecoder();
let buffer = "";

while (true) {
  const { value, done } = await reader.read();
  if (done) break;
  buffer += decoder.decode(value, { stream: true });
  // 生产代码应按空行拆分 SSE 帧，并解析 event/data 字段。
}
```

浏览器前端不应直接持有长期有效的服务器 API Key。面向终端用户的应用应由自己的后端调用老实人AI。

---

## 13. 上线前检查清单

1. 使用目标 Key 调用 `GET /v1/models`。
2. 用目标模型完成一次非流式和一次流式请求。
3. 检查完整结束事件和 usage。
4. 验证函数工具、服务端工具和结构化输出。
5. 使用接近真实业务的长上下文测试超时和缓存。
6. 验证 401、403、429、5xx 的处理路径。
7. 设置连接、首字和总耗时监控，至少记录 P50 与 P95。
8. 为非幂等请求设置幂等键或业务去重。
9. 不在日志、截图、前端代码和工单中泄漏 API Key。
10. 图片请求使用独立生图分组，并及时保存生成结果。

如果接入仍然失败，请提供请求时间、协议、Base URL、模型、HTTP 状态码和请求 ID；不要发送完整 API Key。
