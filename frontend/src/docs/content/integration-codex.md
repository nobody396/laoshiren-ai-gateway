# Codex 安装与配置

OpenAI 代码助手，适合在终端和 VS Code 中读取、修改并验证代码。

请先安装 Node.js 和 npm。本文使用老实人AI的 Responses 接口，Base URL 为
`https://api.laoshirenai.com`；API Key 从站内 [API 密钥](https://laoshirenai.com/keys) 页面获取。

> 不要把真实 API Key 写进本文、聊天记录或项目仓库。下面的登录命令会在终端中隐藏输入，
> 并由 Codex 保存到用户目录下的 `auth.json`。

---

## Linux / macOS 配置

### 步骤 1：安装 Codex

```bash
npm install -g @openai/codex@latest
```

### 步骤 2：创建配置文件夹

```bash
mkdir -p ~/.codex
```

### 步骤 3：创建 config.toml

先到 [API 密钥](https://laoshirenai.com/keys) 打开准备使用的 Key，复制该 Key 模型列表中的一个模型 ID。
如果 `~/.codex/config.toml` 已存在，请先备份并合并下面的 Provider，不要覆盖已有 MCP 或其他个人配置。

```toml
model_provider = "laoshirenai"
model = "YOUR_MODEL_ID"
cli_auth_credentials_store = "file"
suppress_unstable_features_warning = true
personality = "pragmatic"

[model_providers.laoshirenai]
name = "老实人AI"
base_url = "https://api.laoshirenai.com"
wire_api = "responses"
requires_openai_auth = true
```

把 `YOUR_MODEL_ID` 替换成这把 Key 的模型列表中实际存在的模型 ID。`base_url` 使用上面的根地址，
不要重复添加 `/v1` 或 `/responses`；Codex 会自动补齐请求路径。

### 步骤 4：登录并生成 auth.json

从 [API 密钥](https://laoshirenai.com/keys) 页面复制 Key，然后在终端执行。粘贴后按 Enter；输入不会显示，
也不会保存到 shell 启动文件。

```bash
read -s OPENAI_API_KEY
printf '\n'
printf '%s' "$OPENAI_API_KEY" | codex login --with-api-key
unset OPENAI_API_KEY
codex login status
```

Codex 会把凭据写入 `~/.codex/auth.json`。不要把这个文件提交到 Git，也不要把内容发给他人。
建议把两个文件限制为仅当前用户可读写：

```bash
chmod 600 ~/.codex/config.toml ~/.codex/auth.json
```

### 步骤 5：启动 Codex

```bash
cd your-project-folder
codex
```

---

## Windows 配置

### 步骤 1：安装 Codex

```powershell
npm install -g @openai/codex@latest
```

### 步骤 2：创建配置文件夹

查看当前用户目录：

```powershell
echo $env:USERPROFILE
```

创建 `.codex` 文件夹：

```powershell
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\.codex"
```

### 步骤 3：创建 config.toml

先到 [API 密钥](https://laoshirenai.com/keys) 打开准备使用的 Key，复制该 Key 模型列表中的一个模型 ID。
如果 `%USERPROFILE%\.codex\config.toml` 已存在，请先备份并合并下面的 Provider。

```toml
model_provider = "laoshirenai"
model = "YOUR_MODEL_ID"
cli_auth_credentials_store = "file"
suppress_unstable_features_warning = true
personality = "pragmatic"

[model_providers.laoshirenai]
name = "老实人AI"
base_url = "https://api.laoshirenai.com"
wire_api = "responses"
requires_openai_auth = true

[features]
elevated_windows_sandbox = true
```

把 `YOUR_MODEL_ID` 替换成这把 Key 的模型列表中实际存在的模型 ID。`base_url` 不要重复添加
`/v1` 或 `/responses`。

### 步骤 4：登录并生成 auth.json

从 [API 密钥](https://laoshirenai.com/keys) 页面复制 Key，然后在 PowerShell 中执行：

```powershell
$secureKey = Read-Host "API Key" -AsSecureString
$plainKey = [System.Net.NetworkCredential]::new("", $secureKey).Password
$plainKey | codex login --with-api-key
$plainKey = $null
codex login status
```

Codex 会把凭据写入 `$env:USERPROFILE\.codex\auth.json`。不要手工把真实 Key 写进文档、脚本或项目仓库，
也不需要设置永久的 `OPENAI_API_KEY` 用户环境变量。

### 步骤 5：启动 Codex

新开一个 PowerShell，进入工程目录并启动：

```powershell
cd your-project-folder
codex
```

---

## VS Code 配置

请先按上面的步骤配置 Codex CLI，并确认 `codex login status` 成功。Codex VS Code 扩展会复用同一用户目录下的
`config.toml` 和登录凭据。

### 步骤 1：安装 Codex 扩展

在 VS Code 扩展市场搜索并安装 `Codex`。

### 步骤 2：确认配置文件

macOS / Linux 使用 `~/.codex/config.toml`；Windows 使用 `%USERPROFILE%\.codex\config.toml`。

### 步骤 3：确认 API Key 登录

通常不需要在扩展里再次输入 Key。如果扩展仍提示登录，请先在 VS Code 集成终端运行：

```bash
codex login status
```

然后执行 `Developer: Reload Window`。

---

## 验证

```bash
codex --version
codex exec --skip-git-repo-check --ephemeral --json "只回复 CODEX_OK"
```

看到最终回复 `CODEX_OK`，并在站内使用记录中看到这次请求，才表示 API Key、Base URL、模型和 Codex 会话均已接通。

## 常见问题

- 出现官方网页登录：确认 `requires_openai_auth = true`，再运行 `codex login --with-api-key`。
- 返回 `401`：API Key 缺失、错误或已经停用，请重新复制当前 Key 登录。
- 返回模型不可用：只能填写当前 Key 的模型列表实际返回的模型 ID，不要猜模型名。
- 报 `wire_api = "chat" is no longer supported`：把 Provider 改为 `wire_api = "responses"`。
- VS Code 仍显示旧配置：先确认终端里的 `codex login status`，再执行 `Developer: Reload Window`。
