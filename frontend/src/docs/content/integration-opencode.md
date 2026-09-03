# OpenCode

OpenCode 可以通过自定义 OpenAI 兼容 Provider 接入老实人AI。本文已在 OpenCode `1.18.15` 上分别使用 `gpt-5.6-sol`、`kimi-k3` 和 `glm-5.3` 完成文件读取、Shell、文件修改和多轮任务测试。

## 客户端原生协议

OpenCode 由 Provider Package 决定协议：

- `@ai-sdk/openai-compatible` → Chat Completions；
- `@ai-sdk/openai` → Responses；
- `@ai-sdk/anthropic` → Anthropic Messages；
- `@ai-sdk/google` → Gemini GenerateContent。

每种协议应使用独立 Provider ID；客户端能加载 Package 只是协议候选，模型能否完成工具循环仍需实测。

## 模型兼容范围

- 当前已完成 OpenCode Agent 闭环：`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`、`gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`、全部 Qwen 3.6/3.7/3.8 公布模型、`deepseek-v4-pro-0813`、`deepseek-v4-flash-0731`、`claude-sonnet-5`、`gemini-3.7-flash`、`grok-4.6`、`kimi-k3`、`glm-5.3`。
- 同一份 Provider 模板可以声明多个模型，但当前使用的 Key 必须返回准备选择的模型。
- 切换模型系列时，必须同时切换 Provider Package、模型 ID、Base URL 和拥有该模型的 Key；不能只改模型名。

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择准备使用模型的分组创建 Key。

## 2. 设置 Key

```bash
export LAOSHIRENAI_API_KEY='YOUR_API_KEY'
```

## 3. 手动配置

在项目目录创建 `opencode.json`：

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "laoshirenai": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "老实人AI",
      "options": {
        "baseURL": "https://api.laoshirenai.com/v1",
        "apiKey": "{env:LAOSHIRENAI_API_KEY}"
      },
      "models": {
        "YOUR_MODEL_ID": {
          "name": "YOUR_MODEL_ID"
        }
      }
    }
  }
}
```

`@ai-sdk/openai-compatible` 使用 Chat Completions。配置中的 Key 来自环境变量，不需要写入 JSON。把两个 `YOUR_MODEL_ID` 都替换为当前 Key 返回的模型 ID。

跨协议 Provider Package 与 Base URL：

| 协议 | Provider Package | Base URL |
| --- | --- | --- |
| Responses | `@ai-sdk/openai` | `https://api.laoshirenai.com/v1` |
| Chat Completions | `@ai-sdk/openai-compatible` | `https://api.laoshirenai.com/v1` |
| Anthropic Messages | `@ai-sdk/anthropic` | `https://api.laoshirenai.com/v1` |
| Gemini GenerateContent | `@ai-sdk/google` | `https://api.laoshirenai.com/v1beta` |

## 4. 启动使用

```bash
opencode run --model laoshirenai/YOUR_MODEL_ID '读取当前目录并只回复 OK'
```

同一个 OpenCode 配置可以使用 GPT 或 Kimi，但切换模型时也必须换成拥有该模型的 Key。协议相同不代表任意 Key 都能调用任意模型。

`opencode.json` 必须位于当前项目根目录，或者通过 `OPENCODE_CONFIG=/完整路径/opencode.json` 显式指定。若 OpenCode 启动在另一个项目目录，它会报 `ProviderModelNotFoundError`；这属于配置没有加载，不是 Chat Completions 协议不兼容。

## 常见错误

- Provider 不出现：检查 `provider`、`npm` 和 `models` 层级是否正确。
- `ProviderModelNotFoundError`：确认启动目录包含这份 `opencode.json`，或设置 `OPENCODE_CONFIG` 指向它。
- `401`：环境变量没有设置或 Key 无效。
- 模型不可用：Key 所选分组没有开放该模型。
