# API 概览

## Base URL

| 协议 | Base URL |
| --- | --- |
| OpenAI | `https://api.laoshirenai.com/v1` |
| Anthropic | `https://api.laoshirenai.com` |
| Gemini | `https://api.laoshirenai.com` |
| Antigravity | `https://api.laoshirenai.com/antigravity` |

客户端会自动追加路径时，不要重复添加 `/v1` 或 `/v1beta`。

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

## 常见状态码

| 状态码 | 含义 |
| --- | --- |
| `400` | 模型、协议或请求参数不匹配 |
| `401` | Key 无效、停用或缺失 |
| `403` | 当前 Key 或方案无权访问 |
| `429` | 并发或频率达到限制 |
| `500`–`504` | 网关或上游暂时失败 |

失败时记录北京时间、客户端、模型和完整错误，再联系支持。不要发送 API Key。
