# Codex 配置排错指南

这页用于排查 Codex CLI/App 接入老实人AI后的 Provider、Base URL、登录、WSL 和 Responses 模式问题。

## 快速结论

- Codex 官方客户端和 CC Switch 一键导入：通常使用 `https://api.laoshirenai.com`。
- 普通 OpenAI SDK 或通用 OpenAI 兼容客户端：通常使用 `https://api.laoshirenai.com/v1`。
- Codex 官方客户端按站内教程使用 Responses 模式，不要盲目套用普通 OpenAI SDK 的 `/v1` 口径。

## 常见症状

- Codex 反复要求登录。
- Provider 已添加但请求没有进入老实人AI。
- 模型列表缺失。
- WSL 下启动失败或 TUI 初始化失败。
- 请求返回 401、403、404、429、502、503。

## 排查顺序

1. 确认你用的是 Codex 官方客户端，不是普通 OpenAI SDK。
2. 确认 Provider 的 Base URL 是站内教程要求的地址。
3. 确认 API Key 是完整 Key，不是订单号或兑换码。
4. 确认 Key 分组支持 Codex/OpenAI 协议。
5. 如果在 WSL 中使用，确认配置文件写在 WSL 环境实际读取的位置。
6. 关闭旧窗口，重新打开 Codex 测试。
7. 到使用记录里确认请求是否进入平台。

## 判断是客户端还是平台

如果使用记录里完全没有请求，优先查本地 Provider、Key、环境变量和网络。  
如果使用记录里有请求但失败，优先查分组、模型、余额、限流和上游状态。

