本文说明如何把老实人AI API Key 配置到 **Grok Build CLI**。配置完成后，Grok Build 会通过老实人AI的 Responses API 调用模型。

生产 Base URL：

```text
https://api.laoshirenai.com/v1
```

## 准备工作

开始前确认：

1. 已在[老实人AI API 密钥页面](https://laoshirenai.com/keys)创建有效 Key；
2. 当前 Key 有可用额度，并且已经授权目标模型；
3. 本教程配置的是 Grok Build CLI，不是 Grok 网页聊天。

## 安装 Grok Build

macOS / Linux / WSL：

```bash
curl -fsSL https://x.ai/cli/install.sh | bash
```

安装后确认：

```bash
grok --version
```

### 老实人AI一键安装并配置

下面使用老实人AI现有的安装配置脚本。它会安装或更新 Grok Build、写入老实人AI配置并验证 API Key。执行后按照提示粘贴 API Key。

Windows 会从老实人AI同站缓存下载并校验 Grok Build；macOS / Linux / WSL 仍调用 xAI 官方安装源，因此这些系统目前不能标注为“国内镜像直装”。

运行前先把 `YOUR_MODEL_ID` 替换成当前 Key 模型列表中的模型 ID。

macOS / Linux / WSL：

```bash
curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=0.7.17' | LAOSHIRENAI_TOOLS='grok' LAOSHIRENAI_MODEL_ID='YOUR_MODEL_ID' LAOSHIRENAI_PROTOCOL='responses' bash
```

Windows PowerShell：

```powershell
$env:LAOSHIRENAI_TOOLS='grok'; $env:LAOSHIRENAI_MODEL_ID='YOUR_MODEL_ID'; $env:LAOSHIRENAI_PROTOCOL='responses'; irm 'https://laoshirenai.com/auto-config/install.ps1?v=0.7.17' | iex
```

## 手动配置 Grok Build

### 第一步：设置 API Key

macOS / Linux / WSL：

```bash
export LSRAI_API_KEY="YOUR_API_KEY"
```

Windows PowerShell：

```powershell
$env:LSRAI_API_KEY = "YOUR_API_KEY"
```

这些设置只对当前终端有效。配置完成后，要从同一个终端启动 Grok Build。

### 第二步：修改 config.toml

用户级配置文件位置：

| 系统 | 文件位置 |
| --- | --- |
| macOS / Linux / WSL | `~/.grok/config.toml` |
| Windows | `%USERPROFILE%\.grok\config.toml` |

如果设置过 `GROK_HOME`，请修改该目录中的 `config.toml`。已有文件时只合并下面字段，不要覆盖其他设置。

```toml
[models]
default = "lsrai"

[model.lsrai]
model = "YOUR_MODEL_ID"
base_url = "https://api.laoshirenai.com/v1"
name = "lsrai"
env_key = "LSRAI_API_KEY"
api_backend = "responses"
```

把 `YOUR_MODEL_ID` 替换成当前 Key 模型列表中的准确 ID。Provider 条目和 `name` 统一使用纯英文小写 `lsrai`。

### 第三步：检查并启动

```bash
grok inspect
grok
```

`grok inspect` 应当显示用户级配置和 `lsrai` 模型。也可以直接执行：

```bash
grok -p "请只回复：lsrai Grok Build 连接成功" -m lsrai
```

## 查询并切换模型

macOS / Linux / WSL：

```bash
curl -sS 'https://api.laoshirenai.com/v1/models' -H "Authorization: Bearer $LSRAI_API_KEY"
```

Windows PowerShell：

```powershell
(Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{ Authorization = "Bearer $env:LSRAI_API_KEY" }).data.id
```

从返回结果复制准确的模型 ID，然后修改 `[model.lsrai]` 下的 `model`。保存后完全退出并重新启动 Grok Build。

## 验证连接

同时满足以下两项才算配置成功：

1. Grok Build 正常返回结果；
2. 老实人AI的使用记录中出现这次请求。

## 常见问题

| 现象 | 可能原因 | 处理方法 |
| --- | --- | --- |
| 返回 `401` | Key 缺失、错误或已经停用 | 在同一终端重新设置 `LSRAI_API_KEY` |
| 提示模型不存在 | 模型 ID 不属于当前 Key | 查询 `/v1/models` 并使用准确 ID |
| `grok inspect` 看不到配置 | 修改了错误目录 | 检查 `GROK_HOME` 和用户级 `config.toml` |
| 返回 `404` | Base URL 或后端类型错误 | 使用 `/v1` 地址和 `api_backend = "responses"` |
| 启动后仍使用旧模型 | 客户端没有重新读取配置 | 完全退出并重新启动 Grok Build |

## 安全提示

- 不要把真实 Key 写进 `config.toml`；
- 不要在聊天、截图或代码仓库中暴露 Key；
- Key 泄露后立即在老实人AI停用并重新创建。

配置字段依据：[Grok Build 官方设置](https://docs.x.ai/build/settings)、[Grok Build 自定义模型](https://docs.x.ai/build/overview#custom-models)。
