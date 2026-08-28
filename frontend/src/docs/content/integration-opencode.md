# OpenCode

**协议：** OpenAI / Anthropic / Gemini 兼容

**配置状态：** 即将支持一次性配置

OpenCode 需要保存凭据，并在用户级 `opencode.json` 中声明 Provider 和模型。现有页面可以生成配置片段，但一次性配置、备份、回滚和真实系统测试尚未完成。

发布前必须完成：

1. Key 通过安全凭据入口保存，不写入项目配置。
2. Provider 只合并老实人AI 项，不覆盖其他 Provider。
3. 模型列表来自当前 Key 的实时目录。
4. Windows PowerShell 与 macOS / Linux 验收。
5. 最小请求、工具调用、重复运行和回滚验证。

在状态改为“已验证”前，不提供会修改客户配置的终端命令。
