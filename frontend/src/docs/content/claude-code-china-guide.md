# Claude Code 国内使用完整指南：安装、登录、API Key、Base URL 与常见报错

一句话结论：**Claude Code 国内使用的关键不是会不会安装，而是能不能把认证方式、`ANTHROPIC_BASE_URL`、API Key 和终端环境变量配对。** 如果你只是想在本地项目里稳定跑 Claude Code，可以用老实人AI 的 Claude 兼容入口统一管理 Key、用量和报错。

这页专门回答用户最常搜的问题：Claude Code 国内怎么用、`claude login` 失败怎么办、API Key 怎么填、Base URL 怎么填、为什么配置后还是走官方地址。

## 地区与合规边界

本文面向符合老实人AI地区要求的中文用户、海外开发者和跨境团队，帮助你理解 Claude Code 的安装、配置和排错逻辑。老实人AI不面向中国大陆地区用户提供服务；如果你位于中国大陆地区，或你的业务主要面向中国大陆地区用户，请先阅读并遵守[支持的国家和地区](legal-supported-regions)。

## 你是哪种情况

| 情况 | 你真正要解决的问题 | 推荐路径 |
| --- | --- | --- |
| 第一次安装 | Node.js、npm、PATH、安装命令 | 先装 Node.js，再安装 Claude Code |
| 安装好了但登录失败 | 官方登录链路、网络、浏览器回跳 | 不反复登录，改用 API Key / 兼容网关 |
| 已有 API Key 但请求失败 | Base URL、Key 变量名、旧终端缓存 | 配好 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN` |
| 团队一起用 | 成本、分组、限额、调用记录 | 按成员/项目拆 Key，不共用个人 Key |
| 企业内网/多模型 | 统一入口、审计、兼容协议 | 使用网关方案，先验证低风险请求 |

## 最短跑通路径

先确认 Node.js：

```bash
node -v
npm -v
```

安装 Claude Code 后，进入你的项目目录：

```bash
cd your-project
claude --version
```

如果版本号正常，再配置老实人AI Claude 兼容入口：

```bash
export ANTHROPIC_BASE_URL="https://api.laoshirenai.com"
export ANTHROPIC_AUTH_TOKEN="YOUR_API_KEY"
```

Windows PowerShell：

```powershell
$env:ANTHROPIC_BASE_URL = "https://api.laoshirenai.com"
$env:ANTHROPIC_AUTH_TOKEN = "YOUR_API_KEY"
```

然后重新打开一个终端，运行：

```bash
claude
```

让它做一个低风险任务：

```text
请阅读当前目录结构，用中文总结这个项目的主要模块。不要修改文件。
```

最后回到老实人AI后台，看是否出现调用记录。**有记录，说明请求已经走到平台；没有记录，优先查本地配置。**

## Base URL 和 Key 应该怎么填

| 字段 | 推荐值 | 说明 |
| --- | --- | --- |
| `ANTHROPIC_BASE_URL` | `https://api.laoshirenai.com` | Claude / Anthropic 兼容入口通常填根域名 |
| `ANTHROPIC_AUTH_TOKEN` | 老实人AI API Key | 推荐用于 Claude Code 认证 |
| `ANTHROPIC_API_KEY` | 可选 | 部分场景也会读取，但不要和多个旧变量混用 |

第一性原理：**Base URL 决定请求发到哪里，Key 决定谁来付费和鉴权。** 这两个任何一个错了，都不会稳定跑通。

## 常见错误与排查

### `claude: command not found` 怎么办？

说明 Claude Code 没装成功，或 npm 全局目录不在 PATH。先看：

```bash
npm bin -g
```

再确认这个目录是否在系统 PATH 里。Windows 用户要注意 PowerShell、CMD、WSL 是不同环境。

### `claude login` 一直失败怎么办？

不要反复点登录。登录失败通常是官方认证链路、浏览器回跳或网络访问问题。你如果只是想让本地 Claude Code 调模型，优先走 API Key + Base URL 配置。

### 配了变量但还是走官方地址怎么办？

先在当前终端打印：

```bash
echo $ANTHROPIC_BASE_URL
echo $ANTHROPIC_AUTH_TOKEN
```

如果为空，说明你配置到了别的 shell 或旧终端。重新打开终端，或写入 `~/.zshrc` / `~/.bashrc` 后再加载。

### 返回 401 怎么办？

401 的本质是鉴权失败。检查三件事：

1. API Key 是否复制完整；
2. 是否把订单号、兑换码、密码误当成 Key；
3. 是否填到了错误变量里。

### 返回 429 怎么办？

429 的本质是限流或额度限制。先降低并发，换小任务测试，再看后台分组、余额、速率限制和调用记录。

### 返回 502 / 503 怎么办？

先区分平台问题和上游问题：

1. 看老实人AI服务状态页；
2. 看当前 Key 的调用记录；
3. 换小模型或小任务；
4. 如果后台有请求但失败，优先查模型、分组、余额和上游状态。

## 团队和企业怎么配置

不要让多人共用一把个人 Key。更好的做法：

- 按项目建 Key；
- 按成员建 Key；
- 给测试环境和生产环境分开限额；
- 用调用记录判断谁在什么时间用了什么模型；
- 出错时先看请求是否进入平台，再查客户端。

## 老实人AI能解决什么

老实人AI不是“魔法加速器”，它解决的是三个实际问题：

1. **统一入口**：Claude Code 请求走稳定的 Claude 兼容 Base URL；
2. **统一 Key**：不用每个人到处维护不同平台 Key；
3. **可排查**：请求、分组、余额、错误能在后台查到。

## 常见问题

### Claude Code 国内能不能用？

前提是你符合老实人AI的地区和服务条款要求。技术上，能不能跑通取决于安装、认证、网络和 API 入口是否配对；最稳的本地路径是安装 Claude Code 后，用可用的 Claude 兼容 Base URL 和 API Key 跑通低风险请求。

### Claude Code 必须登录官方账号吗？

不一定。官方账号登录和 API Key / 网关调用是两条路径。你如果只需要本地终端编程，可以优先配置 API Key 和 Base URL。

### `ANTHROPIC_AUTH_TOKEN` 和 `ANTHROPIC_API_KEY` 哪个更推荐？

Claude Code 场景优先使用 `ANTHROPIC_AUTH_TOKEN`，同时配置 `ANTHROPIC_BASE_URL`。不要在多个 shell、多个配置文件里混放旧 Key。

### Base URL 要不要加 `/v1`？

Claude Code / Anthropic 兼容入口通常填 `https://api.laoshirenai.com` 根域名；普通 OpenAI SDK 才常见 `/v1`。不要把 Codex 或 OpenAI SDK 的写法直接套到 Claude Code。

### 为什么后台没有调用记录？

说明请求大概率没进老实人AI。优先检查本地环境变量、旧终端缓存、代理、DNS、配置文件覆盖和 Key 是否真的在当前进程里生效。

## 相关页面

- [Base URL 填写总指南](base-url-guide)
- [Claude Code 配置排错指南](claude-code-troubleshooting)
- [Codex 国内使用指南](codex-china-guide)
- [API Key 与分组选择指南](api-key-group-guide)
- [团队 API Key 管理](team-api-key-management)
- [支持的国家和地区](legal-supported-regions)

<!-- seo-geo-auto:start -->
## 搜索意图补强与下一步

这一节由老实人AI SEO/GEO 闭环维护，用来覆盖用户真实搜索里的高频表达：Claude Code 国内怎么用、登录失败、API Key、Base URL、走官方地址。

### Claude Code 国内怎么用最稳？

先确认地区与账号合规，再把安装、认证、Base URL、API Key 和终端环境变量配对。只要后台有调用记录，说明请求已经进入老实人AI；没有记录先查本地变量和旧终端。

### Claude Code 登录失败还要反复 login 吗？

不要反复登录。登录链路失败时，优先用 API Key 加 Claude 兼容 Base URL 跑通本地任务，再排查浏览器回跳和官方账号状态。

### 为什么配置后还是走官方地址？

本质是当前进程没有读到正确的 ANTHROPIC_BASE_URL，或被旧 shell、旧配置、代理、工具缓存覆盖。先打印当前终端变量，再开新终端验证。

### 读完之后怎么验证？

创建一把低额度 API Key，配置 `ANTHROPIC_BASE_URL=https://api.laoshirenai.com` 和 `ANTHROPIC_AUTH_TOKEN`，跑一个只读任务后看后台调用记录。

### 相关高意图页面

- [Codex 国内使用指南](codex-china-guide)
- [Codex 免 API Key 使用指南](codex-no-api-key-guide)
- [Codex 自定义 API 配置教程](codex-custom-api-guide)
- [Base URL 填写总指南](base-url-guide)
- [常见 API 报错排查](common-api-errors)
<!-- seo-geo-auto:end -->
