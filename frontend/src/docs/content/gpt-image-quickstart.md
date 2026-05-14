# GPT-Image-2 使用指南

GPT-Image-2 是 DragonCode 提供的 AI 图片生成服务，使用独立的图片接口。

---

## 准备工作

### 1. 创建 API Key

前往 [API Keys 管理页面](https://your-domain.example/keys)，点击「创建 API Key」，分组选择 **GPT-Image**。

创建完成后请妥善保存，后续所有请求都需要在 Header 中携带此 Key。

### 2. 接口地址

GPT-Image-2 使用独立 Base URL：

```
https://your-domain.example/gpt-image/v1
```

---

## AI Agent 一键接入

如果你使用 Claude Code、Codex 等支持 Skills 的 AI 编码工具，可以直接将以下地址发送给你的 AI：

```
https://github.com/bozhouDev/images2-gen/blob/main/README.md  帮我安装这个skills 并且生成一个图片
```

AI 会自动安装生图技能并引导你完成配置，之后只需对 AI 说「帮我生成一张图片」即可。

---


## API 详细说明

### 文生图（Text to Image）

提交生成任务：

```bash
curl 'https://your-domain.example/gpt-image/v1/images/generations' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -d '{
    "model": "gpt-image-2",
    "prompt": "一只可爱的猫咪在阳光下打盹",
    "n": 1,
    "size": "1:1",
    "resolution": "2k"
  }'
```

成功响应：

```json
{
  "code": 200,
  "data": [
    {
      "status": "submitted",
      "task_id": "task_xxx"
    }
  ]
}
```

### 查询任务结果

将 `task_xxx` 替换为提交任务时返回的 `task_id`：

```bash
curl 'https://your-domain.example/gpt-image/v1/tasks/task_xxx' \
  -H 'Authorization: Bearer YOUR_API_KEY'
```

**任务处理中：**

```json
{
  "code": 200,
  "data": {
    "id": "task_xxx",
    "status": "pending",
    "progress": 0
  }
}
```

**任务完成：**

```json
{
  "code": 200,
  "data": {
    "id": "task_xxx",
    "status": "completed",
    "progress": 100,
    "result": {
      "images": [
        {
          "url": [
            "https://your-domain.example/gpt-image/media/task_xxx/0?token=..."
          ]
        }
      ]
    }
  }
}
```

最终图片地址为 `data.result.images[0].url[0]`，可直接在浏览器打开或放到 `<img src="...">` 中展示。

---

### 图生图（Image to Image）

在请求体中添加 `image_urls` 字段，即可基于参考图进行编辑或风格转换。

**使用 URL 引用图片：**

```bash
curl 'https://your-domain.example/gpt-image/v1/images/generations' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -d '{
    "model": "gpt-image-2",
    "prompt": "把这张图改成水彩风格",
    "n": 1,
    "size": "1:1",
    "resolution": "2k",
    "image_urls": [
      "https://example.com/photo.jpg"
    ]
  }'
```

**使用 base64 传入图片：**

```json
{
  "model": "gpt-image-2",
  "prompt": "把这张图改成赛博朋克风格",
  "n": 1,
  "size": "1:1",
  "resolution": "2k",
  "image_urls": [
    "data:image/png;base64,iVBORw0KGgo..."
  ]
}
```

**图生图注意事项：**

- `image_urls` 为数组，最多支持 16 张参考图
- 支持 URL 和 base64 data URI 两种格式，可混用
- base64 图片会占用请求体大小，过大可能被网关拦截，建议优先使用 URL
- 图生图的计费方式不变，仍按 `resolution` 档位计费

---

## 参数说明

| 参数 | 说明 |
|---|---|
| `model` | 固定填写 `gpt-image-2` |
| `prompt` | 图片生成提示词 |
| `n` | 固定为 `1` |
| `size` | 图片比例，见下方支持列表 |
| `resolution` | 输出分辨率档位，见下方支持列表 |
| `image_urls` | （可选）参考图片数组，用于图生图。支持 URL 和 base64 data URI，最多 16 张 |

### resolution 支持

| 档位 | 说明 |
|---|---|
| `1k` | 1K 分辨率 |
| `2k` | 2K 分辨率（推荐） |
| `4k` | 4K 分辨率（仅支持部分比例） |

### size 支持

| 比例 | 支持 1k/2k | 支持 4k |
|---|---|---|
| `auto` | 支持 | 不支持 |
| `1:1` | 支持 | 不支持 |
| `3:2` / `2:3` | 支持 | 不支持 |
| `4:3` / `3:4` | 支持 | 不支持 |
| `5:4` / `4:5` | 支持 | 不支持 |
| `16:9` / `9:16` | 支持 | 支持 |
| `2:1` / `1:2` | 支持 | 支持 |
| `21:9` / `9:21` | 支持 | 支持 |

> **注意：** `4k` 分辨率仅支持 `16:9`、`9:16`、`2:1`、`1:2`、`21:9`、`9:21` 这几种比例。其他比例搭配 `4k` 会报错。

---

## Python 示例

```python
import time
import requests

# 替换为你的 API Key
base_url = "https://your-domain.example/gpt-image/v1"
api_key = "YOUR_API_KEY"

headers = {
    "Authorization": f"Bearer {api_key}",
    "Content-Type": "application/json",
}

# 步骤一：提交生成任务
submit = requests.post(
    f"{base_url}/images/generations",
    headers=headers,
    json={
        "model": "gpt-image-2",
        "prompt": "一只可爱的猫咪在阳光下打盹",
        "n": 1,
        "size": "1:1",
        "resolution": "2k",
    },
)

submit.raise_for_status()
task_id = submit.json()["data"][0]["task_id"]
print(f"任务已提交: {task_id}")

# 步骤二：轮询查询任务状态
while True:
    result = requests.get(
        f"{base_url}/tasks/{task_id}",
        headers={"Authorization": f"Bearer {api_key}"},
    )
    result.raise_for_status()

    data = result.json()["data"]
    status = data["status"]

    if status == "completed":
        # 步骤三：获取图片链接
        image_url = data["result"]["images"][0]["url"][0]
        print(f"图片生成完成: {image_url}")
        break

    if status == "failed":
        raise RuntimeError(data.get("error", {}).get("message", "任务失败"))

    print(f"任务处理中... 进度: {data.get('progress', 0)}%")
    time.sleep(5)
```

---

## 计费说明

- 提交任务时**不扣费**
- 只有任务完成、图片成功生成并返回后，才会产生用量记录并扣费
- 失败、审核未通过、任务未完成、图片不可用等情况**不会扣费**
- 扣费根据请求中的 `resolution` 参数计算：
  - `1k` → 1K 图片价格
  - `2k` → 2K 图片价格
  - `4k` → 4K 图片价格

---

## 图片链接说明

任务完成后返回的图片链接为本站链接（`https://your-domain.example/gpt-image/media/...`），带有临时访问 token，可直接使用。
