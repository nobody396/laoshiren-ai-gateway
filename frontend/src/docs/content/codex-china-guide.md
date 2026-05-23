# Codex 国内使用指南

Codex 常见接入方式更接近 OpenAI 兼容协议。国内用户最常遇到的问题是 Provider 没配对、Base URL 多写或少写 `/v1`、本地旧配置覆盖新配置、WSL 和宿主机环境变量不一致。

老实人AI可以作为 Codex 的 OpenAI 兼容接入入口，帮助你统一管理 API Key、模型分组、调用记录和成本。

## 先确认你的运行环境

- Codex CLI、Codex App 或其他 Codex 兼容客户端。
- macOS、Windows、WSL 或 Linux。
- 是否使用 OpenAI 兼容 Provider。
- 是否需要在 WSL 和 Windows 两边分别配置。

## Base URL 怎么填

多数 OpenAI 兼容工具使用：

```text
https://api.laoshirenai.com/v1
```

但不同客户端字段命名不完全一致，可能叫 `base_url`、`baseURL`、`api_base`、`OpenAI Base URL`。如果客户端文档要求填写根地址，再按客户端要求调整。

## API Key 怎么填

在老实人AI后台创建 API Key 后，复制到 Codex 的 Provider 配置里。不要把订单号、兑换码、登录密码当成 API Key。

## WSL 用户注意

Windows PowerShell 里的环境变量不会自动等于 WSL 里的环境变量。你在 Windows 配好了 Codex，不代表 WSL 里也能直接用。

WSL 里建议单独检查：

```bash
echo $OPENAI_API_KEY
echo $OPENAI_BASE_URL
```

如果为空，需要在 WSL 的 shell 配置里重新写入。

## 常见问题

**Codex 反复登录怎么办？**  
如果你走的是第三方 OpenAI 兼容接口，应优先确认 Provider 配置，而不是反复走官方登录流程。

**连接失败怎么办？**  
先确认 Base URL 是否为 `https://api.laoshirenai.com/v1`，再确认本机代理、DNS、终端环境变量和客户端配置文件。

**模型不可用怎么办？**  
检查 API Key 绑定的分组是否支持目标模型。分组不同，模型、倍率和路由能力也可能不同。

**请求有费用但结果不符合预期怎么办？**  
查看调用记录中的模型、输入、输出和错误信息，先确认请求实际走到了哪个模型。

相关页面：

- [Codex 配置排错指南](codex-troubleshooting)
- [API Key 与分组选择指南](api-key-group-guide)
- [常见 API 报错排查](common-api-errors)

