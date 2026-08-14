# 老实人 AI 快速开始指南

## 前置条件：安装 Node.js 环境

Claude Code 依赖 Node.js 运行环境，请先参考 [Node.js 环境安装指南](nodejs-setup) 完成安装并验证。

## 更省事的方式：自动配置

如果你不想手动安装和逐项写配置，推荐直接使用 [自动配置工具](auto-config-tool)。


## 1. 配置 Claude Code

### 第一步：安装 Claude Code

```bash
npm install -g @anthropic-ai/claude-code
```

macOS / Linux 遇到权限问题时加 `sudo`：

```bash
sudo npm install -g @anthropic-ai/claude-code
```

验证安装：

```bash
claude --version
```

---

### 第二步：创建 API Key

登录 [老实人 AI 控制台](https://laoshirenai.com/keys)，进入 **API 密钥** 页面，点击 **创建密钥**。


填写密钥名称，选择分组（模型和倍率），按需配置 IP 限制、额度限制、速率限制和有效期。新手建议直接使用默认配置。

> **安全提示**：API Key 等同于账号凭证，请妥善保管，切勿提交到代码仓库或公开分享。

---

### 第三步：导入密钥到 Claude Code

推荐使用 **CC Switch** 工具进行一键配置，也可手动设置环境变量。

#### 方式一：CC Switch（推荐）

先到本站 [安装与下载](/resources) 页面，选择 **安装 CC Switch**，复制页面生成的命令并在 PowerShell 执行。Windows 安装包会从本站缓存下载并校验 SHA-256，不需要访问 GitHub。安装完成后，回到 API 密钥页点击 **导入 CC Switch**：


导入后点击 **启用** 即可。

如需指定模型，可在 Claude Code 启动时使用 `claude --model opus` 或 `claude --model claude-opus-4-8`。不指定模型时，客户端会按自身默认策略选择模型。

如果 `/model` 里看不到网关提供的 Claude 模型，请先升级 Claude Code 到最新版，并确认配置中包含：

```bash
CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1
```

#### 方式二：手动配置环境变量

点击密钥旁的 **复制** 按钮获取 API Key。


需要设置以下两个环境变量：

| 变量名 | 值 |
|---|---|
| `ANTHROPIC_BASE_URL` | `https://api.laoshirenai.com` |
| `ANTHROPIC_AUTH_TOKEN` | 您的 API Key |

**临时设置（当前终端会话有效）**

Windows (PowerShell)：
```powershell
$env:ANTHROPIC_BASE_URL = "https://api.laoshirenai.com"
$env:ANTHROPIC_AUTH_TOKEN = "YOUR_API_KEY"
```

macOS / Linux：
```bash
export ANTHROPIC_BASE_URL="https://api.laoshirenai.com"
export ANTHROPIC_AUTH_TOKEN="YOUR_API_KEY"
```

**永久设置（重启后依然有效）**

Windows (PowerShell)：
```powershell
[System.Environment]::SetEnvironmentVariable("ANTHROPIC_BASE_URL", "https://api.laoshirenai.com", [System.EnvironmentVariableTarget]::User)
[System.Environment]::SetEnvironmentVariable("ANTHROPIC_AUTH_TOKEN", "YOUR_API_KEY", [System.EnvironmentVariableTarget]::User)
```

macOS (Zsh)：
```bash
echo 'export ANTHROPIC_BASE_URL="https://api.laoshirenai.com"' >> ~/.zshrc
echo 'export ANTHROPIC_AUTH_TOKEN="YOUR_API_KEY"' >> ~/.zshrc
source ~/.zshrc
```

Linux (Bash)：
```bash
echo 'export ANTHROPIC_BASE_URL="https://api.laoshirenai.com"' >> ~/.bashrc
echo 'export ANTHROPIC_AUTH_TOKEN="YOUR_API_KEY"' >> ~/.bashrc
source ~/.bashrc
```

---

### 第四步：开始使用

进入任意项目目录，运行：

```bash
claude
```

Claude Code 会自动分析当前目录的代码并提供智能编程辅助。

---

## 常见问题（FAQ）

**Q：运行 `npm install -g` 提示权限不足？**
A：建议直接使用本站 [安装与下载](/resources) 页面生成的一键命令。Windows 默认安装到当前用户目录，不需要管理员 PowerShell；macOS / Linux 手动使用 npm 时再按系统提示处理权限。

**Q：`claude --version` 提示命令不存在？**
A：确认 npm 全局目录已加入系统 `PATH`，可运行 `npm bin -g` 查看路径并手动添加。

**Q：连接失败或返回 401 错误？**
A：检查 `ANTHROPIC_BASE_URL` 是否为 `https://api.laoshirenai.com`，以及 `ANTHROPIC_AUTH_TOKEN` 是否填写正确且未过期。

**Q：如何切换不同模型或倍率？**
A：在 [老实人 AI 控制台](https://laoshirenai.com/dashboard)) 创建不同分组的密钥，通过 CC Switch 在多个配置间快速切换。
