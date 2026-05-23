# macOS 配置指南

这页用于 macOS 终端环境下配置 Claude Code、Codex、环境变量和老实人AI API Key。

## 配置前准备

- 准备老实人AI后台创建的 API Key。
- 确认 Key 的分组适合当前工具。
- 关闭旧终端窗口，避免旧环境变量影响测试。

## Claude Code

Claude Code 使用：

```text
https://api.laoshirenai.com
```

如果之前配置过其他 Claude 或 Anthropic 服务，优先清理旧环境变量，再重新打开终端。

## Codex

Codex 官方客户端按站内教程配置 Provider 和 Responses 模式。普通 OpenAI SDK 则通常使用 `/v1` 地址。

## 排查重点

- 终端是否读到了新的环境变量。
- 当前客户端实际启用的是哪个 Provider。
- API Key 是否完整。
- 分组是否支持当前协议。
- 使用记录里是否出现请求。

## 验收

用最短问题测试，不要用旧长会话判断新配置是否生效。

