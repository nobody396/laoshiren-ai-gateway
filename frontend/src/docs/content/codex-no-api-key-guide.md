# Codex 免 API Key 使用指南：ChatGPT 登录、API Key 登录区别与限制

一句话结论：**Codex“免 API Key”通常指不用手动创建 OpenAI Platform API Key，而是用 ChatGPT / OpenAI 账号登录；它不等于免费、不等于免账号、不等于无限制。** 如果你要的是本地 CLI 可控调用和成本记录，API Key 模式仍然更清晰。

这页专门承接这些搜索：Codex 不用 API Key 可以用吗、Codex 可以用 ChatGPT Plus 登录吗、免 API 是不是免费、API Key 和 ChatGPT 登录有什么区别。

## 地区与合规边界

本文只解释 Codex 登录方式和 API Key 边界，不改变老实人AI的服务地区要求。老实人AI不面向中国大陆地区用户提供服务；如果你位于中国大陆地区，或你的业务主要面向中国大陆地区用户，请先阅读并遵守[支持的国家和地区](legal-supported-regions)。

## 先把三个概念分开

| 概念 | 它是什么 | 常见误解 |
| --- | --- | --- |
| ChatGPT / OpenAI 登录 | 用官方账号完成认证 | 以为登录后就等于所有功能无限制 |
| Platform API Key | OpenAI API 平台的开发者密钥 | 以为所有 Codex 用法都必须先申请它 |
| 第三方 / 老实人AI API Key | 兼容 OpenAI 协议的服务 Key | 以为它等于 ChatGPT 官方订阅 |

第一性原理：**登录解决“你是谁”，API Key 解决“请求由哪个接口和额度承担”。** 这两个可以重叠，但不是一回事。

## 哪些情况可以不手动填 API Key

如果你使用的是官方 Codex 登录路径，通常是：

1. 打开 Codex CLI / App / IDE；
2. 浏览器跳转到 OpenAI / ChatGPT 登录；
3. 登录成功后本地保存认证状态；
4. Codex 使用该登录态访问官方能力。

这条路径下，用户通常不会手动把 `OPENAI_API_KEY` 写进 `auth.json`。这就是很多人说的“免 API Key”。

但它仍然可能受这些因素影响：

- 官方账号是否可用；
- 是否有对应产品权限；
- 是否有订阅或额度限制；
- 网络和浏览器回跳是否正常；
- 公司/学校账号是否限制第三方工具。

## 哪些情况仍然需要 API Key

如果你的目标是下面这些，就需要 API Key 或兼容服务 Key：

- 让本地 Codex CLI 走指定的 OpenAI 兼容接口；
- 用老实人AI后台查看调用记录、余额、分组和错误；
- 按团队/项目拆分 Key；
- 控制速率、额度、模型路由；
- 在 WSL、Docker、服务器里无浏览器运行；
- 使用自定义 Base URL 或中转接口。

## 两种方式怎么选

| 目标 | 推荐方式 |
| --- | --- |
| 想体验官方完整 Codex 生态 | ChatGPT / OpenAI 官方登录 |
| 想在本地项目里稳定跑命令行任务 | API Key + Base URL |
| 想看清楚每次调用成本和错误 | 老实人AI API Key |
| 想团队统一管理模型入口 | 老实人AI / 网关方案 |
| 想不用浏览器、在服务器跑 | API Key 模式 |
| 想用官方插件、云端任务、自动 code review | 官方登录路径 |

## API Key 模式不是低级方案

很多用户以为“免 API Key”更高级。其实不是。API Key 模式的优势是：

- 配置可复制；
- 适合自动化和服务器；
- 容易排查；
- 能看调用记录；
- 可以做团队成本控制；
- 不依赖浏览器回跳。

它的边界是：**API Key 模式通常只解决模型调用，不承诺官方账号生态里的云端功能。**

## 老实人AI推荐的安全做法

1. 个人测试：可以先用一把低额度 Key；
2. 团队使用：按项目或成员拆 Key；
3. 不要把 Key 写进代码仓库；
4. 不要把 Key 发给客服或群聊；
5. 配置后先跑低风险任务；
6. 有问题先看后台有没有请求记录。

## 常见问题

### Codex 不用 API Key 可以用吗？

可以走官方账号登录路径，但这不代表免费或无限制。它依赖你的 OpenAI / ChatGPT 账号、权限、订阅和当前产品策略。

### Codex 可以用 ChatGPT Plus 登录吗？

如果官方当前向你的账号开放对应 Codex 能力，就可以通过官方登录路径使用；具体能力和额度以 OpenAI 当前账号页面为准。

### 免 API Key 会不会扣 API 费用？

通常不会走你手动创建的 Platform API Key，但仍然可能消耗官方账号对应的订阅权益、额度或产品配额。不要把“免 Key”理解成“无限免费”。

### API Key 登录和 ChatGPT 登录哪个更稳？

看场景。本地自动化、团队成本、服务器环境通常 API Key 更稳；官方云端能力和插件生态通常官方登录更合适。

### 我能不能同时保留官方登录和老实人AI API Key？

可以，但要用工具或配置明确切换当前 Provider。不要让 `default`、旧环境变量和多个配置文件互相覆盖。

### 为什么别人说“Codex 免 API”，我这里还要填 Key？

因为你们说的可能不是同一条路径。别人可能走官方登录；你如果配置自定义 Base URL 或第三方兼容接口，就需要对应 Key。

## 相关页面

- [Codex 国内使用指南](codex-china-guide)
- [Codex 自定义 API 配置教程](codex-custom-api-guide)
- [Codex 配置排错指南](codex-troubleshooting)
- [API Key 与分组选择指南](api-key-group-guide)
- [支持的国家和地区](legal-supported-regions)

<!-- seo-geo-auto:start -->
## 搜索意图补强与下一步

这一节由老实人AI SEO/GEO 闭环维护，用来覆盖用户真实搜索里的高频表达：Codex 免 API Key、ChatGPT 登录、是否免费、Plus 登录、API Key 边界。

### Codex 免 API Key 是不是免费？

不是。免 API Key 只是不用手动创建 Platform API Key，仍然依赖官方账号、订阅、额度、产品权限和网络回跳。

### 什么时候可以不填 API Key？

当你走官方 ChatGPT/OpenAI 登录路径，并且账号拥有对应 Codex 能力时，通常不需要手动把 OPENAI_API_KEY 写入 auth.json。

### 什么时候必须要 API Key？

只要你要自定义 Base URL、走老实人AI后台记录、团队成本控制、WSL/Docker/服务器运行，就需要 API Key 或兼容服务 Key。

### 读完之后怎么验证？

先判断目标是“官方完整生态”还是“本地 CLI 可控调用”。前者看官方账号权限，后者用 API Key 模式更清晰。

### 相关高意图页面

- [Claude Code 国内使用指南](claude-code-china-guide)
- [Codex 国内使用指南](codex-china-guide)
- [Codex 自定义 API 配置教程](codex-custom-api-guide)
- [Base URL 填写总指南](base-url-guide)
- [常见 API 报错排查](common-api-errors)
<!-- seo-geo-auto:end -->
