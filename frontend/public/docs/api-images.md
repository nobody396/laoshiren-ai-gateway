# Images

## 接入信息

| 用途 | Base URL | 说明 |
| --- | --- | --- |
| OpenAI 兼容图片接口 | `https://api.laoshirenai.com/v1` | 文生图和图片编辑 |
| GPT-Image 专用分组 | `https://api.laoshirenai.com/gpt-image/v1` | 异步任务接口 |

先在 [API 密钥](https://laoshirenai.com/keys) 创建支持图片生成的分组 Key。普通文本分组不一定开放图片能力。

## 查询图片模型

```bash
curl https://api.laoshirenai.com/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

请求必须使用当前 Key 返回的图片模型 ID。不要把文本模型名当成图片模型名。

## 文生图

```bash
curl https://api.laoshirenai.com/v1/images/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_IMAGE_MODEL_ID",
    "prompt": "一只坐在窗边的橘猫，柔和自然光，真实摄影质感",
    "size": "1024x1024",
    "quality": "high",
    "n": 1,
    "response_format": "b64_json"
  }'
```

## 常用参数

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `model` | 是 | 当前 Key 可用的图片模型 ID |
| `prompt` | 是 | 图片内容、构图、风格和限制条件 |
| `size` | 否 | 输出尺寸或比例，支持范围取决于模型 |
| `quality` | 否 | `low`、`medium`、`high` 或模型支持值 |
| `n` | 否 | 生成数量；没有明确支持时使用 `1` |
| `response_format` | 否 | 常用值为 `b64_json`；部分通道固定返回 URL 或二进制 |
| `background` | 否 | `auto`、`transparent` 或 `opaque`，取决于模型 |
| `output_format` | 否 | `png`、`jpeg` 或 `webp`，取决于模型 |

这些参数采用兼容转发。**接口接收参数不代表每个模型都支持该参数**，正式使用前必须用目标模型验证。

## 同步响应

接口可能返回 URL：

```json
{
  "data": [
    {"url": "https://example.com/generated/image.png"}
  ]
}
```

也可能返回 Base64：

```json
{
  "data": [
    {"b64_json": "iVBORw0KGgoAAAANSUhEUg..."}
  ]
}
```

部分通道直接返回 `image/png`、`image/jpeg` 或 `image/webp`。客户端应先检查响应头 `Content-Type`，再决定按 JSON 还是图片二进制处理。

## 图片编辑

图片编辑使用 `multipart/form-data`：

```bash
curl https://api.laoshirenai.com/v1/images/edits \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -F "model=YOUR_IMAGE_MODEL_ID" \
  -F "prompt=保留商品主体和包装文字不变，把背景改成浅灰色摄影棚" \
  -F "size=2048x2048" \
  -F "quality=high" \
  -F "n=1" \
  -F "image=@/path/to/reference.png"
```

多图融合可以重复传入 `image`：

```bash
-F "image=@/path/to/product.png" \
-F "image=@/path/to/background.png"
```

是否支持多图、遮罩和最大文件大小取决于目标模型与分组。上传前先压缩图片，避免请求体过大。

## GPT-Image 专用异步任务

GPT-Image 分组使用专用 Base URL：

```text
https://api.laoshirenai.com/gpt-image/v1
```

### 1. 提交任务

```bash
curl https://api.laoshirenai.com/gpt-image/v1/images/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只坐在窗边的橘猫",
    "n": 1,
    "size": "1:1",
    "resolution": "2k"
  }'
```

提交成功后读取 `data[0].task_id`。

### 2. 查询任务

```bash
curl https://api.laoshirenai.com/gpt-image/v1/tasks/TASK_ID \
  -H "Authorization: Bearer YOUR_API_KEY"
```

任务状态包括：

```text
submitted
processing
completed
failed
```

只有 `completed` 后才读取图片结果。任务失败时读取错误信息，不要无限轮询。

### 3. 获取图片

完成响应中的图片地址由老实人AI提供，格式类似：

```text
https://api.laoshirenai.com/gpt-image/media/TASK_ID/0?token=...
```

这是临时访问地址。业务系统取得图片后应及时保存到自己的存储。

## 计费

- 以模型目录和当前分组显示价格为准。
- 图片实际生成成功后才记录对应图片用量。
- GPT-Image异步任务在完成并取得图片后结算；失败任务不应产生成功图片用量。
- `size`、`resolution`、`quality` 和数量可能影响价格，不能只按请求次数估算。

## 生产处理建议

1. 为请求设置合理超时。
2. 异步任务按返回状态轮询，不固定假设生成时间。
3. URL、Base64和图片二进制三种响应都要兼容。
4. 下载成功后保存图片，不长期依赖临时URL。
5. 重试前先确认前一次任务是否已成功，避免重复生成和重复扣费。

## 常见错误

- `400 invalid model`：模型 ID 不属于当前 Key。
- `400 invalid size/quality`：尺寸或质量档位不受目标模型支持。
- `401`：Key 无效、停用或缺失。
- `403`：当前 Key 所属分组未开放图片能力。
- `413`：上传图片或 Base64 请求体过大。
- `429`：并发或频率达到限制。
- `500`–`504`：上游生成失败或超时；先查询任务状态，再决定是否重试。

## 验证

只有取得可解码的图片 URL、Base64或图片二进制才算成功。HTTP 200、202或任务已提交本身都不等于图片已经生成完成。
