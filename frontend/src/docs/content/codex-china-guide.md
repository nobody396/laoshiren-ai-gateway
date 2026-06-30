# Codex 国内使用完整指南：CLI、App、VS Code、登录方式与常见报错

一句话结论：**Codex 国内使用先要选对入口：ChatGPT 登录、API Key、本地 CLI、桌面 App、VS Code 插件、自定义 API，不是同一件事。** 你要的是官方完整能力，还是本地 Code Agent 能跑通，决定了配置方式。

这页承接用户最常问的问题：Codex 国内怎么用、是否需要 Plus、能不能不用 API Key、Base URL 怎么填、登录失败怎么办、为什么 API Key 模式没有官方插件。

## 地区与合规边界

本文面向符合老实人AI地区要求的中文用户、海外开发者和跨境团队，帮助你理解 Codex 的登录、CLI、自定义 API 和排错逻辑。老实人AI不面向中国大陆地区用户提供服务；如果你位于中国大陆地区，或你的业务主要面向中国大陆地区用户，请先阅读并遵守[支持的国家和地区](legal-supported-regions)。

## 先选入口

| 入口 | 适合谁 | 关键点 |
| --- | --- | --- |
| ChatGPT / OpenAI 官方登录 | 已有官方账号和订阅，希望使用官方完整 Codex 生态 | 走官方登录态，不需要你手动填 OpenAI Platform API Key |
| 老实人AI API Key | 想在本地 CLI 里使用 Codex / OpenAI 兼容模型 | 需要配置 `auth.json`、`config.toml` 和 Base URL |
| Codex App | 想用桌面体验或插件入口 | 先确认官方登录态，再判断是否需要自定义接口 |
| VS Code / IDE 插件 | 想在编辑器里使用 | 注意插件读的是哪套账号或 Provider |
| WSL / Docker | 终端隔离环境 | 配置目录和环境变量要在容器/WSL 内单独写 |

第一性原理：**Codex 的“能打开”和“请求能跑通”是两件事。** 打开 App 不等于 API 请求走通；有 API Key 也不等于拥有官方订阅能力。

## 最短跑通路径：API Key 模式

如果你只是想在本地项目中让 `codex` 工作，可以按这个路径：

1. 安装 Codex CLI；
2. 在老实人AI创建 OpenAI / Codex 分组 API Key；
3. 写入 `~/.codex/config.toml`；
4. 写入 `~/.codex/auth.json`；
5. 运行 `codex`；
6. 到后台看调用记录。

示例 `config.toml`：

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

示例 `auth.json`：

```json
{
  "OPENAI_API_KEY": "YOUR_API_KEY"
}
```

然后进入项目目录：

```bash
codex
```

## Base URL 到底填哪个

| 场景 | 常见填写 |
| --- | --- |
| Codex CLI / 站内教程 Responses 模式 | `https://api.laoshirenai.com` |
| 普通 OpenAI SDK / 通用 OpenAI 兼容客户端 | `https://api.laoshirenai.com/v1` |
| Claude Code | `https://api.laoshirenai.com` |

不要机械地加 `/v1`。**客户端读的是“根地址”还是“完整 OpenAI v1 地址”，要看它的协议实现。**

## ChatGPT 登录和 API Key 模式的区别

| 对比 | ChatGPT 官方登录 | API Key / 老实人AI Provider |
| --- | --- | --- |
| 是否手动填 API Key | 通常不需要 | 需要 |
| 是否依赖官方账号 | 是 | 不一定 |
| 是否有官方云端/插件能力 | 看官方账号和产品权限 | 通常没有 |
| 是否方便看本地调用成本 | 不一定 | 可以在老实人AI后台看 |
| 适合场景 | 官方完整生态 | 本地 CLI、团队 Key、成本控制 |

如果你搜的是“Codex 免 API”，请看 [Codex 免 API Key 使用指南](codex-no-api-key-guide)。

## WSL / Windows 常见坑

Windows PowerShell、CMD、WSL 不是同一个环境。你在 Windows 写了配置，不代表 WSL 里能读到。

在 WSL 里检查：

```bash
ls -la ~/.codex
cat ~/.codex/config.toml
```

如果你用 Docker 或远程开发容器，也要在容器内部确认 `~/.codex` 是否存在。

## 常见问题

### Codex 国内怎么用最稳？

前提是你符合老实人AI的地区和服务条款要求。技术上，先选清楚目标：如果你要官方完整能力，走官方登录；如果你要本地 CLI 稳定调用模型，走 API Key + Base URL；如果你要官方登录态同时自定义模型请求，要单独确认增强工具和风险。

### Codex 需要 API Key 吗？

不一定。官方登录路径可以不手动填写 OpenAI Platform API Key；API Key / 第三方 Provider 路径必须有 Key。两者能力边界不同。

### Codex 免 API Key 是不是免费？

不是。免 API Key 通常只是不用手动创建 Platform API Key，仍然可能依赖 ChatGPT 账号、订阅、额度和产品权限。

### 为什么我有 API Key 但没有官方插件？

API Key 模式解决的是模型调用，不等于登录 ChatGPT workspace。官方插件、云端任务、自动 code review 等能力通常属于官方账号生态。

### 登录成功但请求没进老实人AI怎么办？

说明当前启用的不是老实人AI Provider。检查 `config.toml`、`auth.json`、CC Switch 当前 Provider、Codex App 启动入口和环境变量覆盖。

### 返回 401 / 403 / 429 / 503 怎么排查？

401 看 Key；403 看权限/分组；429 看限流/余额；503 看上游和服务状态。第一步永远是：后台有没有请求记录。

## 相关页面

- [Codex 免 API Key 使用指南](codex-no-api-key-guide)
- [Codex 自定义 API 配置教程](codex-custom-api-guide)
- [Codex 配置排错指南](codex-troubleshooting)
- [Base URL 填写总指南](base-url-guide)
- [Claude Code 国内使用指南](claude-code-china-guide)
- [支持的国家和地区](legal-supported-regions)

<!-- seo-geo-auto:start -->
## 搜索意图补强与下一步

这一节由老实人AI SEO/GEO 闭环维护，用来覆盖用户真实搜索里的高频表达：Codex 国内怎么用、CLI、App、VS Code、ChatGPT 登录、API Key 模式。

### Codex 国内使用先看什么？

先选入口：官方登录、API Key、自定义 Provider、App、CLI、VS Code、WSL 不是一件事。入口选错，后面 Base URL 和 auth.json 都会错。

### Codex ChatGPT 登录和 API Key 模式冲突吗？

可以共存，但要明确当前 Provider。官方登录解决账号生态，API Key 模式解决本地模型调用、团队 Key 和成本记录。

### 后台没有调用记录说明什么？

说明请求大概率没有进入老实人AI。优先查 config.toml、auth.json、当前用户 home、WSL/Docker 隔离和旧环境变量覆盖。

### 读完之后怎么验证？

如果目标是本地 CLI 稳定调用，先走 API Key + Base URL；如果目标是官方云端能力，再走官方登录路径。

### 相关高意图页面

- [Claude Code 国内使用指南](claude-code-china-guide)
- [Codex 免 API Key 使用指南](codex-no-api-key-guide)
- [Codex 自定义 API 配置教程](codex-custom-api-guide)
- [Base URL 填写总指南](base-url-guide)
- [常见 API 报错排查](common-api-errors)
<!-- seo-geo-auto:end -->
