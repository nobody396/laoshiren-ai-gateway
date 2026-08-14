# 老实人 AI 自动配置工具

这篇文档解决一件事：用一条命令把现有的 `Claude Code` 或 `Codex` 接到 `老实人 AI`；只有客户端确实缺失时，脚本才会自动安装。

---

## 1. 创建 API Key

脚本执行过程中会提示你输入 API Key，因此先在 老实人 AI 控制台创建好备用。

登录 [老实人 AI 控制台](https://laoshirenai.com/keys)，进入 **API 密钥** 页面，点击 **创建密钥**。


填写密钥名称，并根据要使用的工具选择分组：

- 配置 **Claude Code**：选择 Claude 相关分组
- 配置 **Codex**：选择 `codex` 分组
- 同时配置两者：需要分别创建两枚 Key，各自对应上面一种分组

IP 限制、额度限制、速率限制和有效期可按需配置，新手建议直接使用默认配置。

创建完成后，在列表中点击密钥旁的 **复制** 按钮拿到完整的 API Key，稍后在脚本交互中粘贴。

> **安全提示**：API Key 等同于账号凭证，请妥善保管，切勿提交到代码仓库或公开分享。

---

## 2. 推荐用法

### macOS / Linux

直接执行：

```bash
curl -fsSL https://laoshirenai.com/auto-config/install.sh?v=0.7.8 | bash
```

脚本会自动完成以下动作：

- 检测当前系统是否已有可用的 Node.js
- 如果没有，则在当前用户目录下安装本地 Node.js 运行时
- 将 npm 镜像切到国内源，降低无代理环境下载失败率
- 先检测已有的 `Claude Code` / `Codex` CLI，存在且可运行时不重复安装
- 仅在所选客户端缺失时安装对应 CLI
- 写入对应配置文件
- 对 Claude Code 和 Codex 执行 API Key 测试，确认 `/v1/models` 可以正常返回
- 最后执行版本检查，确认命令可以运行

### Windows PowerShell

直接执行：

```powershell
irm https://laoshirenai.com/auto-config/install.ps1?v=0.7.8 | iex
```

Windows 脚本还会检测现有 Claude Code CLI 和官方 `OpenAI.Codex` App。已有客户端时不会再下载 Node.js 或重复安装，只会备份原配置、写入中转配置并测试 API Key。

在控制台的 **API 密钥** 列表里也可以直接点击 **一键配置**：OpenAI 分组会生成 Codex 命令，Anthropic / Antigravity 分组会生成 Claude Code 命令，Windows 和 Mac 会自动显示各自适用的命令。

---

## 3. 常用参数

如果你想完全非交互执行，可以直接把参数写进命令里。

> **注意：** Claude Code 和 Codex 使用不同的 API Key，配置 `all` 时需要分别提供两个 Key。

### 只配置 Claude Code

macOS / Linux：

```bash
curl -fsSL https://laoshirenai.com/auto-config/install.sh?v=0.7.8 | bash -s -- --api-key YOUR_CLAUDE_KEY --tools claude
```

Windows PowerShell（管道模式通过环境变量传参）：

```powershell
$env:LAOSHIRENAI_CLAUDE_API_KEY='YOUR_CLAUDE_KEY'; $env:LAOSHIRENAI_TOOLS='claude'; irm https://laoshirenai.com/auto-config/install.ps1?v=0.7.8 | iex
```

Windows PowerShell（下载后直接执行）：

```powershell
.\install.ps1 --api-key YOUR_CLAUDE_KEY --tools claude
```

### 只配置 Codex

macOS / Linux：

```bash
curl -fsSL https://laoshirenai.com/auto-config/install.sh?v=0.7.8 | bash -s -- --codex-api-key YOUR_CODEX_KEY --tools codex --base-url https://api.laoshirenai.com
```

Windows PowerShell（管道模式）：

```powershell
$env:LAOSHIRENAI_CODEX_API_KEY='YOUR_CODEX_KEY'; $env:LAOSHIRENAI_TOOLS='codex'; $env:LAOSHIRENAI_BASE_URL='https://api.laoshirenai.com'; irm https://laoshirenai.com/auto-config/install.ps1?v=0.7.8 | iex
```

Windows PowerShell（下载后直接执行）：

```powershell
.\install.ps1 --codex-api-key YOUR_CODEX_KEY --tools codex
```

如果你明确需要重新安装最新版 CLI，可在下载脚本后增加 `--force-client-install`。默认不要加这个参数。

### 同时配置 Claude Code 和 Codex

macOS / Linux：

```bash
curl -fsSL https://laoshirenai.com/auto-config/install.sh?v=0.7.8 | bash -s -- --api-key YOUR_CLAUDE_KEY --codex-api-key YOUR_CODEX_KEY
```

Windows PowerShell（管道模式）：

```powershell
$env:LAOSHIRENAI_CLAUDE_API_KEY='YOUR_CLAUDE_KEY'; $env:LAOSHIRENAI_CODEX_API_KEY='YOUR_CODEX_KEY'; irm https://laoshirenai.com/auto-config/install.ps1?v=0.7.8 | iex
```

Windows PowerShell（下载后直接执行）：

```powershell
.\install.ps1 --api-key YOUR_CLAUDE_KEY --codex-api-key YOUR_CODEX_KEY
```

### 自定义 API 地址

如果你部署了自定义域名，可以覆盖默认地址：

```bash
curl -fsSL https://laoshirenai.com/auto-config/install.sh?v=0.7.8 | bash -s -- --api-key YOUR_CLAUDE_KEY --base-url https://api.laoshirenai.com
```

---

## 4. 脚本会写哪些文件

### Claude Code

脚本会写入：

```text
~/.claude/settings.json
```

核心字段如下：

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://api.laoshirenai.com",
    "ANTHROPIC_AUTH_TOKEN": "YOUR_API_KEY",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0"
  }
}
```

### Codex

脚本会写入：

```text
~/.codex/auth.json
~/.codex/config.toml
```

`auth.json`：

```json
{
  "OPENAI_API_KEY": "YOUR_API_KEY"
}
```

`config.toml`：

```toml
model_provider = "OpenAI"
model = "gpt-5.4"
review_model = "gpt-5.4"
model_reasoning_effort = "high"
disable_response_storage = true
network_access = "enabled"
preferred_auth_method = "apikey"

[model_providers.OpenAI]
name = "OpenAI"
base_url = "https://api.laoshirenai.com"
wire_api = "responses"
requires_openai_auth = true
```

---

## 5. 不需要管理员权限吗

默认不需要。

脚本优先使用系统已有的 Node.js；如果没有，就把 Node.js 安装到当前用户目录：

- macOS / Linux：`~/.laoshirenai/node`
- Windows：`%USERPROFILE%\.laoshirenai\node`

缺失的客户端包会安装到当前用户目录，而不是系统全局目录。已经存在的客户端会原样保留。

---

## 6. 首次执行后要做什么

### macOS / Linux

脚本结束后建议执行：

```bash
source ~/.zshrc
```

如果你不是 `zsh`，则按脚本最后输出的实际 profile 文件执行 `source`。

### Windows

如果复用的是 Codex App，请完全退出后重新打开 App；如果使用的是 CLI，重新打开一个 PowerShell 窗口即可。

---

## 7. 如何确认已经成功

### Claude Code

```bash
claude --version
```

### Codex

```bash
codex --version
```

脚本会分别用所选客户端的 API Key 请求 `/v1/models`。看到 `Claude Code API Key 测试通过` 或 `Codex API Key 测试通过`，再加上对应的 `claude --version` / `codex --version` 能输出版本号，才说明配置和命令链路都已经打通。

---

## 8. 备份与回滚

如果你的机器上已经有旧配置，脚本会在首次覆盖前自动生成 `.bak` 备份，例如：

- `~/.claude/settings.json.bak`
- `~/.codex/auth.json.bak`
- `~/.codex/config.toml.bak`

如果需要回滚，直接把对应 `.bak` 文件恢复回来即可。

---

## 9. 保存 OpenAI 官方订阅到 CC Switch

如果你已经在 Codex App 或 Codex CLI 中登录了 ChatGPT/OpenAI 官方订阅，可以把当前本机登录态保存成 CC Switch 的独立 Provider。这样以后即使导入老实人 AI API 服务，也能在 CC Switch 里一键切回官方订阅。

脚本只读写本机文件：

- `~/.codex/auth.json`
- `~/.codex/config.toml`
- `~/.cc-switch/cc-switch.db`

不会把 OpenAI token 上传到 老实人 AI。

macOS / Linux：

```bash
curl -fsSL https://laoshirenai.com/auto-config/save-openai-official-provider.sh?v=1.0.0 | CCS_OPENAI_PROVIDER_NAME="OpenAI Official Pro" bash
```

Windows PowerShell：

```powershell
$env:CCS_OPENAI_PROVIDER_NAME='OpenAI Official Pro'; irm https://laoshirenai.com/auto-config/save-openai-official-provider.ps1?v=1.0.0 | iex
```

执行完成后，重启或打开 CC Switch，在 `Codex` 页面启用 `OpenAI Official Pro` 即可切回官方订阅；要测试老实人 AI 接口服务时，再启用老实人 AI 导入的 Provider。

---

## 10. 常见问题

### 点击“一键导入”后 CC Switch 没有打开

不需要自己判断是版本太旧、便携版还是 Deep Link 损坏。回到 API 密钥页面，点击 **“导入打不开？诊断修复”**，页面会根据 Windows 或 Mac 显示一行命令。

Windows：

1. 按 `Win + R`
2. 输入 `powershell`，按回车
3. 复制并粘贴下面整行命令，按回车

```powershell
irm https://laoshirenai.com/auto-config/diagnose-cc-switch.ps1?v=1.2.3 | iex
```

Mac：

1. 按 `Command（⌘）+ 空格`
2. 输入“终端”或 `Terminal`，按回车
3. 复制并粘贴下面整行命令，按回车

```bash
curl -fsSL 'https://laoshirenai.com/auto-config/diagnose-cc-switch.sh?v=1.2.2' | bash
```

脚本会自动查找 CC Switch、读取版本并修复 `ccswitch://` 协议。如果没有安装，或者版本低于本站缓存的最新版，Windows 会只从老实人 AI 本站缓存获取安装包，核对 SHA-256 后自动安装，不再让中国大陆用户回退到 GitHub。Mac 还会验证开发者签名和 Apple 公证。只有本站缓存不可用、文件校验失败或系统权限不足时，才会打开官方下载页供人工处理。脚本不会读取或上传 API Key。

Windows 绿色便携版会自动升级为当前用户的 MSI 安装版，原有 CC Switch 配置仍保存在用户配置目录中，后续即可使用 CC Switch 自带的自动更新。若 CC Switch 正在系统托盘运行，脚本会先尝试安全关闭；仍未退出时只需要按提示右键退出一次，脚本会继续完成下载、安装和 Deep Link 修复，不需要自己寻找安装包。

### Windows 上提示需要 git-bash

Claude Code 在 Windows 上依赖 git-bash 运行。脚本会自动检测并从老实人 AI 本站缓存下载 Git for Windows，校验 SHA-256 后安装，无需手动操作。

### Windows 管道模式（`irm | iex`）怎么传参数

`irm ... | iex` 后面**不能直接跟参数**，需要通过环境变量传入：

```powershell
$env:LAOSHIRENAI_TOOLS='claude'; $env:LAOSHIRENAI_CLAUDE_API_KEY='YOUR_KEY'; irm https://laoshirenai.com/auto-config/install.ps1?v=0.7.8 | iex
```

如果不传环境变量，脚本会交互式提示输入 API Key。

### 没有代理，脚本还能跑吗

脚本默认优先使用国内镜像：

- Node.js 优先从 `npmmirror` 拉取
- npm registry 默认切到 `https://registry.npmmirror.com`

如果镜像失败，脚本还会再尝试官方源。

### 我只想写配置，不想安装客户端

可以加上：

```bash
--skip-client-install
```

适合你已经装好 `claude` 或 `codex`，只想重新写配置文件的情况。

### API 地址不是 `https://api.laoshirenai.com`

使用 `--base-url` 覆盖即可。

### 原来的配置被覆盖了怎么办

优先检查同目录下的 `.bak` 备份文件，脚本第一版已经为配置覆盖预留了回滚路径。

---

## 11. 下一步

- 想看手动安装与逐步解释：继续查看 [Claude Code快速开始指南](claude-code-quickstart) 和 [Codex快速开始指南](codex-quickstart)
- 还没准备好本地环境：查看 [Node.js环境安装指南](nodejs-setup)
- 遇到其他常见问题：查看 [常见问题](faq)
