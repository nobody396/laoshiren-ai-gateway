# Antigravity

**协议：** Anthropic Messages / Gemini GenerateContent

**配置状态：** 当前通过 Claude Code、Gemini CLI 或 OpenCode 使用

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 Antigravity 可用方案并创建 Key。

## 2. 选择客户端

- Claude 模型：使用 [Claude Code](integration-claude-code)。
- Gemini 模型：使用 [Gemini CLI](integration-gemini-cli)。
- 同时管理两类模型：等待新版 OpenCode 一次性配置完成验收。

Antigravity Base URL：

```text
https://api.laoshirenai.com/antigravity
```

不要使用普通 Claude / Gemini 分组的 Key。

## 3. 验证

完成一个新会话，并确认使用记录中的请求路径以 `/antigravity/` 开头。

## 4. 回滚

按所选客户端的回滚步骤恢复。

## 常见错误

- 路径没有 `/antigravity`：配置使用了错误 Base URL。
- `403`：当前 Key 不是 Antigravity 可用方案。
- Claude 与 Gemini 请求混用：按客户端选择正确协议。
- 修改后无效：完全退出客户端并新建会话。
