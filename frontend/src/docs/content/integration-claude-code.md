# Claude Code

把当前 API Key 可以调用的模型导入 Claude Code。配置基线版本：Claude Code `2.1.251`；连接协议为 **Anthropic Messages**。

## 客户端原生协议

Claude Code 使用 Anthropic Messages。Base URL 填写网关根地址，客户端会自动请求 `/v1/messages`；不使用 Responses、Chat Completions 或 GenerateContent 配置。

## 模型兼容范围

分组决定 Key 能调用的模型；`/v1/models` 返回精确模型 ID。只有通过 Anthropic Messages 与 Claude Code 真实 Agent 验证的模型才提供一键导入。

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，创建 Key，并选择准备使用的分组。

分组决定这把 Key 可以调用哪些模型。创建完成后，系统会使用这把 Key 查询 `/v1/models`，只读取该分组真实开放的模型 ID。

## 2. 选择模型和 Claude Code

从这把 Key 返回的模型中选择一个主模型，再选择 **Claude Code**。

只有同时满足以下条件时，Claude Code 才会出现在可导入客户端中：

- 分组允许 Anthropic Messages；
- 模型推荐或支持 Anthropic Messages；
- 该模型已经完成 Claude Code 真实 Agent 验证。

协议兼容只代表“可能可以配置”，真实验证通过后才会提供一键导入。

## 3. 复制一次性命令

只有 API 密钥页面为当前 Key 实际生成命令时才执行；不要复制文档中的示例或自行拼接安装地址。正式命令使用 10 分钟有效、只能使用一次的凭证，不包含明文 API Key。

正式命令执行后会自动：

1. 读取当前 Key 的模型列表；
2. 写入 Base URL 和 Key；
3. 设置主模型；
4. 设置 Opus、Sonnet、Haiku、Fable 模型槽位；
5. 读取并验证最终配置；
6. 运行一个最小 Claude Code 任务。

## 4. 执行并验证

命令完成不等于配置成功。最终必须同时确认：

- Claude Code 可以启动；
- 当前模型来自这把 Key 的 `/v1/models`；
- 可以读取测试文件；
- 可以调用一次本地工具；
- 可以接收工具结果并返回最终答案。

## 命令会修改什么

Claude Code 用户配置文件：

```text
macOS / Linux: ~/.claude/settings.json
Windows: %USERPROFILE%\.claude\settings.json
```

只更新以下必要配置：

| 配置 | 用途 |
| --- | --- |
| `model` | Claude Code 默认使用的主模型 |
| `effortLevel` | 当前模型已经验证的默认推理强度 |
| `ANTHROPIC_BASE_URL` | 老实人AI API 地址 |
| `ANTHROPIC_AUTH_TOKEN` | 当前 API Key |
| `ANTHROPIC_MODEL` | 当前主模型 ID |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | Opus 槽位模型 |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | Sonnet 槽位模型 |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | Haiku 槽位模型 |
| `ANTHROPIC_DEFAULT_FABLE_MODEL` | Fable 槽位模型（客户端支持时） |
| `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY` | 允许读取网关模型 |

命令会先备份原文件，然后只更新上面的字段。已有的 MCP、Hook、权限、其他环境变量和其他设置都会保留，不会整份替换 `settings.json`。

如果现有 JSON 无法解析，命令会停止并提示错误，不会把配置清空后重写。重复执行同一份配置不会产生重复内容。

## 模型槽位怎么填写

所有写入的模型 ID 都必须来自当前 Key 的 `/v1/models`。

### Key 返回多个 Claude 模型

系统按照维护好的模型优先级，把可用模型分别填入对应槽位：

```text
主模型    → 用户选择的模型
Opus      → 当前 Key 可用的 Opus 模型
Sonnet    → 当前 Key 可用的 Sonnet 模型
Haiku     → 当前 Key 可用的 Haiku 模型
Fable     → 当前 Key 可用的 Fable 模型
```

某个系列不存在时，该槽位回退到用户选择的主模型，避免 Claude Code 自动使用这把 Key 无法调用的官方模型 ID。

### Key 只返回一个模型

主模型和所有受支持槽位都填写这个模型。

### 导入非 Claude 模型

只有已经通过 Anthropic Messages 与 Claude Code 真实验证的模型才允许导入。导入时，主模型和所有受支持槽位都映射到用户选择的模型。

## 手动配置

不使用一键命令时，可以手动编辑 `settings.json`。下面只展示需要合并的字段；不要覆盖文件中的其他内容。

```json
{
  "model": "YOUR_MAIN_MODEL",
  "effortLevel": "YOUR_VERIFIED_EFFORT",
  "env": {
    "ANTHROPIC_BASE_URL": "https://api.laoshirenai.com",
    "ANTHROPIC_AUTH_TOKEN": "YOUR_API_KEY",
    "ANTHROPIC_MODEL": "YOUR_MAIN_MODEL",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "YOUR_OPUS_OR_MAIN_MODEL",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "YOUR_SONNET_OR_MAIN_MODEL",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "YOUR_HAIKU_OR_MAIN_MODEL",
    "ANTHROPIC_DEFAULT_FABLE_MODEL": "YOUR_FABLE_OR_MAIN_MODEL",
    "CLAUDE_CODE_ATTRIBUTION_HEADER": "0",
    "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": "1"
  }
}
```

Base URL 使用根地址，不要添加 `/v1` 或 `/v1/messages`；Claude Code 会自动请求 `/v1/messages`。

## 更换模型

回到 API 密钥页面，重新选择：

1. Key；
2. 该 Key 开放的模型；
3. Claude Code。

再生成一次命令即可。新命令只更新主模型、对应槽位和必要连接字段，不覆盖其他 Claude Code 设置。

## 常见错误

- **模型没有出现**：当前 Key 的分组没有开放该模型。
- **Claude Code 没有出现**：该模型尚未完成 Claude Code 真实验证，或者协议不匹配。
- **`401`**：Key 无效、停用或没有正确写入。
- **`400 model not supported`**：配置中的模型 ID 不在当前 Key 的 `/v1/models` 中。
- **仍使用旧模型**：完全退出 Claude Code，重新打开终端并创建新会话。
- **配置文件无法解析**：先修复现有 `settings.json`，再重新执行命令。

[查看模型支持的协议和客户端 →](models)
