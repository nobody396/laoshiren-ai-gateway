# Dragon Code 自动配置工具

这篇文档解决一件事：用一条命令把 `Claude Code` 或 `Codex` 安装并接到 `Dragon Code`，即使你的机器还没有 Node.js，也可以继续走下去。 

---

## 1. 创建 API Key

脚本执行过程中会提示你输入 API Key，因此先在 Dragon Code 控制台创建好备用。

登录 [Dragon Code 控制台](https://your-domain.example/keys)，进入 **API 密钥** 页面，点击 **创建密钥**。

![](https://assets.example.com/dragoncode/file-20260317102346193.png)
![](https://assets.example.com/dragoncode/file-20260317102505220.png)

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
curl -fsSL https://your-domain.example/auto-config/install.sh | bash
```

脚本会自动完成以下动作：

- 检测当前系统是否已有可用的 Node.js
- 如果没有，则在当前用户目录下安装本地 Node.js 运行时
- 将 npm 镜像切到国内源，降低无代理环境下载失败率
- 安装 `Claude Code` 和 `Codex`
- 写入对应配置文件
- 最后执行版本检查，确认命令可以运行

### Windows PowerShell

直接执行：

```powershell
irm https://your-domain.example/auto-config/install.ps1 | iex
```

---

## 3. 常用参数

如果你想完全非交互执行，可以直接把参数写进命令里。

> **注意：** Claude Code 和 Codex 使用不同的 API Key，配置 `all` 时需要分别提供两个 Key。

### 只配置 Claude Code

macOS / Linux：

```bash
curl -fsSL https://your-domain.example/auto-config/install.sh | bash -s -- --api-key YOUR_CLAUDE_KEY --tools claude
```

Windows PowerShell（管道模式通过环境变量传参）：

```powershell
$env:DRAGON_CLAUDE_API_KEY='YOUR_CLAUDE_KEY'; $env:DRAGON_TOOLS='claude'; irm https://your-domain.example/auto-config/install.ps1 | iex
```

Windows PowerShell（下载后直接执行）：

```powershell
.\install.ps1 --api-key YOUR_CLAUDE_KEY --tools claude
```

### 只配置 Codex

macOS / Linux：

```bash
curl -fsSL https://your-domain.example/auto-config/install.sh | bash -s -- --codex-api-key YOUR_CODEX_KEY --tools codex
```

Windows PowerShell（管道模式）：

```powershell
$env:DRAGON_CODEX_API_KEY='YOUR_CODEX_KEY'; $env:DRAGON_TOOLS='codex'; irm https://your-domain.example/auto-config/install.ps1 | iex
```

Windows PowerShell（下载后直接执行）：

```powershell
.\install.ps1 --codex-api-key YOUR_CODEX_KEY --tools codex
```

### 同时配置 Claude Code 和 Codex

macOS / Linux：

```bash
curl -fsSL https://your-domain.example/auto-config/install.sh | bash -s -- --api-key YOUR_CLAUDE_KEY --codex-api-key YOUR_CODEX_KEY
```

Windows PowerShell（管道模式）：

```powershell
$env:DRAGON_CLAUDE_API_KEY='YOUR_CLAUDE_KEY'; $env:DRAGON_CODEX_API_KEY='YOUR_CODEX_KEY'; irm https://your-domain.example/auto-config/install.ps1 | iex
```

Windows PowerShell（下载后直接执行）：

```powershell
.\install.ps1 --api-key YOUR_CLAUDE_KEY --codex-api-key YOUR_CODEX_KEY
```

### 自定义 API 地址

如果你部署了自定义域名，可以覆盖默认地址：

```bash
curl -fsSL https://your-domain.example/auto-config/install.sh | bash -s -- --api-key YOUR_CLAUDE_KEY --base-url https://your-domain.example.com
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
    "ANTHROPIC_BASE_URL": "https://your-domain.example",
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

[model_providers.OpenAI]
name = "OpenAI"
base_url = "https://your-domain.example"
wire_api = "responses"
requires_openai_auth = true
```

---

## 5. 不需要管理员权限吗

默认不需要。

脚本优先使用系统已有的 Node.js；如果没有，就把 Node.js 安装到当前用户目录：

- macOS / Linux：`~/.dragoncode/node`
- Windows：`%USERPROFILE%\.dragoncode\node`

客户端包也会安装到当前用户目录，而不是系统全局目录。

---

## 6. 首次执行后要做什么

### macOS / Linux

脚本结束后建议执行：

```bash
source ~/.zshrc
```

如果你不是 `zsh`，则按脚本最后输出的实际 profile 文件执行 `source`。

### Windows

重新打开一个 PowerShell 窗口即可。

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

只要命令能输出版本号，通常就说明安装链路已经打通。

---

## 8. 备份与回滚

如果你的机器上已经有旧配置，脚本会在首次覆盖前自动生成 `.bak` 备份，例如：

- `~/.claude/settings.json.bak`
- `~/.codex/auth.json.bak`
- `~/.codex/config.toml.bak`

如果需要回滚，直接把对应 `.bak` 文件恢复回来即可。

---

## 9. 常见问题

### Windows 上提示需要 git-bash

Claude Code 在 Windows 上依赖 git-bash 运行。脚本会自动检测并安装 Git for Windows（优先从国内 npmmirror 镜像下载），无需手动操作。

### Windows 管道模式（`irm | iex`）怎么传参数

`irm ... | iex` 后面**不能直接跟参数**，需要通过环境变量传入：

```powershell
$env:DRAGON_TOOLS='claude'; $env:DRAGON_CLAUDE_API_KEY='YOUR_KEY'; irm https://your-domain.example/auto-config/install.ps1 | iex
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

### API 地址不是 `https://your-domain.example`

使用 `--base-url` 覆盖即可。

### 原来的配置被覆盖了怎么办

优先检查同目录下的 `.bak` 备份文件，脚本第一版已经为配置覆盖预留了回滚路径。

---

## 10. 下一步

- 想看手动安装与逐步解释：继续查看 [Claude Code快速开始指南](claude-code-quickstart) 和 [Codex快速开始指南](codex-quickstart)
- 还没准备好本地环境：查看 [Node.js环境安装指南](nodejs-setup)
- 遇到其他常见问题：查看 [常见问题](faq)
