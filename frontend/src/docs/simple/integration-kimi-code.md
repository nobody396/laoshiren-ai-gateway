本文说明如何把老实人AI API Key 配置到 **Kimi Code CLI**。配置完成后，Kimi Code 会通过老实人AI的 OpenAI Chat Completions 接口调用模型。

生产 Base URL：

```text
https://api.laoshirenai.com/v1
```

> 本教程已按 `@moonshot-ai/kimi-code` 0.41.0 核对。国内 npm 镜像安装和配置文件解析均已在隔离目录中验证。

## 准备工作

开始前确认：

1. 已在[老实人AI API 密钥页面](https://laoshirenai.com/keys)创建有效 Key；
2. 当前 Key 有可用额度，并且已经授权目标模型；
3. 如果使用 npm 安装，Node.js 必须为 22.19.0 或更高版本。

检查 Node.js 版本：

```bash
node --version
```

## 安装 Kimi Code

### 官方安装方式

macOS / Linux：

```bash
curl -fsSL https://code.kimi.com/kimi-code/install.sh | bash
```

Windows PowerShell：

```powershell
irm https://code.kimi.com/kimi-code/install.ps1 | iex
```

Windows 首次启动前还需要安装 Git for Windows。

### 国内 npm 镜像安装

macOS / Linux / WSL：

```bash
npm install -g @moonshot-ai/kimi-code@latest --registry=https://registry.npmmirror.com
```

Windows PowerShell：

```powershell
npm.cmd install -g @moonshot-ai/kimi-code@latest --registry=https://registry.npmmirror.com
```

安装后重新打开终端并确认：

```bash
kimi --version
```

## 配置 Kimi Code

### 第一步：查询可用模型

macOS / Linux / WSL：

```bash
export LSRAI_API_KEY="YOUR_API_KEY"
curl -sS 'https://api.laoshirenai.com/v1/models' -H "Authorization: Bearer $LSRAI_API_KEY"
```

Windows PowerShell：

```powershell
$env:LSRAI_API_KEY = "YOUR_API_KEY"
(Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{ Authorization = "Bearer $env:LSRAI_API_KEY" }).data.id
```

从返回结果中复制一个准确的模型 ID，下面用 `YOUR_MODEL_ID` 表示。

### 第二步：修改 config.toml

用户级配置文件位置：

| 系统 | 文件位置 |
| --- | --- |
| macOS / Linux / WSL | `~/.kimi-code/config.toml` |
| Windows | `%USERPROFILE%\.kimi-code\config.toml` |

文件不存在时创建它；已经存在时，把下面的 `lsrai` Provider 和模型合并进去，不要覆盖已有的权限、服务或其他 Provider。

```toml
default_model = "lsrai/YOUR_MODEL_ID"

[providers.lsrai]
type = "openai"
base_url = "https://api.laoshirenai.com/v1"
api_key = "YOUR_API_KEY"

[models."lsrai/YOUR_MODEL_ID"]
provider = "lsrai"
model = "YOUR_MODEL_ID"
max_context_size = 262144
```

替换两类占位内容：

1. 把三处 `YOUR_MODEL_ID` 替换成第一步查到的同一个模型 ID；
2. 把 `YOUR_API_KEY` 替换成老实人AI API Key。

Provider ID 统一使用纯英文小写 `lsrai`。本教程使用 `type = "openai"`，对应 OpenAI Chat Completions；不要改成 `openai_responses`，也不需要添加任何 WebSocket 配置。

保存后检查配置语法：

```bash
kimi doctor
```

看到 `All checked config files are valid.` 说明配置文件格式正确。

### 第三步：启动 Kimi Code

进入项目目录后运行：

```bash
kimi
```

如果只想执行一次测试任务：

```bash
kimi -p '请只回复：lsrai Kimi Code 连接成功' --output-format text
```

## 切换模型

重新查询当前 Key 的模型列表，然后同时修改 `config.toml` 中的：

1. 顶层 `default_model`；
2. `[models."lsrai/YOUR_MODEL_ID"]` 表名；
3. 模型条目中的 `model`。

三处必须使用同一个模型 ID。保存后完全退出并重新启动 Kimi Code。

也可以启动时临时指定已经配置好的模型别名：

```bash
kimi --model "lsrai/YOUR_MODEL_ID"
```

## 验证连接

启动 Kimi Code 后输入：

```text
请只回复：lsrai Kimi Code 连接成功
```

同时满足以下两项才算配置成功：

1. Kimi Code 正常返回结果；
2. 老实人AI的使用记录中出现这次请求。

## 常见问题

| 现象 | 可能原因 | 处理方法 |
| --- | --- | --- |
| `kimi` 命令不存在 | 安装目录还没有进入 PATH | 重新打开终端，再运行 `kimi --version` |
| npm 提示 Node.js 版本过低 | 当前 Node.js 低于 22.19.0 | 升级 Node.js，或改用官方安装脚本 |
| `kimi doctor` 报错 | TOML 引号、表名或字段位置错误 | 对照完整配置修复，不要重复声明同一个 Provider |
| 返回 `401` | Key 错误、已停用或没有正确写入 | 重新复制当前有效 Key |
| 提示模型不存在 | 模型 ID 不属于当前 Key | 重新查询 `/v1/models` 并替换三处模型 ID |
| 工具调用失败 | Provider 协议配置错误 | Kimi 模型保持 `type = "openai"` |

## 安全提示

- `config.toml` 中包含 API Key，不要把它提交到 Git 或发给他人；
- macOS / Linux 可执行 `chmod 600 ~/.kimi-code/config.toml` 限制文件权限；
- Key 泄露后立即在老实人AI停用并重新创建。

配置字段依据：[Kimi Code 官方安装文档](https://www.kimi.com/code/docs/kimi-code-cli/guides/getting-started)、[Provider 与模型配置](https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/providers)、[配置文件说明](https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/config-files.html)。
