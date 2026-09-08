本文说明如何把老实人AI API Key 配置到 **Antigravity CLI（`agy`）**。配置完成后，Antigravity CLI 会通过老实人AI的 Gemini GenerateContent 接口调用模型。

生产 Base URL：

```text
https://api.laoshirenai.com
```

> 本教程只适用于 Antigravity CLI，不适用于 Antigravity 桌面应用或 IDE 插件。

> 官方安装脚本已在隔离目录中安装验证，当前得到的版本为 `1.1.27`。

## 准备工作

开始前确认：

1. 已在[老实人AI API 密钥页面](https://laoshirenai.com/keys)创建有效 Key；
2. 当前 Key 有可用额度，并且已经授权 Gemini 模型；
3. 准备使用的模型 ID 来自当前 Key 的模型列表。

## 安装 Antigravity CLI

macOS / Linux：

```bash
curl -fsSL https://antigravity.google/cli/install.sh | bash
```

Windows PowerShell：

```powershell
irm https://antigravity.google/cli/install.ps1 | iex
```

Windows CMD：

```cmd
curl -fsSL https://antigravity.google/cli/install.cmd -o install.cmd && install.cmd && del install.cmd
```

安装后重新打开终端并确认：

```bash
agy --version
```

当前还没有老实人AI同站安装镜像。以上安装命令直接使用 Google 官方源，不能标注为国内镜像直装。

## 配置 Antigravity CLI

### 第一步：修改 settings.json

配置文件位置：

| 系统 | 文件位置 |
| --- | --- |
| macOS / Linux | `~/.gemini/antigravity-cli/settings.json` |
| Windows | `%USERPROFILE%\.gemini\antigravity-cli\settings.json` |

文件不存在时创建它；已经存在时，只合并 `modelProvider`，不要覆盖其他设置。

```json
{
  "modelProvider": "gemini"
}
```

`modelProvider` 只能写 `gemini`。只设置 API Key 而不修改这个文件不会生效。

### 第二步：设置 API Key 和 Base URL

Antigravity CLI 不会自动读取项目里的 `.env`，必须在准备启动 `agy` 的终端设置环境变量。

macOS / Linux：

```bash
export GEMINI_API_KEY="YOUR_API_KEY"
export GOOGLE_GEMINI_BASE_URL="https://api.laoshirenai.com"
```

Windows PowerShell：

```powershell
$env:GEMINI_API_KEY = "YOUR_API_KEY"
$env:GOOGLE_GEMINI_BASE_URL = "https://api.laoshirenai.com"
```

这里必须使用 Antigravity CLI 固定读取的 `GEMINI_API_KEY`，不能改成 `LSRAI_API_KEY` 或 `GOOGLE_API_KEY`。

### 第三步：查询模型

macOS / Linux：

```bash
curl -sS 'https://api.laoshirenai.com/v1/models' -H "Authorization: Bearer $GEMINI_API_KEY"
```

Windows PowerShell：

```powershell
(Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{ Authorization = "Bearer $env:GEMINI_API_KEY" }).data.id
```

macOS / Linux 命令返回 JSON，请从 `data` 数组复制准确的 Gemini 模型 `id`。

### 第四步：启动

从刚才设置环境变量的同一个终端运行：

```bash
agy --model "YOUR_MODEL_ID"
```

把 `YOUR_MODEL_ID` 替换成上一步查到的准确模型 ID。进入界面后，标题栏应显示使用 Gemini API Key，而不是 Google 账号邮箱。

## 切换模型

完全退出当前会话，然后使用新的模型 ID 启动：

```bash
agy --model "NEW_MODEL_ID"
```

也可以查看当前客户端能够列出的模型：

```bash
agy models
```

最终仍以当前老实人AI Key 的 `/v1/models` 返回结果为准。

## 验证连接

启动 Antigravity CLI 后输入：

```text
请只回复：lsrai Antigravity CLI 连接成功
```

同时满足以下两项才算配置成功：

1. Antigravity CLI 正常返回结果；
2. 老实人AI的使用记录中出现这次请求。

## 常见问题

| 现象 | 可能原因 | 处理方法 |
| --- | --- | --- |
| 启动后打开 Google 登录 | 没有设置 `modelProvider` | 在用户级 `settings.json` 中设置 `gemini` |
| 提示 `GEMINI_API_KEY` 未设置 | Key 没有进入当前进程 | 在同一终端重新设置 Key 并启动 |
| `.env` 里的 Key 没生效 | Antigravity CLI 不读取 `.env` | 直接设置终端环境变量 |
| 返回 `401` | Key 错误、停用或没有传入 | 重新复制当前有效 Key |
| 提示模型不可用 | 模型 ID 不属于当前 Key | 查询 `/v1/models` 后重新选择 |
| 返回 `404` | Base URL 多写了路径 | 使用根地址，不添加 `/v1` 或 `/v1beta` |

## 恢复 Google 官方登录

如果不再使用老实人AI：

1. 从 `settings.json` 删除 `modelProvider`；
2. 清除 `GEMINI_API_KEY` 和 `GOOGLE_GEMINI_BASE_URL`；
3. 完全退出并重新启动 `agy`。

## 安全提示

- 不要把真实 Key 写进 `settings.json`；
- 不要把 Key 放进项目 `.env`、截图或代码仓库；
- Key 泄露后立即在老实人AI停用并重新创建。

配置字段依据：[Antigravity CLI 官方安装与 API Key 配置](https://www.antigravity.google/docs/cli/install/)。
