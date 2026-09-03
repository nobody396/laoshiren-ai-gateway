# ZCode

ZCode 使用自定义模型供应商连接 **Anthropic Messages** 或 OpenAI 兼容接口。本文使用 Anthropic Messages、`claude-sonnet-5` 完成了真实 Agent 文件读取、`pwd`、文件修改和修改后复读测试。

当前验证版本：ZCode App `3.10.1`，App 内置 ZCode CLI `0.16.5`。

## 客户端原生协议

当前 ZCode App `3.10.1` 的自定义供应商 API Format 支持：

- OpenAI Responses；
- OpenAI Chat Completions；
- Anthropic Messages。

自定义供应商不支持 Gemini GenerateContent。一个 Provider 实例只能选择一种 API Format；同一模型需要测试两种协议时，应分别建立两个 Provider。

## 模型兼容范围

- 当前 Claude Key 返回的 Fable、Haiku、Sonnet 和 Opus 模型可以作为 Anthropic 自定义供应商模型添加；具体子集以当前 Key 为准。
- 已完成真实 ZCode App 与内置 CLI Agent 闭环：`claude-sonnet-5`。
- `glm-5.3` 已通过 ZCode 的 Chat Completions Agent 闭环；Responses 基础连接可用，但联网搜索不可用。
- Qwen `qwen3.6-flash`、`qwen3.6-plus`、`qwen3.7-flash`、`qwen3.7-max`、`qwen3.7-plus`、`qwen3.8-max` 均已完成 Responses 文件读取与工具续轮闭环。
- 跨模型真实 CLI Agent 闭环：`kimi-k3`、`grok-4.6`（Chat Completions），`qwen3.8-max`、`deepseek-v4-flash-0731`（Responses）。
- `gpt-5.6-sol` 的 Responses 请求会因 ZCode 当前整套工具声明被网关以 400 拒绝，因此不能只凭 Responses 协议把 GPT 标为 ZCode 可用。
- 模型能否使用由“当前 Key 返回模型 + 分组协议 + ZCode 客户端行为”共同决定，不能只看供应商名称。

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 Claude 分组创建 Key，再从该 Key 的模型列表复制模型 ID。

本文验证使用：

```text
Base URL: https://api.laoshirenai.com
协议: Anthropic Messages
模型: claude-sonnet-5
```

## 2. 在 ZCode App 中配置

1. 打开 ZCode，点击模型选择器底部的 **管理模型**。
2. 在供应商列表底部点击 **添加供应商**。
3. 名称填写“老实人AI Claude”，协议选择 **Anthropic**。
4. Base URL 填写 `https://api.laoshirenai.com`，不要附加 `/v1/messages`。
5. API Key 填写站内创建的 Key。
6. 点击 **添加模型**，填写该 Key 的模型列表返回的模型 ID。
7. 保存、打开启用开关，再在模型选择器中选中该模型。

### OpenAI 兼容模型

配置 `glm-5.3` 等 OpenAI 兼容模型时：

```text
Chat Completions Base URL: https://api.laoshirenai.com/v1
Responses Base URL: https://api.laoshirenai.com
模型: 当前 Key 的 /v1/models 返回值
```

Base URL 只填写基础地址。不要填写完整的 `/v1/chat/completions` 或 `/v1/responses`；ZCode 会按所选 API 格式自动追加端点，填写完整路径会导致重复拼接并返回 `404 page not found`。

`glm-5.3` 的模型配置应填写：

```text
上下文窗口: 1048576
最大输出 Token: 131072
输入类型: 仅文本
输出类型: 仅文本
```

不要勾选图片或视频。ZCode 即使先用 FFmpeg 从视频抽帧，后续把图片帧交给阿里云 GLM 5.3 时仍会被上游拒绝。

`glm-5.3` 使用 Chat Completions 已完成文件读取、Shell、修改、复读和最终回复。选择 Responses 可以完成基础连接，但阿里云百炼官方能力表明确标注 `ZHIPU/GLM-5.3` 不支持联网搜索，因此 Responses `web_search` 会失败。

ZCode App 的供应商配置保存在：

```text
macOS / Linux: ~/.zcode/v2/config.json
Windows: %USERPROFILE%\.zcode\v2\config.json
```

日常使用应通过 ZCode 的模型设置界面修改供应商；不要用整份文件覆盖已有配置。

## 3. ZCode CLI 手动配置

ZCode App 自带的命令行 Agent 使用独立配置文件：

```text
macOS / Linux: ~/.zcode/cli/config.json
Windows: %USERPROFILE%\.zcode\cli\config.json
```

如果已有该文件，只合并下面的 `provider.laoshirenai-claude` 和 `model.main`，不要覆盖其他 MCP、Skill、Hook 或权限设置：

```json
{
  "provider": {
    "laoshirenai-claude": {
      "kind": "anthropic",
      "options": {
        "apiKeyRequired": true,
        "baseURL": "https://api.laoshirenai.com"
      },
      "models": {
        "claude-sonnet-5": {
          "name": "claude-sonnet-5"
        }
      }
    }
  },
  "model": {
    "main": "laoshirenai-claude/claude-sonnet-5"
  }
}
```

`glm-5.3` 使用 Chat Completions 时，CLI Provider 改为：

```json
{
  "provider": {
    "laoshirenai-glm": {
      "kind": "openai-compatible",
      "options": {
        "apiKeyRequired": true,
        "baseURL": "https://api.laoshirenai.com/v1"
      },
      "models": {
        "glm-5.3": {
          "name": "glm-5.3"
        }
      }
    }
  },
  "model": {
    "main": "laoshirenai-glm/glm-5.3"
  }
}
```

当前终端提供 Key：

```bash
export ZCODE_API_KEY='YOUR_API_KEY'
```

Windows PowerShell：

```powershell
$env:ZCODE_API_KEY='YOUR_API_KEY'
```

`model.main` 必须使用 `供应商ID/模型ID`；模型 ID 必须来自当前 Key 的模型列表。

## 4. 启动使用

ZCode App：打开项目目录，选中刚才添加的模型，然后直接描述文件任务。

macOS 上也可以调用 App 内置 CLI：

```bash
node "/Applications/ZCode.app/Contents/Resources/glm/zcode.cjs" \
  --prompt "读取 README，运行 pwd，最后总结项目" \
  --cwd "$PWD" \
  --mode yolo \
  --json
```

Windows / Linux 的 App 安装位置可能不同；日常使用直接从 ZCode App 启动即可。

## 常见错误

- `Model config is missing`：缺少 `~/.zcode/cli/config.json` 中的 `provider` 或 `model.main`。
- `model not supported`：模型 ID 不在当前 Key 的模型列表中。
- `401`：Key 无效、停用或 `ZCODE_API_KEY` 没有传入当前进程。
- `404 page not found`：OpenAI 兼容 Base URL 填成了完整端点；改为 `https://api.laoshirenai.com/v1`。
- Responses 联网搜索失败：阿里云百炼的 `ZHIPU/GLM-5.3` 官方能力表标注“不支持联网搜索”；这不代表普通 Responses 请求失败。
- App 能用但 CLI 不能用：App 的 `v2/config.json` 与 CLI 的 `cli/config.json` 是两个配置入口，需要分别配置。
- 修改后仍使用旧模型：新建 ZCode 会话并重新选择模型。

ZCode 自定义供应商的界面步骤与配置位置可参阅 [ZCode 官方连接模型文档](https://zcode.z.ai/cn/docs/configuration)。
