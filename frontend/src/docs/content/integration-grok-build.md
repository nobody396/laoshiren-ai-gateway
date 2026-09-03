# Grok Build

Grok Build 通过 **OpenAI Responses** 协议调用 Grok。本文配置已在 Grok Build `1.0.13` 与 `grok-4.6` 上完成文件读取、Shell、文件修改和多轮任务测试。

## 客户端原生协议

Grok Build `1.0.13` 的自定义模型 `api_backend` 支持：

- `responses` → OpenAI Responses；
- `chat_completions` → OpenAI Chat Completions；
- `messages` → Anthropic Messages。

Gemini GenerateContent 不支持。未知 `api_backend` 不会变成 Gemini，而会回退到默认 Chat 行为，因此不能据此宣称支持。

## 模型兼容范围

- 当前 Grok Key 可选择：`grok-4.5`、`grok-4.6`。
- 已完成真实 Grok Build Agent 闭环：`grok-4.6`。
- 跨协议真实 Agent 闭环：`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`、`gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`（Responses），`glm-5.3`（Chat Completions），`claude-sonnet-5`（Messages）。
- DeepSeek Responses 已完成真实 Agent 闭环：`deepseek-v4-pro-0813`、`deepseek-v4-flash-0731`。
- Qwen Responses 已完成真实 Agent 闭环：`qwen3.6-flash`、`qwen3.6-plus`、`qwen3.7-flash`、`qwen3.7-plus`、`qwen3.8-max`。`qwen3.7-max` 能返回工具结果，但当前客户端 resident actor 重复以 `DeadFailed` 退出，暂记为不支持。
- 因此 Grok Build 不只能够使用 Grok 模型；但仍只能选择当前 Key 返回、且在本文列为已实测的模型。不要从“支持三种后端”推导所有模型均可用。

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 Grok 分组创建 Key。

## 2. 一键配置

在 Key 右侧点击 **一键配置**，选择 Grok Build。生成命令的结构与其他客户端一致：

```bash
curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=0.7.14' | \
  LAOSHIRENAI_SETUP_TOKEN='ONE_TIME_SETUP_TOKEN' \
  LAOSHIRENAI_TOOLS='grok' bash
```

Windows 使用同版本 `install.ps1`，并把 `LAOSHIRENAI_TOOLS` 设为 `grok`。

## 3. 手动配置

先设置 Key：

```bash
export LAOSHIRENAI_GROK_API_KEY='YOUR_API_KEY'
```

编辑 `~/.grok/config.toml`：

```toml
[models]
default = "laoshirenai-grok"
web_search = "laoshirenai-grok"

[model."laoshirenai-grok"]
model = "grok-4.6"
base_url = "https://api.laoshirenai.com"
name = "Grok 4.6"
env_key = "LAOSHIRENAI_GROK_API_KEY"
api_backend = "responses"
context_window = 262144
supports_backend_search = true
```

需要其他 Grok 模型时，把 `model` 改成当前 Key 模型列表中的 ID。

跨协议模型使用对应的 `api_backend` 和 Base URL：

| 模型示例 | `api_backend` | Base URL |
| --- | --- | --- |
| `gpt-5.6-sol` | `responses` | `https://api.laoshirenai.com` |
| `glm-5.3` | `chat_completions` | `https://api.laoshirenai.com/v1` |
| `claude-sonnet-5` | `messages` | `https://api.laoshirenai.com/v1` |

切换模型系列时还要换成拥有该模型的 Key，不能只修改 `model`。

## 4. 启动使用

```bash
grok --version
grok --model grok-4.6
```

单次任务：

```bash
grok --single '读取当前目录并只回复 OK' --model grok-4.6
```

## 常见错误

- `401`：环境变量没有设置或 Key 无效。
- 模型不存在：从当前 Key 的 `/v1/models` 复制模型 ID。
