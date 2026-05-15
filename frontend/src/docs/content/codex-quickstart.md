# 老实人 AI × Codex 快速开始指南

## 前置条件：安装 Node.js 环境

Codex 依赖 Node.js 运行环境，请先参考 [Node.js 环境安装指南](nodejs-setup) 完成安装并验证。

## 更省事的方式：自动配置

如果你不想手动安装和逐项写配置，推荐直接使用 [自动配置工具](auto-config-tool)。

---

## 1. 安装 Codex

```bash
# Windows
npm install -g @openai/codex@latest

# macOS / Linux
sudo npm install -g @openai/codex@latest
```

验证安装：

```bash
codex --version
```

输出版本号即表示安装成功。

---

## 2. 创建 API Key

登录 [老实人 AI 控制台](https://laoshirenai.com/keys)，进入 **API 密钥** 页面，点击 **创建密钥**。

![](https://assets.example.com/dragoncode/file-20260317102346193.png)
![](https://assets.example.com/dragoncode/file-20260317102505220.png)

填写密钥名称，选择Codex分组（模型和倍率），按需配置 IP 限制、额度限制、速率限制和有效期。新手建议直接使用默认配置。

> **安全提示**：API Key 等同于账号凭证，请妥善保管，切勿提交到代码仓库或公开分享。

---

## 3. 导入密钥到 Codex

推荐使用 **CC Switch** 工具进行一键配置，也可手动创建配置文件。

### 方式一：CC Switch（推荐）

前往 [CC Switch Release](https://github.com/farion1231/cc-switch/blob/main/docs/release-notes/v3.12.2-zh.md) 下载安装后，点击 **导入到 CCS** 完成一键导入：

![](https://assets.example.com/dragoncode/file-20260317102723257.png)
![](https://assets.example.com/dragoncode/file-20260317104909443.png)

导入后点击 **启用** 即可。

### 方式二：手动配置文件

#### 创建配置目录

```powershell
# Windows (PowerShell)
if (Test-Path "$env:USERPROFILE\.codex") { Remove-Item -Recurse -Force "$env:USERPROFILE\.codex" }
mkdir "$env:USERPROFILE\.codex"
```

```bash
# macOS / Linux
rm -rf ~/.codex && mkdir -p ~/.codex
```

#### 创建 `config.toml`

在 `~/.codex/`（Windows 为 `%USERPROFILE%\.codex\`）目录下创建 `config.toml`：

```toml
model_provider = "laoshirenai"
model = "gpt-5.3-codex"
model_reasoning_effort = "high"
disable_response_storage = true
preferred_auth_method = "apikey"

[model_providers.laoshirenai]
name = "laoshirenai"
base_url = "https://api.laoshirenai.com"
wire_api = "responses"
requires_openai_auth = true
```

#### 创建 `auth.json`

点击密钥旁的 **复制** 按钮获取 API Key：

![](https://assets.example.com/dragoncode/file-20260317103239907.png)

在同一目录下创建 `auth.json`，将 `YOUR_API_KEY` 替换为你在控制台创建的密钥：

```json
{
  "OPENAI_API_KEY": "YOUR_API_KEY"
}
```

---

## 4. 开始使用

进入任意项目目录，运行：

```bash
codex
```

Codex 会自动分析当前目录的代码并提供智能编程辅助。更多用法请参考 [OpenAI 官方文档](https://github.com/openai/codex)。

---

## 常见问题（FAQ）

**Q：运行 `npm install -g` 提示权限不足？**
A：macOS / Linux 在命令前加 `sudo`；Windows 使用管理员权限运行 PowerShell。

**Q：`codex --version` 提示命令不存在？**
A：确认 npm 全局目录已加入系统 `PATH`，可运行 `npm bin -g` 查看路径并手动添加。

**Q：连接失败或返回 401 错误？**
A：检查 `config.toml` 中 `base_url` 是否为 `https://api.laoshirenai.com`，以及 `auth.json` 中的 API Key 是否正确且未过期。

**Q：如何切换不同模型或倍率？**
A：在 [老实人 AI 控制台](https://laoshirenai.com/dashboard) 创建不同分组的密钥，更新 `auth.json` 中的 Key，或通过 CC Switch 在多个配置间快速切换。
