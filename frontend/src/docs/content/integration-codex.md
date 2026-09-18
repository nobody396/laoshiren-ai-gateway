# Codex

Codex 使用 **OpenAI Responses** 协议。本文已在 Codex CLI `0.151.0` 上完成 GPT、Qwen 与 DeepSeek 的真实 Shell 读文件 Agent 闭环，并在 `0.153.4` 上补充完成 GPT-6 Astra 的分组模型目录、Shell 工具续轮和最终回复验收；同时复核 Chat Completions 配置边界。

## 客户端原生协议

- OpenAI Responses：支持，也是当前唯一可配置的 `wire_api`。
- Chat Completions：当前版本不支持；`wire_api = "chat"` 会在请求发出前被拒绝。
- Anthropic Messages / Gemini GenerateContent：不支持。

## 模型兼容范围

- 当前 GPT 标准线路可选择：`gpt-6-astra`、`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.5`、`gpt-5.4`、`gpt-5.3-codex-spark`；最终以当前 Key 的模型列表为准。
- 已完成真实 Codex Agent 闭环：`gpt-6-astra`、`gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`、`gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`、`qwen3.6-flash`、`qwen3.6-plus`、`qwen3.7-flash`、`qwen3.7-max`、`qwen3.7-plus`、`qwen3.8-max`、`deepseek-v4-flash-0731`、`deepseek-v4-pro-0813`。
- Qwen 与 DeepSeek 会显示“未知模型，使用 fallback model metadata”警告，但实测可以完成 Responses 流式续轮、Shell 调用和最终回复。
- `glm-5.2` 虽能调用 Responses API，但真实 Codex 工具续轮反复断流并最终失败，因此不列为 Codex 可用模型。
- `kimi-k3` 虽然可以调用 Responses API，但在 Codex 中缺少兼容模型元数据，真实客户端测试失败，因此不列为 Codex 支持模型。
- 当前 Codex CLI `0.151.0` 已不支持 `wire_api = "chat"`；配置后会在发出网络请求前直接报错，必须使用 `wire_api = "responses"`。

## 推理档位

- `gpt-6-astra`、`gpt-5.6-sol`、`gpt-5.6-terra` 在 `/model` 里可以选到 **Ultra**。
- Ultra 是 Codex 客户端的档位，不是 Responses API 的取值：直接把 `ultra` 发给上游会返回 400。网关会把它改写成 `max` 再转发，所以选 Ultra 实际按 **max** 推理，**不包含** ChatGPT 端 Ultra 的自动子任务分发。
- 分组如果设了推理上限或映射，改写后的 `max` 仍然照常受限。例如企业高速线路把 `max` 映射为 `xhigh`，在该线路上选 Ultra 就按 `xhigh` 执行。
- 其余模型的档位以各自模型目录为准，不提供 Ultra。

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，按准备使用的模型选择 GPT、Qwen 或 DeepSeek 分组创建 Key，再从该 Key 的模型列表复制模型 ID。

## 2. 一键配置

在 Key 右侧点击 **一键配置**。页面生成的命令会安装或复用 Codex，写入 Responses Provider、Key 和当前分组模型目录。

macOS / Linux 命令结构：

```bash
curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=0.7.14' | \
  LAOSHIRENAI_SETUP_TOKEN='ONE_TIME_SETUP_TOKEN' \
  LAOSHIRENAI_TOOLS='codex' bash
```

Windows PowerShell 命令结构：

```powershell
$env:LAOSHIRENAI_SETUP_TOKEN='ONE_TIME_SETUP_TOKEN';
$env:LAOSHIRENAI_TOOLS='codex';
irm 'https://laoshirenai.com/auto-config/install.ps1?v=0.7.14' | iex
```

## 3. 手动配置

设置 Key：

```bash
export OPENAI_API_KEY='YOUR_API_KEY'
```

编辑 `~/.codex/config.toml`；Windows 路径是 `%USERPROFILE%\.codex\config.toml`：

```toml
model_provider = "laoshirenai"
model = "YOUR_MODEL_ID"
preferred_auth_method = "apikey"

[model_providers.laoshirenai]
name = "laoshirenai"
base_url = "https://api.laoshirenai.com"
wire_api = "responses"
env_key = "OPENAI_API_KEY"
requires_openai_auth = false
```

`base_url` 使用根地址，不要添加 `/v1`；Codex 会自行请求 `/responses`。`env_key` 让 Codex 从环境变量读取 Key，不需要把 Key 写进 TOML。

## 4. 启动使用

```bash
codex --version
codex
```

非交互最小任务：

```bash
codex exec --sandbox read-only '读取当前目录并只回复 OK，不要修改文件'
```

Codex 还依赖模型元数据、工具行为和流式续轮，不是所有 Responses 模型都能直接替代 GPT 编码模型。只使用上方已完成真实 Agent 闭环、且由当前 Key 返回的模型。

## 常见错误

- 出现官方登录页：检查 `requires_openai_auth` 是否为 `false`，并确认 `env_key` 对应的环境变量已经设置。
- `401 API_KEY_REQUIRED`：Codex 没有读到 `OPENAI_API_KEY`。
- 未知模型警告：使用当前 Key 返回且 Codex 模型目录支持的模型。
- `wire_api = "chat" is no longer supported`：这是当前 Codex 的客户端限制；不能用 Chat Completions 绕过，应改用 Responses 或选择支持 Chat 的 ZCode / OpenCode。
