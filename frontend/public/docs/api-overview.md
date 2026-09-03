# API 概览

## Base URL

| 协议 | Base URL |
| --- | --- |
| OpenAI | `https://api.laoshirenai.com/v1` |
| Anthropic | `https://api.laoshirenai.com` |
| Gemini | `https://api.laoshirenai.com` |

客户端会自动追加路径时，不要重复添加 `/v1` 或 `/v1beta`。

## Key与分组

同一个模型可能由多个分组提供。创建Key时请根据预算选择分组；Key创建后，只能调用该分组开放的模型，并按该分组倍率计费。

查询模型时必须使用准备调用的那把Key，不能用其他分组的模型列表代替。

客户端选择的是**请求协议**，模型来自当前 Key 的分组。只要客户端协议、请求模型和 Key 返回的模型列表匹配，同一种协议可以调用不同厂商的模型。例如 Responses 和 Chat Completions 已实际接通 GPT、Grok、GLM、Kimi、MiniMax 与 Qwen；不是只有 GPT 才能使用 OpenAI 兼容协议。

## 鉴权

OpenAI：

```http
Authorization: Bearer YOUR_API_KEY
```

Anthropic：

```http
x-api-key: YOUR_API_KEY
anthropic-version: 2023-06-01
```

`anthropic-version` 表示 **Anthropic Messages API 的协议版本**，不是模型发布日期。Anthropic 当前官方文档仍要求该请求头，并继续使用 `2023-06-01`；使用 Anthropic SDK 时由 SDK 自动发送。参见 [Anthropic API 版本说明](https://platform.claude.com/docs/en/api/versioning)。

Gemini：

```http
x-goog-api-key: YOUR_API_KEY
```

## 端点

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/v1/models` | 查询当前 Key 可用模型 |
| `POST` | `/v1/responses` | OpenAI Responses |
| `POST` | `/v1/chat/completions` | OpenAI Chat Completions |
| `POST` | `/v1/messages` | Anthropic Messages |
| `POST` | `/v1beta/models/{model}:generateContent` | Gemini |
| `POST` | `/v1/images/generations` | 图片生成 |
| `POST` | `/v1/images/edits` | 图片编辑 |

## 当前协议覆盖

| 协议 | 已接通模型 |
| --- | --- |
| Responses | GPT 5.3–5.6、Grok 4.5/4.6、GLM 5.2/5.3、Kimi K2.7 Code/K3、MiniMax M3、Qwen 3.6/3.7/3.8 |
| Chat Completions | GPT、Grok、GLM、Kimi、MiniMax、Qwen |
| Messages | Claude Fable 5、Haiku 4.5、Sonnet 4.5/4.6/5、Opus 4.6/4.7/4.8/5 |
| GenerateContent | Gemini 3.1 Pro、Gemini 3.7 Flash / Flash High |
| Images | `gpt-image-2` |

## 错误响应

OpenAI 兼容接口在模型不存在时返回：

```json
{
  "error": {
    "type": "invalid_request_error",
    "code": "model_not_supported",
    "message": "The model is not supported."
  }
}
```

Anthropic Messages 在模型不存在时返回：

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "The model is not supported"
  }
}
```

Gemini 在模型不存在或当前没有可用账号时返回 Google 兼容错误对象和对应 HTTP 状态码。

## 限流与重试

- `429`后根据返回提示等待，不要立即高并发重试。
- `500`–`504`可以有限重试，并使用指数退避。
- 非幂等任务，例如图片生成，重试前先确认前一次是否已经成功。
- `400`、`401`、`403`通常需要修正请求或权限，不应自动重复发送。

## Request ID与使用记录

- 保存响应头 `X-Request-ID`，同时记录北京时间、客户端、端点、模型、HTTP状态码和耗时。
- 调用后在“使用记录”核对实际模型、分组、状态、Token和费用。
- 超时或连接中断不等于请求一定失败。先按时间和Request ID查询使用记录，再决定是否重试。

## 常见状态码

| 状态码 | 含义 |
| --- | --- |
| `400` | 模型、协议或请求参数不匹配 |
| `401` | Key 无效、停用或缺失 |
| `403` | 当前 Key 或分组无权访问 |
| `429` | 并发或频率达到限制 |
| `500`–`504` | 网关或上游暂时失败 |
