# Images

## 图片生成

```bash
curl https://api.laoshirenai.com/v1/images/generations \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "YOUR_IMAGE_MODEL_ID",
    "prompt": "一只坐在窗边的橘猫",
    "size": "1024x1024",
    "n": 1
  }'
```

## 图片编辑

```text
POST https://api.laoshirenai.com/v1/images/edits
```

图片编辑使用 `multipart/form-data` 上传图片。具体参数以[模型目录](models)中该图片模型的当前能力为准。

## 验证

只有取得可读取的图片 URL、Base64 图片或图片二进制才算成功。HTTP 202 表示任务仍在生成，不表示失败。
