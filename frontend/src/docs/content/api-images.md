# Images

> 已验证：图片生成、单图编辑、遮罩编辑和多图编辑均已通过老实人AI公网网关生产 E2E；Codex 也已通过自然语言自动调用生成与编辑能力。

## 接入信息

```text
分组：GPT Image 2 生图分组
模型：gpt-image-2
Base URL：https://api.laoshirenai.com/v1
生成：POST /images/generations
编辑：POST /images/edits
鉴权：Authorization: Bearer YOUR_API_KEY
```

图片接口使用独立的生图 Key。该分组只开放 `gpt-image-2`。

当前文本分组仍可能保留自身的原生图片能力；新接入请使用独立生图分组，避免把文本 Key 和图片 Key 混在一起。

## 1. 创建并检查 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 **GPT Image 2 生图分组** 创建 Key，然后检查模型权限：

```bash
curl https://api.laoshirenai.com/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

返回结果应包含 `gpt-image-2`。这一步只查询模型，不生成图片，不产生图片费用。

## 2. 图片生成

```bash
curl https://api.laoshirenai.com/v1/images/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只红色陶瓷杯，浅灰色背景，产品摄影，无文字",
    "size": "1024x1024",
    "quality": "low",
    "n": 1
  }'
```

不要传 `response_format`。成功响应的 `data[0].b64_json` 是 Base64 图片。

## 3. 图片编辑

安装 OpenAI Python SDK：

```bash
python3 -m pip install openai
```

下面三种编辑请求形状已经验证。所有示例均固定 `gpt-image-2`、`quality="low"` 和 `n=1`。

### 单图编辑

```python
from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="https://api.laoshirenai.com/v1",
)

with open("source.png", "rb") as source:
    result = client.images.edit(
        model="gpt-image-2",
        image=source,
        prompt="把背景改成纯白色，主体和构图保持不变，无文字",
        size="1024x1024",
        quality="low",
        n=1,
    )
```

### 遮罩编辑

遮罩必须是 PNG，尺寸与原图一致；透明区域表示需要修改的范围。

```python
with open("source.png", "rb") as source, open("mask.png", "rb") as mask:
    result = client.images.edit(
        model="gpt-image-2",
        image=source,
        mask=mask,
        prompt="只在遮罩区域加入一朵白色小花，其余部分保持不变",
        size="1024x1024",
        quality="low",
        n=1,
    )
```

### 多图编辑

把多张参考图作为列表传入；不要先把图片转成 URL 或 `file_id`。

```python
with open("product.png", "rb") as product, open("scene.png", "rb") as scene:
    result = client.images.edit(
        model="gpt-image-2",
        image=[product, scene],
        prompt="把第一张图的产品放入第二张图的场景，保持产品外观，无文字",
        size="1024x1024",
        quality="low",
        n=1,
    )
```

### 保存结果

生成和编辑使用相同的保存方式：

```python
import base64
from pathlib import Path

Path("result.png").write_bytes(base64.b64decode(result.data[0].b64_json))
```

## 4. 在 Codex 中自动生图

安装公开 Skill：

```text
请使用 $skill-installer 安装：
https://github.com/nobody396/laoshirenai-skills/tree/main/skills/laoshirenai-imagegen
```

安装后直接对 Codex 说：

```text
生成一张 1024×1024 的红色陶瓷杯产品图，低质量草稿，浅灰色背景，无文字。
```

首次调用时，Skill 会自动打开只监听 `127.0.0.1` 的本机配置页：

1. 粘贴 **GPT Image 2 生图分组** Key；
2. 点击 **保存并完成配置**；
3. Skill 自动保存 Key、准备运行环境，并通过 `/v1/models` 做零费用检查；
4. 检查通过后自动继续刚才的图片任务，不需要重新提问。

以后提出生成、编辑、遮罩或多图合成需求时，Codex 会自动选择 `laoshirenai-imagegen`。生成调用 `/v1/images/generations`，编辑调用 `/v1/images/edits`。

## 已验证范围

| 能力 | 已验证请求 |
| --- | --- |
| 图片生成 | `1024x1024`、`quality=low`、`n=1` |
| 单图编辑 | 1 张 PNG + 文本提示词 |
| 遮罩编辑 | 1 张 PNG + 1 张同尺寸 PNG 遮罩 + 文本提示词 |
| 多图编辑 | 多张 PNG + 文本提示词 |
| Codex 自动调用 | 自然语言生成与自然语言编辑 |

以下能力尚未完成同等级生产验收，因此不承诺：

- 其他尺寸和质量；
- `n>1`；
- SSE 或其他流式图片输出；
- 异步任务接口；
- Variations 接口；
- Files API 的 `file_id` 输入；
- 未在上表列出的高级参数。

## 成功标准

1. HTTP 200；
2. `data[0].b64_json` 存在且可解码为 1024×1024 图片；
3. 网关生成一条完成状态的 Usage；
4. 余额变化与该 Usage 的实际费用一致。

非幂等图片任务超时后，不要立即自动重试；先确认前一次是否已经生成并计费。

## 常见错误

- `400 images endpoint requires an image model`：使用了文本模型名。
- `401`：Key 缺失、无效或已停用。
- `403`：当前 Key 不属于 GPT Image 2 生图分组。
- `413`：上传图片或请求体过大。
- `429`：并发或请求频率达到限制。
- `500`–`504`：网关或上游暂时失败；确认没有成功图片后再决定是否重试。

不要把 Key 写入项目 `.env`、仓库、截图、聊天记录、URL 或命令参数。
