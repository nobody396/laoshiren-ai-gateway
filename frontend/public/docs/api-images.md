# Images

> 验证状态：2026-08-28已通过Linux生产源服务器到老实人AI公网网关的文生图E2E；Windows PowerShell和图片编辑仍待验收。

## 当前接入合同

```text
分组：GPT Image 2 生图分组
模型：gpt-image-2
Base URL：https://api.laoshirenai.com/v1
接口：POST /images/generations
鉴权：Authorization: Bearer YOUR_API_KEY
```

## 创建生图Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 **GPT Image 2 生图分组** 创建Key。

不要使用Claude、Codex或其他文本分组Key调用图片接口。

## 查询模型

```bash
curl https://api.laoshirenai.com/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

返回结果应包含：

```text
gpt-image-2
```

## 文生图

下面是当前生图线路已验证的最小请求形状：

```bash
curl https://api.laoshirenai.com/v1/images/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只白色陶瓷杯放在纯色背景上，简洁产品摄影",
    "size": "1024x1024",
    "quality": "low",
    "n": 1
  }'
```

**不要传 `response_format`。** 当前生图线路由网关自动处理响应格式，上游会返回Base64图片。

## 已验证参数

| 参数 | 值 | 说明 |
| --- | --- | --- |
| `model` | `gpt-image-2` | 固定图片模型 |
| `prompt` | 字符串 | 图片内容、构图、风格和限制条件 |
| `size` | `1024x1024` | 当前最小探测已通过 |
| `quality` | `low` | 当前最小探测已通过 |
| `n` | `1` | 当前按单张生成验收 |

其他尺寸、质量、高级参数和图片编辑必须逐项实测后才能加入正式文档。

## 成功响应

当前生图线路实测返回JSON，`data[0]`包含 `b64_json`：

```json
{
  "created": 1780000000,
  "data": [
    {
      "b64_json": "iVBORw0KGgoAAAANSUhEUg..."
    }
  ],
  "size": "1024x1024",
  "quality": "low",
  "output_format": "png",
  "usage": {}
}
```

成功标准：

1. HTTP 200。
2. `Content-Type`为 `application/json`。
3. `data[0].b64_json`存在且可以解码为图片。
4. `size`和 `quality`与请求一致。
5. Usage可以被网关记录并正确计费。

## 保存Base64图片

### Python

```python
import base64
from pathlib import Path

image_base64 = response_json["data"][0]["b64_json"]
Path("result.png").write_bytes(base64.b64decode(image_base64))
```

### Node.js

```javascript
import { writeFile } from "node:fs/promises";

const imageBase64 = responseJson.data[0].b64_json;
await writeFile("result.png", Buffer.from(imageBase64, "base64"));
```

## 计费

- 当前生图分组采用图片Token计费。
- 价格以[模型目录](models)和下单时的分组页面为准。
- 只有取得有效图片并完成Usage记录后，才能认定调用成功并结算。
- 失败、空图片和不可解码结果不能计为成功图片。

本次owned E2E生成1张1K图片，数据库只产生1条成功Usage；余额减少值与 `actual_cost` 精确一致。

## 图片编辑

```text
POST /v1/images/edits
```

当前公网网关尚不能解析图片编辑使用的 `multipart/form-data`，会在转发前返回：

```text
400 invalid_request_error: failed to parse request body
```

因此图片编辑目前不对客户开放，本页不提供不可执行的编辑命令。

## 图片编辑参数实测矩阵

| 场景 | 上游直接测试 | 公网网关 | 当前结论 |
| --- | --- | --- | --- |
| 单图编辑 | HTTP 200，12.054秒，有效PNG | HTTP 400，无法解析multipart | 暂不开放 |
| 遮罩编辑 | HTTP 200，41.626秒，有效PNG | 尚未转发 | 暂不开放 |
| 多图融合 | 240秒读取超时 | 尚未转发 | 不支持 |
| `1536x1024` + `quality=medium` | HTTP 200，51.658秒 | 尚未转发 | 待网关实现后复测 |
| `1024x1536` + `quality=high` | HTTP 200，95.992秒 | 尚未转发 | 待网关实现后复测 |
| `n=2` | HTTP 200但只返回1张图片 | 尚未转发 | 客户合同固定 `n=1` |

上游直接成功不等于客户可用。只有公网网关请求、图片结果、Usage和扣费全部通过后，参数才会进入正式命令。

## 已验证边界

- 生图Key使用 `gpt-image-2` 调用Images接口：HTTP 200并返回有效PNG。
- Images接口传文本模型 `gpt-5.6-sol`：HTTP 400，未选择上游账号，未产生Usage。
- 生图Key调用文本Responses模型：HTTP 400，未选择上游账号，未产生Usage。

## 常见错误

- `400 images endpoint requires an image model`：错误使用了文本模型名。
- `400 invalid size/quality`：尺寸或质量未通过目标线路验证。
- `400 response_format`：当前生图线路不要传 `response_format`。
- `401`：Key无效、停用或缺失。
- `403`：当前Key不是GPT Image 2生图分组。
- `413`：请求体过大。
- `429`：并发或请求频率达到限制。
- `500`–`504`：上游生成失败或超时；确认没有成功图片后再重试。

## 剩余验收

1. Windows PowerShell命令真实执行。
2. macOS终端命令真实执行。
3. 网关增加OpenAI图片编辑multipart解析和转发。
4. 单图、遮罩、横竖尺寸通过公网网关E2E后，再补充编辑命令。
5. 文本分组关闭生图能力后，再执行最终交叉边界回归。
