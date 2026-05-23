# Windows 配置指南

这页用于 Windows、PowerShell 和 WSL 环境下配置 Claude Code、Codex 与老实人AI。

## 配置前准备

- 安装当前工具要求的 Node.js 或官方客户端。
- 确认使用 PowerShell，而不是 CMD。
- 准备老实人AI后台创建的 API Key。
- 确认 Key 的分组适合 Claude Code 或 Codex。

## Claude Code

Claude Code 使用：

```text
https://api.laoshirenai.com
```

不要加 `/v1`。如果之前配置过其他 Claude 服务，先清理旧 Claude/Anthropic 环境变量，再新开终端测试。

## Codex

Codex 官方客户端按站内教程使用：

```text
https://api.laoshirenai.com
```

普通 OpenAI SDK 或通用 OpenAI 兼容客户端通常使用：

```text
https://api.laoshirenai.com/v1
```

## WSL 注意事项

WSL 和 Windows 是两套环境。Windows 里配置过的 Provider 或环境变量，不一定会被 WSL 里的 Codex 读取。排查时要确认配置文件写在 WSL 实际读取的位置。

## 验收

新开终端，发送一个最短测试问题。然后到老实人AI使用记录里确认请求进入平台。

