# API 概览

## Base URL

| 协议 | Base URL |
| --- | --- |
| OpenAI | `https://api.laoshirenai.com/v1` |
| Anthropic | `https://api.laoshirenai.com` |
| Gemini | `https://api.laoshirenai.com` |
| Antigravity | `https://api.laoshirenai.com/antigravity` |

客户端会自动追加路径时，不要重复添加 `/v1` 或 `/v1beta`。

## Key与分组

同一个模型可能由多个分组提供。创建Key时请根据预算选择分组；Key创建后，只能调用该分组开放的模型，并按该分组倍率计费。

查询模型时必须使用准备调用的那把Key，不能用其他分组的模型列表代替。

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

## 能力需要分别验证

下面是不同能力，不能因为普通文本成功就默认全部可用：

- 流式输出
- Function/Tool Calling
- 结构化输出
- 推理内容
- 图片和文件输入
- 缓存读取与写入
- Usage完整返回

支持范围取决于模型、分组和上游协议。正式业务接入前，用目标Key和目标模型逐项测试。

## 错误响应

OpenAI兼容接口通常返回：

```json
{
  "error": {
    "type": "invalid_request_error",
    "message": "The model is not supported"
  }
}
```

Anthropic接口通常返回：

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "The model is not supported"
  }
}
```

Gemini接口使用Google兼容错误结构。客户端应同时记录HTTP状态码、错误类型、错误消息和请求时间。

## 限流与重试

- `429`后根据返回提示等待，不要立即高并发重试。
- `500`–`504`可以有限重试，并使用指数退避。
- 非幂等任务，例如图片生成，重试前先确认前一次是否已经成功。
- `400`、`401`、`403`通常需要修正请求或权限，不应自动重复发送。

## 常见状态码

| 状态码 | 含义 |
| --- | --- |
| `400` | 模型、协议或请求参数不匹配 |
| `401` | Key 无效、停用或缺失 |
| `403` | 当前 Key 或分组无权访问 |
| `429` | 并发或频率达到限制 |
| `500`–`504` | 网关或上游暂时失败 |

## 安全

- 不要把API Key写入仓库、截图、URL或聊天记录。
- 客户端日志不得打印完整Authorization Header。
- 只使用本地用户级配置，不把个人Key写入项目配置。

失败时记录北京时间、客户端、模型、端点和完整错误，再联系支持。不要发送API Key。
