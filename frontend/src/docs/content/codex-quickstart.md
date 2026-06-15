# 老实人 AI × Codex 快速开始指南

## 前置条件：安装 Node.js 环境

Codex 依赖 Node.js 运行环境，请先参考 [Node.js 环境安装指南](nodejs-setup) 完成安装并验证。

## 更省事的方式：自动配置

如果你不想手动安装和逐项写配置，推荐直接使用 [自动配置工具](auto-config-tool)。

## 先搞清楚：API Key、官方登录和 Codex++ 不是同一件事

Codex 现在有三种常见启动方式：

| 启动方式 | 适合谁 | 能用什么 |
|---------|--------|----------|
| **老实人 AI API Key / 第三方 API** | 想通过老实人 AI API 服务使用更可控的 token 额度 | 本地终端里的 `codex`、读写文件、执行命令、普通本地任务 |
| **OpenAI 官方订阅登录** | 已有 ChatGPT Plus / Pro / Business / Enterprise，想用官方完整 Codex 生态 | 本地 Codex、Codex App / IDE、云端任务、官方插件、自动 code review、Slack / GitHub 等云端集成 |
| **Codex++** | 已经安装并登录 Codex App，又想保留插件入口，同时把模型请求转到兼容 API | 通过第三方外部启动器启动 Codex App，保留官方账号能力，并可选开启自定义接口注入 |

简单说：**老实人 AI API 服务可以让 Codex 在本地干活，但不等于登录了 ChatGPT 官方订阅。** 只依赖 API Key 时，和 ChatGPT workspace、Codex cloud、官方插件目录、云端自动化相关的能力可能不可用或受限。

`Codex++` 是一个第三方开源增强工具，不是 OpenAI 官方产品，也不是一个新模型。它的思路是：先让 Codex App 保持 ChatGPT/OpenAI 官方登录态，官方账号继续负责插件入口和账号能力；再由 Codex++ 外部启动器注入增强脚本，可选把模型请求切到自定义兼容 API。

如果你的目标只是“像官方订阅一样使用完整功能”，优先在 CC Switch 里添加并启用 `OpenAI Official`。如果你的目标是“官方登录态 + 模型请求走自定义接口 + Codex App 增强”，再考虑使用 Codex++。

---

## 1. 安装 Codex

如果你要安装 **Codex App for Windows 桌面版**，不要用下面的 npm 命令，请先看 [Codex App Windows 下载](codex-app-windows-download)。

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


填写密钥名称，选择Codex分组（模型和倍率），按需配置 IP 限制、额度限制、速率限制和有效期。新手建议直接使用默认配置。

> **安全提示**：API Key 等同于账号凭证，请妥善保管，切勿提交到代码仓库或公开分享。

---

## 3. 导入密钥到 Codex

推荐使用 **CC Switch** 工具进行一键配置，也可手动创建配置文件。

### 方式一：CC Switch（推荐）

前往 [CC Switch Release](https://github.com/farion1231/cc-switch/releases/latest) 下载安装后，点击 **导入到 CCS** 完成一键导入：


导入后点击 **启用** 即可。

#### 官方订阅和接口服务同时放进 CC Switch

如果你既有 OpenAI 官方订阅，又要测试 老实人 AI API 服务，请在 CC Switch 里保留两类 Provider：

- **官方订阅 / 完整功能入口**：在 CC Switch 顶部切到 `Codex`，点击 `+`，选择 `OpenAI Official`，按提示登录你的 ChatGPT/OpenAI 账号。这个 Provider 才代表官方订阅入口，适合需要 Codex 云端任务、官方插件、自动 code review、Slack / GitHub 等云端集成的人。
- **老实人 AI API 服务 / 本地 API 入口**：在 老实人 AI 的 API 密钥页面，给不同分组分别创建 Key，例如 `OpenAI Plus 测试`、`OpenAI Pro 测试`，再分别点击 **导入到 CCS**。这个入口适合本地 `codex` 终端任务和可控 token 消耗。
- **切换方式**：回到 CC Switch 的 `Codex` 页面，在 Provider 列表里启用你要用的那一个。不要依赖 `default` 当官方入口；`default` 只是当前配置快照，若之前被接口服务覆盖过，内容可能已经不是官方订阅。

导入到 CCS 的老实人 AI Provider 名称会带上站点名、工具名、分组名和密钥名，便于区分，例如 `老实人 AI - Codex - OpenAI Pro - Pro 测试 Key`。

#### Codex++ 是什么情况

如果你使用的是 [BigPizzaV3/CodexPlusPlus](https://github.com/BigPizzaV3/CodexPlusPlus)，请注意它和 CC Switch 的定位不同：

- CC Switch 主要负责管理和切换本机 Codex / Claude Code 的 Provider 配置。
- Codex++ 是 Codex App 的外部增强启动器，通过 `Codex++` 入口启动原版 Codex App，并注入增强功能。
- Codex++ 的自定义接口注入适合“已经在 Codex App 登录官方账号，但希望模型请求走自定义兼容 API”的场景。
- 使用 Codex++ 时，仍然建议先确认 Codex App 已检测到 ChatGPT 官方登录状态，再添加接口服务配置。

如果你已经在 Codex App 或 Codex CLI 里登录过官方订阅，也可以用脚本把当前本机登录态保存成独立 Provider。脚本只读写本机 `~/.codex` 和 `~/.cc-switch`，不会把 OpenAI token 上传到 老实人 AI。

macOS / Linux：

```bash
curl -fsSL https://laoshirenai.com/auto-config/save-openai-official-provider.sh | CCS_OPENAI_PROVIDER_NAME="OpenAI Official Pro" bash
```

Windows PowerShell：

```powershell
$env:CCS_OPENAI_PROVIDER_NAME='OpenAI Official Pro'; irm https://laoshirenai.com/auto-config/save-openai-official-provider.ps1 | iex
```

执行完成后，重启或打开 CC Switch，在 `Codex` 页面会看到 `OpenAI Official Pro`，之后就可以在官方订阅和 老实人 AI API 服务之间切换。

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

**Q：为什么我用 API Key 后没有官方插件、云端任务或自动化？**
A：这是正常的。API Key 模式主要解决本地 Codex 调用模型的问题；需要 ChatGPT workspace、Codex cloud、官方插件和云端自动化时，请切到 `OpenAI Official` 并用 ChatGPT 官方账号登录。

**Q：Codex++ 能解决这个问题吗？**
A：它解决的是另一条路径：用 Codex++ 启动 Codex App，让官方登录态继续负责插件入口和账号能力，再可选把模型请求转到兼容 API。它是第三方工具，不是 OpenAI 官方功能；使用前请确认你信任该工具，并保留 `~/.codex` 配置备份。
