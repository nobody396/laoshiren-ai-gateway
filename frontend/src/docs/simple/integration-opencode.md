本文说明如何把老实人AI API Key 配置到 **OpenCode 1.x**。配置完成后，OpenCode 会通过老实人AI的 Responses API 调用模型。

生产 Base URL：

```text
https://api.laoshirenai.com/v1
```

> 本教程已按 `opencode` 1.18.29 核对，不适用于仍在测试中的 `opencode2`。

## 准备工作

开始前确认：

1. 已在[老实人AI API 密钥页面](https://laoshirenai.com/keys)创建有效 Key；
2. 当前 Key 有可用额度，并且已经授权目标模型；
3. `opencode --version` 可以正常输出版本。

## 安装 OpenCode

macOS / Linux / WSL：

```bash
curl -fsSL https://opencode.ai/install | bash
```

也可以通过 npm 安装：

```bash
npm install -g opencode-ai
```

国内 npm 镜像：

```bash
npm install -g opencode-ai --registry=https://registry.npmmirror.com
```

Windows PowerShell：

```powershell
npm.cmd install -g opencode-ai --registry=https://registry.npmmirror.com
```

Windows 推荐优先在 WSL 中使用 OpenCode。

## 配置 OpenCode

### 第一步：设置 API Key

macOS / Linux / WSL：

```bash
export LSRAI_API_KEY="YOUR_API_KEY"
```

Windows PowerShell：

```powershell
$env:LSRAI_API_KEY = "YOUR_API_KEY"
```

### 第二步：修改 opencode.json

用户级配置文件位置：

| 系统 | 文件位置 |
| --- | --- |
| macOS / Linux / WSL | `~/.config/opencode/opencode.json` |
| Windows | `%USERPROFILE%\.config\opencode\opencode.json` |

文件不存在时创建它；已经存在时，把 `lsrai` 合并到现有 `provider` 中，不要覆盖插件、权限或其他 Provider。

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "lsrai/YOUR_MODEL_ID",
  "provider": {
    "lsrai": {
      "npm": "@ai-sdk/openai",
      "name": "lsrai",
      "options": {
        "baseURL": "https://api.laoshirenai.com/v1",
        "apiKey": "{env:LSRAI_API_KEY}"
      },
      "models": {
        "YOUR_MODEL_ID": {
          "name": "YOUR_MODEL_ID"
        }
      }
    }
  }
}
```

所有 Provider ID 和显示名称统一使用纯英文小写 `lsrai`。把三处 `YOUR_MODEL_ID` 替换成同一个准确模型 ID。

这里使用 `@ai-sdk/openai`，因为本教程走 Responses API；不要替换成只用于 Chat Completions 的 `@ai-sdk/openai-compatible`。

### 第三步：启动 OpenCode

从设置了 `LSRAI_API_KEY` 的同一个终端运行：

```bash
opencode
```

进入 OpenCode 后运行 `/models`，选择 `lsrai/YOUR_MODEL_ID`。

## 查询并切换模型

macOS / Linux / WSL：

```bash
curl -sS 'https://api.laoshirenai.com/v1/models' -H "Authorization: Bearer $LSRAI_API_KEY"
```

Windows PowerShell：

```powershell
(Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{ Authorization = "Bearer $env:LSRAI_API_KEY" }).data.id
```

切换模型时，需要同时修改：

1. 顶层 `model`；
2. `provider.lsrai.models` 中的模型键；
3. 模型条目里的 `name`。

修改后重新启动 OpenCode，再通过 `/models` 选择新模型。

## 验证连接

启动 OpenCode 后输入：

```text
请只回复：lsrai OpenCode 连接成功
```

同时满足以下两项才算配置成功：

1. OpenCode 正常返回结果；
2. 老实人AI的使用记录中出现这次请求。

## 常见问题

| 现象 | 可能原因 | 处理方法 |
| --- | --- | --- |
| 返回 `401` | Key 没有传入 OpenCode | 从设置 `LSRAI_API_KEY` 的同一终端启动 |
| `/models` 没有 `lsrai` | Provider ID、JSON 结构或路径错误 | 检查用户级 `opencode.json` |
| 提示模型不存在 | 三处模型 ID 不一致或无权限 | 查询 `/v1/models` 并统一替换 |
| 工具调用失败 | 使用了错误的 Provider 包 | Responses 使用 `@ai-sdk/openai` |
| 配置解析失败 | JSON 逗号、引号或括号错误 | 修复 JSON，不要重复添加第二个顶层 `provider` |

## 安全提示

- 使用 `{env:LSRAI_API_KEY}` 引用环境变量，不把真实 Key 写进 JSON；
- 不要提交包含 Key 的 `.env` 或配置文件；
- Key 泄露后立即在老实人AI停用并重新创建。

配置字段依据：[OpenCode 自定义 Provider](https://opencode.ai/docs/providers#custom-provider)、[OpenCode 模型配置](https://opencode.ai/docs/models)。
