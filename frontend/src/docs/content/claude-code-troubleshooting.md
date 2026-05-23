# Claude Code 配置排错指南

这页用于排查 Claude Code 接入老实人AI后无法连接、配置不生效或仍然走官方登录的问题。

## 先确认四件事

| 检查项 | 正确状态 |
| --- | --- |
| Base URL | `https://api.laoshirenai.com` |
| API Key | 老实人AI后台创建的完整 Key |
| 分组 | 支持 Claude Code/Anthropic 协议 |
| 测试方式 | 新开终端或新会话，用最短问题测试 |

## 常见原因

- Base URL 错填成 `https://api.laoshirenai.com/v1`。
- API Key 不完整，或者把订单号、兑换码填进客户端。
- 当前 Key 的分组不支持 Claude Code。
- 旧终端窗口还保留旧环境变量。
- 之前配置过其他 Claude/Anthropic 服务，环境变量冲突。
- 网络代理、VPN 或系统代理影响连接。

## 排查步骤

1. 在老实人AI后台确认 API Key 和分组。
2. 在 Claude Code 当前配置里确认实际 Base URL 和 Token。
3. 清理旧的 Claude/Anthropic 环境变量。
4. 关闭旧 Claude Code 和终端窗口。
5. 新开终端，用一句短问题测试。
6. 到老实人AI使用记录里确认请求是否进入平台。

## 什么时候联系客服

如果你已经确认地址、Key、分组、旧窗口和网络环境都正确，但使用记录里仍然没有请求，或请求进入平台后持续失败，可以带上报错时间、入口工具、模型名和打码截图联系客服。不要发送完整 API Key。

