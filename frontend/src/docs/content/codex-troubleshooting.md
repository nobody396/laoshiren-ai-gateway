# Codex 配置排错指南

这页用于排查 Codex CLI/App 接入老实人AI后的 Provider、Base URL、登录、WSL 和 Responses 模式问题。

## 快速结论

- Codex 官方客户端和 CC Switch 一键导入：通常使用 `https://api.laoshirenai.com`。
- 普通 OpenAI SDK 或通用 OpenAI 兼容客户端：通常使用 `https://api.laoshirenai.com/v1`。
- Codex 官方客户端按站内教程使用 Responses 模式，不要盲目套用普通 OpenAI SDK 的 `/v1` 口径。
- 第三方 API Key 主要支持本地 Codex 调用模型；需要官方插件、Codex cloud、自动 code review、Slack / GitHub 等云端集成时，请使用 ChatGPT/OpenAI 官方登录入口。
- CC Switch 里 `OpenAI Official` 才代表官方订阅；老实人 AI Provider 代表中转 API，不要把两者混在一起。
- Codex++ 是第三方 Codex App 外部增强启动器。它不是 OpenAI 官方产品；它通常要求先有 Codex App 官方登录态，再通过中转注入把模型请求转到兼容 API。

## 常见症状

- Codex 反复要求登录。
- Provider 已添加但请求没有进入老实人AI。
- 本地 `codex` 能用，但没有官方插件、云端任务或自动化入口。
- 从原版 Codex App 打开时没有 Codex++ 菜单或增强功能。
- 模型列表缺失。
- WSL 下启动失败或 TUI 初始化失败。
- 请求返回 401、403、404、429、502、503。

## 排查顺序

1. 确认你用的是 Codex 官方客户端，不是普通 OpenAI SDK。
2. 先判断你要的是哪条路：本地 API 任务用老实人 AI Provider；完整官方功能用 `OpenAI Official`；Codex App 增强和中转注入用 Codex++ 入口。
3. 确认 Provider 的 Base URL 是站内教程要求的地址。
4. 确认 API Key 是完整 Key，不是订单号或兑换码。
5. 确认 Key 分组支持 Codex/OpenAI 协议。
6. 如果在 WSL 中使用，确认配置文件写在 WSL 环境实际读取的位置。
7. 关闭旧窗口，重新打开 Codex 测试。
8. 到使用记录里确认请求是否进入平台。

## Codex++ 常见判断

- 如果没有看到 `Codex++` 菜单，先确认你是从 `Codex++` 入口启动，而不是从原版 Codex App 启动。
- 如果插件入口仍提示需要登录 ChatGPT，先确认 Codex App 本身已经完成官方账号登录。
- 如果中转注入后请求没有进入平台，检查 Codex++ 管理工具里当前启用的 Base URL、Key 和模型配置。
- 如果要回到官方模式，在 Codex++ 管理工具里清除 API 模式，再重启 Codex App。

## 判断是客户端还是平台

如果使用记录里完全没有请求，优先查本地 Provider、Key、环境变量和网络。  
如果使用记录里有请求但失败，优先查分组、模型、余额、限流和上游状态。
