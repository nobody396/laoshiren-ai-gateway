本文说明如何把老实人AI API Key 配置到 **Codex CLI**。配置完成后，Codex 会通过老实人AI的 Responses API 调用模型。

生产 Base URL：

```text
https://api.laoshirenai.com/v1
```

## 准备工作

开始前确认：

1. `codex --version` 可以正常输出版本；
2. 已在[老实人AI API 密钥页面](https://laoshirenai.com/keys)创建有效 Key；
3. 当前 Key 有可用额度，并且已经授权目标模型。

没有安装 Codex 时执行：

```bash
npm install -g @openai/codex@latest
```

### 国内网络环境一键安装并配置

下面使用老实人AI现有的安装配置脚本。它会自动准备 Node.js、安装或更新 Codex CLI、写入老实人AI配置，并验证 API Key。macOS 和 Windows 还会安装或更新 **Codex App**。执行后按照提示粘贴 API Key。

运行前先把 `YOUR_MODEL_ID` 替换成当前 Key 模型列表中的模型 ID。

macOS（Codex App + CLI）：

```bash
curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=0.7.17' | LAOSHIRENAI_TOOLS='codex' LAOSHIRENAI_MODEL_ID='YOUR_MODEL_ID' LAOSHIRENAI_PROTOCOL='responses' LAOSHIRENAI_INSTALL_CODEX_APP='1' bash
```

Windows PowerShell（Codex App + CLI）：

```powershell
$env:LAOSHIRENAI_TOOLS='codex'; $env:LAOSHIRENAI_MODEL_ID='YOUR_MODEL_ID'; $env:LAOSHIRENAI_PROTOCOL='responses'; $env:LAOSHIRENAI_INSTALL_CODEX_APP='1'; irm 'https://laoshirenai.com/auto-config/install.ps1?v=0.7.17' | iex
```

Linux（Codex CLI）：

```bash
curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=0.7.17' | LAOSHIRENAI_TOOLS='codex' LAOSHIRENAI_MODEL_ID='YOUR_MODEL_ID' LAOSHIRENAI_PROTOCOL='responses' bash
```

## 手动配置 Codex

### 第一步：设置 API Key

下面的设置只对当前终端有效。配置完成后，要从同一个终端启动 Codex。

macOS / Linux：

```bash
export LSRAI_API_KEY="YOUR_API_KEY"
```

Windows PowerShell：

```powershell
$env:LSRAI_API_KEY = "YOUR_API_KEY"
```

把 `YOUR_API_KEY` 替换成老实人AI创建的 Key。不要把 Key 写进项目文件或提交到 Git。

### 第二步：修改 config.toml

用户级配置文件位置：

| 系统 | 文件位置 |
| --- | --- |
| macOS / Linux | `~/.codex/config.toml` |
| Windows | `%USERPROFILE%\.codex\config.toml` |

文件不存在时创建它；已经存在时，修改原有的同名字段和 Provider，不要重复追加第二份。

```toml
model = "YOUR_MODEL_ID"
model_provider = "lsrai"

[model_providers.lsrai]
name = "lsrai"
base_url = "https://api.laoshirenai.com/v1"
env_key = "LSRAI_API_KEY"
wire_api = "responses"
requires_openai_auth = false
```

把 `YOUR_MODEL_ID` 替换成当前 Key 模型列表中实际显示的模型 ID。

注意：

- Base URL 必须带 `/v1`，但不要再添加 `/responses`；
- `wire_api` 保持 `responses`；
- 不要添加 `supports_websockets` 或 `responses_websockets_v2`；
- `model` 和 `model_provider` 必须放在文件顶层，不能写进 `[model_providers.lsrai]` 里面；
- Provider ID 和 `name` 统一使用纯英文小写 `lsrai`。

### 第三步：启动 Codex

从刚才设置 `LSRAI_API_KEY` 的同一个终端运行：

```bash
codex
```

本页使用环境变量认证，不修改原有 `auth.json`，也不要求运行官方账号登录。

## 验证连接

启动 Codex 后输入：

```text
请只回复：老实人AI Codex 连接成功
```

同时满足以下两项才算配置成功：

1. Codex 返回正常结果；
2. 老实人AI的使用记录中出现这次请求。

## 切换模型

先查询当前 Key 可以访问的模型。

macOS / Linux：

```bash
curl -sS 'https://api.laoshirenai.com/v1/models' -H "Authorization: Bearer $LSRAI_API_KEY"
```

Windows PowerShell：

```powershell
(Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{ Authorization = "Bearer $env:LSRAI_API_KEY" }).data.id
```

macOS / Linux 命令返回 JSON，请从 `data` 数组中复制准确的 `id`。

修改 `~/.codex/config.toml` 顶部的 `model`：

```toml
model = "NEW_MODEL_ID"
```

模型 ID 必须来自当前 Key 的模型列表。修改后完全退出并重新启动 Codex。

## 常见问题

| 现象 | 可能原因 | 处理方法 |
| --- | --- | --- |
| 返回 `401` | Key 缺失、错误或已经停用 | 重新设置 `LSRAI_API_KEY` |
| 提示未设置环境变量 | Codex 不是从设置 Key 的终端启动 | 在同一终端重新设置 Key 并启动 |
| 提示模型不存在 | 模型 ID 不属于当前 Key | 使用当前 Key 模型列表中的准确 ID |
| 返回 `404` | Base URL 或协议写错 | 使用 `https://api.laoshirenai.com/v1` 和 `responses` |
| 配置没有生效 | 修改了项目配置或重复声明 Provider | 修改用户目录下的 `config.toml` 并删除重复字段 |

## 安全提示

- 不要在聊天、截图、公开文档或代码仓库中暴露 API Key；
- 不要把真实 Key 直接写进 `config.toml`；
- 不同设备建议使用不同 Key，停用设备时同时停用对应 Key；
- Key 泄露后立即在老实人AI停用并重新创建。

配置字段依据：[OpenAI Codex 自定义 Provider](https://learn.chatgpt.com/docs/config-file/config-advanced#custom-model-providers)、[OpenAI Codex 配置参考](https://learn.chatgpt.com/docs/config-file/config-reference)。
