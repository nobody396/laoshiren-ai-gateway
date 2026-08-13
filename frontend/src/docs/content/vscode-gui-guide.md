# 老实人 AI × Claude Code VS Code 图形化操作教程


## 前置条件

- 已完成 [Node.js 环境安装](nodejs-setup)
- 已安装 Claude Code（`npm install -g @anthropic-ai/claude-code`）

---

## 1. 安装 VS Code

前往 [VS Code 官网](https://code.visualstudio.com/Download) 下载对应平台的安装包。

Mac 用户注意选择正确的芯片架构：

- **ARM64**：M1 / M2 / M3 / M4 芯片
- **x64**：Intel 芯片

---

## 2. 安装 Claude Code for VS Code 插件

打开 VS Code，点击左侧 **扩展市场**（Extensions），搜索 `Claude Code for VS Code`，认准发布者为 **Anthropic**，点击 **安装**。


安装完成后，打开任意项目，点击右上角的 **Claude Code 图标** 打开插件面板。


---

## 3. 创建 API Key

登录 [老实人 AI 控制台](https://laoshirenai.com/keys)，进入 **API 密钥** 页面，点击 **创建密钥**。


填写密钥名称，选择分组（模型和倍率），按需配置 IP 限制、额度限制、速率限制和有效期。新手建议直接使用默认配置。

> **安全提示**：API Key 等同于账号凭证，请妥善保管，切勿提交到代码仓库或公开分享。

---

## 4. 导入密钥

推荐使用 **CC Switch** 工具进行一键配置，也可手动设置环境变量。

### 方式一：CC Switch（推荐）

先到本站 [安装与下载](/resources) 页面，选择 **安装 CC Switch**，复制页面生成的命令并在 PowerShell 执行。Windows 安装包会从本站缓存下载并校验 SHA-256，不需要访问 GitHub。安装完成后，回到 API 密钥页点击 **导入 CC Switch**：


导入后点击 **启用** 即可。

### 方式二：手动配置环境变量

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

## 5. 跳过官方登录

首次打开插件可能出现 Anthropic 官方登录页面，显示三种登录方式。

**不要选择任何一种**——等待 3-5 秒页面会自动消失。如果已按上一步配置好环境变量，插件会自动使用 老实人 AI 的 API Key，无需登录。


如果登录页面没有自动消失，左下角点击设置，然后搜索Claude code，勾选 **Disable Login Prompt** 手动关闭。


配置成功后界面如下：


---

## 6. 开始使用

在输入框中用自然语言描述需求，通过对话形式与 Claude Code 交互，插件会自动读取和修改当前项目的代码。


---

## 7. 使用技巧

### 三种操作模式

左下角可切换操作模式：


**1. Ask before edits（编辑前询问）**

最保守的模式。Claude 每一次修改都会先展示变更内容，征求你的确认后才会写入文件。适合刚上手不熟悉工具时使用，或在修改关键代码时需要逐步把关的场景。

**2. Edit automatically（自动编辑）**

最高自动化的模式。Claude 会直接修改代码文件，不再逐步确认。适合需求已经明确、希望快速落地修改的场景。需要注意改动可能超出预期，建议事后检查 Git diff 确认变更范围。

**3. Plan mode（规划模式）**

不直接改代码，而是先扫描项目结构，给出完整的修改方案和执行步骤，等你确认后再执行。适合复杂任务，比如代码重构、多文件联动改动、架构调整等。先看方案再动手，避免大规模改动失控。

### 多窗口并行开发

点击右上角 **+** 号可开启多个 Claude Code 窗口，例如一个负责开发、一个负责测试。


### 引用项目文件

在输入框中输入 `@` 后选择文件名，可将文件内容作为上下文传递给 Claude Code。


### 上传文件

输入 `/attach file`，从文件选择器中上传文件。


> **注意**：右下角的附件图标是 **引用文件**（添加上下文），不是上传文件，注意区分。

### 切换模型

输入 `/model`，选择 **Switch Model**，按需切换模型。


---

## 常见问题（FAQ）

**Q：插件安装后找不到 Claude Code 图标？**
A：确认插件已安装成功，尝试重启 VS Code。图标位于编辑器右上角工具栏。

**Q：登录页面一直不消失？**
A：在对话框输入 `/config`，勾选 `Disable Login Prompt` 关闭登录提示。

**Q：插件提示连接失败或 401 错误？**
A：检查 `ANTHROPIC_BASE_URL` 是否为 `https://api.laoshirenai.com`，以及 `ANTHROPIC_AUTH_TOKEN` 是否填写正确且未过期。

**Q：如何切换不同模型或倍率？**
A：在 [老实人 AI 控制台](https://laoshirenai.com/dashboard) 创建不同分组的密钥，通过 CC Switch 在多个配置间快速切换。

**Q：VS Code 应该选哪个版本？**
A：Mac 用户根据芯片选择：M 系列芯片选 ARM64，Intel 芯片选 x64。Windows 用户直接下载默认版本即可。
