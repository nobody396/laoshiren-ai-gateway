# Antigravity

Antigravity CLI 使用 **Gemini GenerateContent** 协议。本文使用 Antigravity CLI `1.1.27`、Gemini 分组和 `gemini-3.8-flash`、`gemini-3.7-flash` 完成了真实 Agent 文件读取、Shell、文件修改和修改后复读测试。

## 客户端原生协议

- Gemini GenerateContent：支持；API Key 自定义端点只接受 `modelProvider: "gemini"`。
- OpenAI Responses / Chat Completions / Anthropic Messages：没有公开的自定义 Provider 配置入口。
- 默认 Google 登录会连接共享 Agent Harness，但这不是可由用户填写 Base URL 的额外公开协议。

## 模型兼容范围

- 已完成真实 Antigravity Agent 闭环：`gemini-3.8-flash`、`gemini-3.7-flash`。
- Antigravity 使用 `gemini-3.7-flash`，并通过 `--effort=low` 或 `--effort=high` 控制当前会话的推理档位；不再依赖单独的 High 模型别名。
- 当前不要选择 `gemini-3.1-pro`：Antigravity `1.1.27` 仍会将它转换为 `gemini-3.1-pro-preview`，而当前 Gemini Key 没有开放该 Preview ID。
- 因为 Gemini 分组当前同时公开以上三个模型，而 `gemini-3.1-pro` 不能通过，所以密钥页面不会显示 Antigravity 一键配置，避免只导入部分模型后误报成功。

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 Gemini 分组创建 Key，再从该 Key 的模型列表确认包含 `gemini-3.7-flash`。

Antigravity CLI 的自定义端点是 Gemini Base URL：

```text
https://api.laoshirenai.com
```

不要填写 `/v1`、`/v1beta`、模型名或完整 `generateContent` 路径，CLI 会自动追加 Gemini 请求路径。

## 2. 安装或更新

macOS / Linux：

```bash
curl -fsSL https://antigravity.google/cli/install.sh | bash
agy --version
```

Windows PowerShell：

```powershell
irm https://antigravity.google/cli/install.ps1 | iex
agy --version
```

本文命令以 `1.1.27` 为验证版本。旧版 `1.0.1` 没有 `--model` 参数，请先更新。

## 3. 手动配置

创建或编辑：

```text
macOS / Linux: ~/.gemini/antigravity-cli/settings.json
Windows: %USERPROFILE%\.gemini\antigravity-cli\settings.json
```

写入：

```json
{
  "modelProvider": "gemini",
  "agentMode": "accept-edits"
}
```

当前终端设置 Base URL 和 Key：

```bash
export GOOGLE_GEMINI_BASE_URL='https://api.laoshirenai.com'
export GEMINI_API_KEY='YOUR_API_KEY'
```

Windows PowerShell：

```powershell
$env:GOOGLE_GEMINI_BASE_URL='https://api.laoshirenai.com'
$env:GEMINI_API_KEY='YOUR_API_KEY'
```

只设置 `GEMINI_API_KEY` 不够；`settings.json` 中还必须将 `modelProvider` 设为 `gemini`。

## 4. 启动使用

交互模式：

```bash
agy --model=gemini-3.7-flash --effort=low
```

最小非交互请求：

```bash
agy --print "只回复 ANTIGRAVITY_OK" \
  --model=gemini-3.7-flash \
  --effort=low
```

进入项目目录后启动 `agy`，即可让 Agent 读取项目、运行命令和修改文件。默认权限模式会在写文件或执行命令前请求确认。

## 当前可用范围

- Antigravity CLI `1.1.27`；
- Gemini API Key 模式；
- Gemini 兼容 Base URL；
- `gemini-3.8-flash` 或 `gemini-3.7-flash`，`low` effort；
- 读取文件、运行 Shell、修改文件和多步 Agent 任务。

## 常见错误

- `flags provided but not defined: -model`：CLI 版本过旧，请更新到 `1.1.27` 或更高版本。
- `--model ... requires --effort`：同时传入 `--effort=low`。
- `GEMINI_API_KEY is not set`：当前终端没有设置 Key，或启动 CLI 后才设置。
- CLI 打开 Google 登录而不是 Key 模式：检查 `settings.json` 中的 `modelProvider` 是否严格等于 `gemini`。
- `model not supported`：改用当前 Key 模型列表中已开放的 `gemini-3.7-flash`。

Antigravity 的 API Key 模式、自定义端点和配置文件规则可参阅 [Antigravity 官方安装与鉴权文档](https://antigravity.google/docs/cli/install/)。
