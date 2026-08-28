# Models

查询当前 API Key 真正可以使用的模型：

```bash
curl https://api.laoshirenai.com/v1/models \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## 规则

- 请求模型必须使用返回结果中的 `id`。
- 同一个逻辑模型只使用一个稳定模型 ID。
- Plus、Pro、混池等是不同可用方案，不是不同模型。
- 当前 Key 只能看到所属方案开放的模型；未来通用 Key 可以返回全部模型。

完整的实时目录见[模型目录](models)。
