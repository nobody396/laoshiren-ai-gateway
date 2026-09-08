本文说明如何把老实人AI API Key 配置到 **Claude Code CLI**。配置完成后，Claude Code 会通过老实人AI的 Anthropic Messages 接口调用模型。

生产 Base URL：

```text
https://api.laoshirenai.com
```

## 开始之前

先准备两样东西：

1. 已经安装 Claude Code；
2. 已在[老实人AI API 密钥页面](https://laoshirenai.com/keys)创建 Key，并知道这把 Key 可以使用的模型 ID。

### 安装 Claude Code

macOS / Linux / WSL：

```bash
curl -fsSL https://claude.ai/install.sh | bash
```

Windows PowerShell：

```powershell
irm https://claude.ai/install.ps1 | iex
```

### 国内网络环境一键安装并配置

下面使用老实人AI现有的安装配置脚本。它会自动准备 Node.js、安装或更新 Claude Code，并写入老实人AI配置；Windows 缺少 Git Bash 时也会自动准备。执行后按照提示粘贴 API Key。

运行前先把 `YOUR_MODEL_ID` 替换成当前 Key 模型列表中的模型 ID。

macOS / Linux / WSL：

```bash
curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=0.7.17' | LAOSHIRENAI_TOOLS='claude' LAOSHIRENAI_MODEL_ID='YOUR_MODEL_ID' LAOSHIRENAI_PROTOCOL='messages' bash
```

Windows PowerShell：

```powershell
$env:LAOSHIRENAI_TOOLS='claude'; $env:LAOSHIRENAI_MODEL_ID='YOUR_MODEL_ID'; $env:LAOSHIRENAI_PROTOCOL='messages'; irm 'https://laoshirenai.com/auto-config/install.ps1?v=0.7.17' | iex
```

也可以通过 npm 安装：

```bash
npm install -g @anthropic-ai/claude-code
```

安装后先确认：

```bash
claude --version
```

## 方式一：修改 settings.json（推荐）

这种方式最直观，Claude Code 每次启动都会读取同一份设置。

配置文件位置：

| 系统 | 文件位置 |
| --- | --- |
| macOS / Linux / WSL | `~/.claude/settings.json` |
| Windows | `%USERPROFILE%\.claude\settings.json` |

文件不存在时创建它；已经存在时，只合并下面的 `env` 字段，不要覆盖原来的权限、MCP 或其他设置。

```json
{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "YOUR_API_KEY",
    "ANTHROPIC_BASE_URL": "https://api.laoshirenai.com",
    "ANTHROPIC_MODEL": "YOUR_MODEL_ID",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "YOUR_MODEL_ID",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "YOUR_MODEL_ID",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "YOUR_MODEL_ID",
    "CLAUDE_CODE_SUBAGENT_MODEL": "YOUR_MODEL_ID",
    "API_TIMEOUT_MS": "3000000"
  }
}
```

替换两处内容：

- `YOUR_API_KEY`：替换成你在老实人AI创建的 Key；
- 所有 `YOUR_MODEL_ID`：替换成这把 Key 模型列表里实际显示的同一个模型 ID。

保存文件后，完全退出 Claude Code，再重新打开终端运行：

```bash
claude
```

## 方式二：使用环境变量

不想修改 `settings.json` 时，可以先在当前终端设置环境变量。关闭终端后，这些设置会失效。

macOS / Linux / WSL：

```bash
export ANTHROPIC_AUTH_TOKEN="YOUR_API_KEY"
export ANTHROPIC_BASE_URL="https://api.laoshirenai.com"
export ANTHROPIC_MODEL="YOUR_MODEL_ID"
export ANTHROPIC_DEFAULT_OPUS_MODEL="YOUR_MODEL_ID"
export ANTHROPIC_DEFAULT_SONNET_MODEL="YOUR_MODEL_ID"
export ANTHROPIC_DEFAULT_HAIKU_MODEL="YOUR_MODEL_ID"
export CLAUDE_CODE_SUBAGENT_MODEL="YOUR_MODEL_ID"
export API_TIMEOUT_MS="3000000"
claude
```

Windows PowerShell：

```powershell
$env:ANTHROPIC_AUTH_TOKEN = "YOUR_API_KEY"
$env:ANTHROPIC_BASE_URL = "https://api.laoshirenai.com"
$env:ANTHROPIC_MODEL = "YOUR_MODEL_ID"
$env:ANTHROPIC_DEFAULT_OPUS_MODEL = "YOUR_MODEL_ID"
$env:ANTHROPIC_DEFAULT_SONNET_MODEL = "YOUR_MODEL_ID"
$env:ANTHROPIC_DEFAULT_HAIKU_MODEL = "YOUR_MODEL_ID"
$env:CLAUDE_CODE_SUBAGENT_MODEL = "YOUR_MODEL_ID"
$env:API_TIMEOUT_MS = "3000000"
claude
```

## 切换模型

### 临时切换

macOS / Linux / WSL：

```bash
ANTHROPIC_MODEL="NEW_MODEL_ID" claude
```

Windows PowerShell：

```powershell
$env:ANTHROPIC_MODEL = "NEW_MODEL_ID"
claude
```

### 永久切换

修改 `settings.json` 中所有模型字段，并把它们统一替换成新的模型 ID，然后完全重启 Claude Code。

## 模型 ID 从哪里获取

打开[老实人AI API 密钥页面](https://laoshirenai.com/keys)，查看当前 Key 的模型列表并复制准确的模型 ID。

也可以从已经设置好 Key 的终端查询。

macOS / Linux / WSL：

```bash
curl -sS 'https://api.laoshirenai.com/v1/models' -H "Authorization: Bearer $ANTHROPIC_AUTH_TOKEN"
```

Windows PowerShell：

```powershell
(Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{ Authorization = "Bearer $env:ANTHROPIC_AUTH_TOKEN" }).data.id
```

macOS / Linux 命令返回 JSON，请从 `data` 数组中复制准确的 `id`。

不要根据模型中文名称猜测，也不要照抄其他分组的模型列表。模型是否可用以当前 Key 的实际授权为准。

## 验证连接

启动 Claude Code 后输入：

```text
请只回复：老实人AI Claude Code 连接成功
```

同时满足以下两项才算配置成功：

1. Claude Code 返回正常结果；
2. 老实人AI的使用记录中出现这次请求。

## 常见问题

| 现象 | 处理方法 |
| --- | --- |
| `claude: command not found` | 重新安装 Claude Code，然后重启终端 |
| 返回 `401` | 检查 `ANTHROPIC_AUTH_TOKEN` 是否为当前有效 Key |
| 提示模型不存在 | 使用当前 Key 模型列表实际显示的模型 ID |
| 仍然调用官方 Claude | 检查 `ANTHROPIC_BASE_URL` 是否为 `https://api.laoshirenai.com`，然后完全重启 |
| 子任务调用其他模型失败 | 确认四个默认模型字段和 `CLAUDE_CODE_SUBAGENT_MODEL` 使用同一个有效模型 ID |

## 安全提示

- `settings.json` 中的 Key 是明文，请不要发送、截图或提交到 Git；
- 不要把 Key 写进项目目录；
- Key 泄露后立即在老实人AI停用并重新创建。

配置字段依据：[Claude Code 环境变量](https://code.claude.com/docs/en/env-vars)、[Claude Code 模型配置](https://code.claude.com/docs/en/model-config)。
