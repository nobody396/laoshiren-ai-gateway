# Codex 自定义 API 配置教程：config.toml、auth.json、Base URL 与常见错误

一句话结论：**Codex 自定义 API 的核心是三件事：`config.toml` 选 Provider，`auth.json` 放 Key，Base URL 和 `wire_api` 要和客户端协议匹配。** 配错一个字段，表现就会变成反复登录、401、404、请求没进平台或模型不可用。

这页承接用户最常搜的问题：Codex 怎么配置第三方 API、Base URL 怎么填、config.toml 怎么写、auth.json 放哪里、Responses 模式是什么、为什么 openai_base_url 不生效。

## 地区与合规边界

本文只解释 Codex 自定义 API 的配置和排错逻辑，不改变老实人AI的服务地区要求。老实人AI不面向中国大陆地区用户提供服务；如果你位于中国大陆地区，或你的业务主要面向中国大陆地区用户，请先阅读并遵守[支持的国家和地区](legal-supported-regions)。

## 适合谁

- 想让 Codex CLI 走老实人AI；
- 想用团队统一 API Key；
- 想在后台看调用记录和成本；
- 想在 WSL / Docker / 服务器里运行；
- 想切换多个 Provider；
- 不想每次都走浏览器官方登录。

如果你只是想用官方账号登录，不想碰 API Key，请先看 [Codex 免 API Key 使用指南](codex-no-api-key-guide)。

## 推荐配置结构

Codex 常见读取目录：

```text
~/.codex/
  config.toml
  auth.json
```

Windows 通常是：

```text
%USERPROFILE%\.codex\
  config.toml
  auth.json
```

WSL、Docker、远程服务器都有自己的 home 目录，不能直接复用宿主机配置。

## config.toml 示例

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

字段解释：

| 字段 | 作用 |
| --- | --- |
| `model_provider` | 当前使用哪个 Provider |
| `model` | 默认模型名 |
| `preferred_auth_method` | 优先使用 API Key 认证 |
| `base_url` | 请求发送到哪里 |
| `wire_api` | 使用 Responses 还是 Chat Completions 协议 |
| `requires_openai_auth` | 告诉客户端需要 OpenAI 风格认证字段 |

## auth.json 示例

```json
{
  "OPENAI_API_KEY": "YOUR_API_KEY"
}
```

不要把订单号、兑换码、登录密码放进这里。API Key 泄露后别人可以消耗你的额度。

## Base URL 怎么判断

| 客户端/场景 | 推荐写法 |
| --- | --- |
| Codex CLI + Responses 模式 | `https://api.laoshirenai.com` |
| 普通 OpenAI SDK | `https://api.laoshirenai.com/v1` |
| 通用 OpenAI 兼容工具要求完整 v1 地址 | `https://api.laoshirenai.com/v1` |
| Claude Code | `https://api.laoshirenai.com` |

判断方法很简单：**如果客户端自己会拼 `/v1/responses` 或协议路径，就不要手动多加一层；如果客户端明确要求 OpenAI Base URL，则通常填 `/v1`。**

## 验证是否生效

运行：

```bash
codex
```

然后让它执行一个只读任务：

```text
请总结当前项目目录结构，不要修改文件。
```

判断顺序：

1. 老实人AI后台是否出现调用记录；
2. 记录里的模型是否符合预期；
3. 如果失败，错误码是什么；
4. 如果后台没有记录，说明本地配置没生效。

## 常见错误

### `openai_base_url` 不生效怎么办？

说明当前 Codex 版本或配置方式不读取这个字段。以站内教程的 `model_providers.<name>.base_url` 为准，不要把其他 SDK 的环境变量机械套过来。

### 为什么填 `/v1` 后 404？

可能是客户端又自动拼了一层协议路径，导致路径重复。Codex CLI + Responses 模式优先用根地址 `https://api.laoshirenai.com`。

### 为什么一直要求登录？

说明当前启用的不是 API Key Provider，或者 `preferred_auth_method`、`auth.json`、Provider 名称没有对上。检查 `model_provider` 是否等于你定义的 provider 名称。

### 为什么后台没有请求记录？

说明请求没进老实人AI。优先查配置文件路径、当前用户 home、WSL/Docker 隔离、CC Switch 当前 Provider、旧环境变量覆盖。

### 401 怎么办？

Key 错了、Key 不完整、Key 被删除、填成了订单号，或者当前配置没有读到 `auth.json`。

### 429 怎么办？

余额、速率、分组或上游限制。降低并发，换小任务，查看后台分组和调用记录。

### 模型不可用怎么办？

模型名和 Key 分组要匹配。不同分组支持的模型、倍率、路由能力可能不同。

## 团队使用建议

- 每个项目一把 Key；
- 每个成员一把 Key；
- 测试 Key 设置低额度；
- 生产 Key 限 IP、限速、限额；
- 发生异常先禁用对应 Key；
- 不把 `auth.json` 提交到 Git。

## 常见问题

### Codex 可以接第三方 API 吗？

可以接兼容 OpenAI / Responses 协议的接口，但能力边界取决于客户端、协议、模型和网关兼容性。先用小任务验证，不要直接跑大项目改写。

### Codex 自定义 API 和官方登录能同时存在吗？

可以，但要明确当前启用哪个 Provider。建议用 CC Switch 或独立备份保存不同配置，避免旧配置互相覆盖。

### Codex 自定义 API 能不能获得官方插件能力？

通常不能。自定义 API 解决的是模型请求路由；官方插件、云端任务、自动 code review 等能力属于官方账号生态。

### `auth.json` 应该放在哪里？

放在当前运行 Codex 的用户 home 下的 `.codex` 目录。WSL、Docker、远程服务器要分别配置。

### 老实人AI怎么帮我排错？

只要请求进了平台，后台就能看到调用记录、模型、分组、错误和用量。没有记录，先查本地配置；有记录，再查平台和上游。

## 相关页面

- [Codex 国内使用指南](codex-china-guide)
- [Codex 免 API Key 使用指南](codex-no-api-key-guide)
- [Codex 配置排错指南](codex-troubleshooting)
- [Base URL 填写总指南](base-url-guide)
- [API Key 与分组选择指南](api-key-group-guide)
- [支持的国家和地区](legal-supported-regions)

<!-- seo-geo-auto:start -->
## 搜索意图补强与下一步

这一节由老实人AI SEO/GEO 闭环维护，用来覆盖用户真实搜索里的高频表达：Codex 第三方 API、config.toml、auth.json、Base URL、Responses 模式、/v1。

### Codex 自定义 API 最容易错在哪里？

错在把 SDK 的 /v1 写法、旧 openai_base_url 字段、官方登录态和 Provider 配置混在一起。Codex CLI 要看当前版本读取哪个字段。

### Base URL 到底要不要加 /v1？

Codex CLI + Responses 模式通常填根地址 `https://api.laoshirenai.com`；普通 OpenAI SDK 或明确要求 v1 的客户端才填 `/v1`。

### 为什么一直要求登录或 401？

说明当前没有启用 API Key Provider，或 auth.json 没被当前用户/WSL/容器读到，也可能 Key 填错、填成订单号或被删除。

### 读完之后怎么验证？

先固定一套 `model_provider`、`base_url`、`wire_api`、`auth.json`，用只读任务验证后台调用记录，再扩展到大项目。

### 相关高意图页面

- [Claude Code 国内使用指南](claude-code-china-guide)
- [Codex 国内使用指南](codex-china-guide)
- [Codex 免 API Key 使用指南](codex-no-api-key-guide)
- [Base URL 填写总指南](base-url-guide)
- [常见 API 报错排查](common-api-errors)
<!-- seo-geo-auto:end -->
