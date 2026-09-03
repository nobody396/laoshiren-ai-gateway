# Kimi Code

Kimi Code 可以通过 **OpenAI Chat Completions** 协议连接老实人AI。本文配置已在 Kimi Code CLI `0.39.1` 与 `kimi-k3` 上完成文件读取、Shell、文件修改和多轮任务测试。

## 客户端原生协议

Kimi Code 的 Provider 类型决定协议：

- `openai` → Chat Completions；
- `openai_responses` → Responses；
- `anthropic` → Anthropic Messages；
- `google-genai` → Gemini GenerateContent。

客户端具备四种 Provider，不代表任意模型能在四种协议下完成 Agent 任务；仍需匹配模型分组和工具调用格式。

## 模型兼容范围

- 当前 Kimi Key 可选择：`kimi-k2.7-code`、`kimi-k3`。
- 已完成真实 Kimi Code Agent 闭环：`kimi-k3`。
- 跨协议真实 Agent 闭环：`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`、`gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`（Responses），`glm-5.3`（Chat Completions），`claude-sonnet-5`（Messages），`gemini-3.7-flash`（GenerateContent）。
- DeepSeek Responses 已完成真实 Agent 闭环：`deepseek-v4-pro-0813`、`deepseek-v4-flash-0731`。
- Qwen Responses 已完成真实 Agent 闭环：`qwen3.7-max`、`qwen3.7-plus`、`qwen3.8-max`。`qwen3.6-flash`、`qwen3.6-plus`、`qwen3.7-flash` 在当前生产网关重复返回 502，暂记为不支持；模型级 reasoning 映射修复部署后需要复测。
- Kimi Code 的通用推理档位是 `low`、`medium`、`high`、`xhigh`、`max`。GLM 5.3 只接受 `low`、`high`、`max`，网关会将 `medium → high`、`xhigh → max`、`none/minimal → low` 后再转发。
- 四种 Provider 是四类候选入口，不代表任意模型均可用；只使用本文列出的已实测模型。

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 Kimi 分组创建 Key。

## 2. 安装

```bash
npm install -g @moonshot-ai/kimi-code@0.39.1
kimi --version
```

也可以使用官方安装脚本：

```bash
curl -fsSL https://code.kimi.com/kimi-code/install.sh | bash
```

## 3. 手动配置

Kimi Code 提供 `KIMI_MODEL_*` 临时配置通道，不需要修改配置文件：

```bash
export KIMI_MODEL_NAME='kimi-k3'
export KIMI_MODEL_API_KEY='YOUR_API_KEY'
export KIMI_MODEL_PROVIDER_TYPE='openai'
export KIMI_MODEL_BASE_URL='https://api.laoshirenai.com/v1'
export KIMI_MODEL_MAX_CONTEXT_SIZE='262144'
export KIMI_MODEL_CAPABILITIES='thinking,tool_use'
```

长期配置文件是 `~/.kimi-code/config.toml`。Kimi Code 的普通 `KIMI_API_KEY` 环境变量不会自动成为自定义 Provider 凭据；直接使用上面的 `KIMI_MODEL_*` 方式最简单。

## 4. 启动使用

```bash
kimi
```

单次任务：

```bash
kimi -p '读取当前目录并只回复 OK' --output-format text
```

Kimi Code 的 Provider 类型决定协议；`openai` 对应 Chat Completions。模型仍必须来自当前 Key 的模型列表。

跨协议实测配置：

| 模型示例 | `KIMI_MODEL_PROVIDER_TYPE` | `KIMI_MODEL_BASE_URL` |
| --- | --- | --- |
| `gpt-5.6-sol` | `openai_responses` | `https://api.laoshirenai.com/v1` |
| `claude-sonnet-5` | `anthropic` | `https://api.laoshirenai.com` |
| `gemini-3.7-flash` | `google-genai` | `https://api.laoshirenai.com` |

每次切换模型系列时，同时更新模型、Provider 类型、Base URL 和 Key。

## 常见错误

- 缺少 `KIMI_MODEL_API_KEY`：CLI 会在启动时直接报凭据缺失。
- 模型不存在：把 `KIMI_MODEL_NAME` 改为当前 Key 返回的模型 ID。
