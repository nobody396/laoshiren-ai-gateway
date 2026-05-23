# Base URL 填写总指南

这页用来判断老实人AI的地址到底应该填 `https://api.laoshirenai.com`，还是 `https://api.laoshirenai.com/v1`。

## 快速结论

| 使用入口 | 推荐 Base URL | 说明 |
| --- | --- | --- |
| Claude Code | `https://api.laoshirenai.com` | Claude/Anthropic 协议入口，不要加 `/v1`。 |
| Anthropic SDK | `https://api.laoshirenai.com` | 按 Anthropic Messages 协议调用。 |
| Codex CLI/App | `https://api.laoshirenai.com` | 按站内 Codex 教程使用 Responses 模式。 |
| CC Switch 一键导入 | `https://api.laoshirenai.com` | 以导入结果和站内教程为准。 |
| OpenAI SDK | `https://api.laoshirenai.com/v1` | 常见 `/v1/models`、`/v1/chat/completions`、`/v1/responses` 场景。 |
| OpenAI 兼容第三方客户端 | `https://api.laoshirenai.com/v1` | 如果客户端不会自动拼 `/v1`，就手动填到 `/v1`。 |
| Antigravity Claude | `https://api.laoshirenai.com/antigravity` | Antigravity 专用 Claude 兼容入口。 |
| Antigravity Gemini | `https://api.laoshirenai.com/antigravity/v1beta` | 如果工具自动拼 `v1beta`，按教程填根路径。 |
| GPT-Image | `https://api.laoshirenai.com/gpt-image/v1` | 生图接口专用入口。 |

## 判断方法

先看工具走什么协议，再看它是否会自动拼接路径。

- Claude Code 和 Anthropic SDK 通常走 Claude/Anthropic 协议。
- Codex 官方客户端按老实人AI教程走 Responses 模式。
- OpenAI SDK 和很多通用客户端会访问 `/v1/models` 或 `/v1/chat/completions`。
- Antigravity 和 GPT-Image 有专用路径，不要和普通 Claude/Codex 混用。

## 常见错误

- 把 Claude Code 配成 `/v1`，容易出现协议错误或连接失败。
- 把普通 OpenAI SDK 配成根地址，容易找不到模型列表。
- 分组不支持当前协议时，即使地址正确也会失败。
- 修改地址后继续用旧终端窗口，可能仍然读到旧环境变量。

## 排查顺序

1. 确认你用的是 Claude Code、Codex、SDK 还是第三方客户端。
2. 按上表确认 Base URL。
3. 确认 API Key 是老实人AI后台创建的完整 Key。
4. 确认 Key 的分组支持当前工具和模型。
5. 关闭旧终端或旧客户端窗口，重新打开后测试。

