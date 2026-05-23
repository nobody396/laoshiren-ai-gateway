# Claude Code 国内使用指南

Claude Code 是面向开发者的 AI 编码工具。国内用户配置时，最容易出问题的地方通常不是安装命令，而是 Base URL、API Key、环境变量和终端缓存。

老实人AI提供 Claude 兼容接入入口，适合希望用统一 API Key 管理 Claude Code 调用、查看用量和排查错误的开发者。

## 你需要准备什么

- 已安装 Node.js 和 npm。
- 已安装 Claude Code。
- 已注册老实人AI账号。
- 已创建可用 API Key。

## 推荐配置

| 项目 | 推荐值 |
| --- | --- |
| `ANTHROPIC_BASE_URL` | `https://api.laoshirenai.com` |
| `ANTHROPIC_AUTH_TOKEN` | 你的老实人AI API Key |

macOS / Linux 临时配置：

```bash
export ANTHROPIC_BASE_URL="https://api.laoshirenai.com"
export ANTHROPIC_AUTH_TOKEN="YOUR_API_KEY"
```

Windows PowerShell 临时配置：

```powershell
$env:ANTHROPIC_BASE_URL = "https://api.laoshirenai.com"
$env:ANTHROPIC_AUTH_TOKEN = "YOUR_API_KEY"
```

## 配置完成后怎么验证

1. 新开一个终端窗口，避免旧环境变量缓存。
2. 进入一个测试项目目录。
3. 运行 `claude`。
4. 让它做一个低风险任务，例如解释当前目录结构。
5. 回到老实人AI后台查看是否产生调用记录。

## 常见问题

**Claude Code 还是走官方地址怎么办？**  
检查当前终端里是否真的存在 `ANTHROPIC_BASE_URL`。如果你同时配置了多个工具，建议关闭旧终端后重新打开。

**返回 401 怎么办？**  
通常是 API Key 错误、复制不完整、Key 被删除或填到了错误变量里。先重新复制 Key，再确认变量名是 `ANTHROPIC_AUTH_TOKEN`。

**返回 429 怎么办？**  
可能是频率限制、分组限制或上游限流。先降低并发和请求频率，再查看后台用量与当前分组。

**返回 503 怎么办？**  
可能是上游模型临时不可用、网络异常或路由异常。先查看服务状态页，再换小任务重试。

## 企业团队怎么用

企业团队不要共用一个个人 Key。建议按项目、成员或环境拆分 API Key，方便统计成本、限制额度、排查异常调用。

相关页面：

- [Base URL 填写总指南](base-url-guide)
- [Claude Code 配置排错指南](claude-code-troubleshooting)
- [团队 API Key 管理](team-api-key-management)

