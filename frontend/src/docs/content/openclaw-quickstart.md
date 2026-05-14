# Dragon Code × OpenClaw 快速开始指南

## 适用场景

这篇文档只解决一件事：把 OpenClaw 的模型服务接到 Dragon Code，并完成一次可用性验证。

如果你还没安装 Node.js，请先参考 [Node.js 环境安装指南](nodejs-setup)。

---

## 1. 准备事项

开始前请先确认以下几点：

- 你已经在 [Dragon Code 控制台](https://your-domain.example/keys) 创建好一枚 API Key
- 你知道要调用的实际模型名，例如 `gpt-5.4`（完整支持列表见下方[支持的模型](#支持的模型)）
- 你的运行环境满足 OpenClaw 官方要求：推荐 `Node 24`，兼容 `Node 22.16+`
- Windows 用户建议优先使用 `WSL2 + Ubuntu` 安装和运行 OpenClaw

---

## 支持的模型

### OpenAI 兼容模型

| 模型名 |
|---|
| `gpt-5.4` |
| `gpt-5.2` |
| `gpt-5.3-codex` |
| `gpt-5.2-codex` |
| `gpt-5.1-codex-max` |
| `gpt-5.1-codex-mini` |

### Anthropic 兼容模型

| 模型名 |
|---|
| `claude-opus-4-6` |
| `claude-sonnet-4-6` |
| `claude-haiku-4-5-20251001` |

---

## 2. 推荐路径：使用 OpenClaw 向导接入

如果你只是想先打通 OpenClaw 和 Dragon Code，最省事的方式是先跑 OpenClaw 自带向导，等浏览器控制台能正常回复后，再继续配置频道或后台常驻。

### 第一步：安装 OpenClaw

macOS / Linux / WSL2 可以直接执行官方安装脚本：

```bash
# 安装 openclaw 命令行工具，--no-onboard 表示安装后先不自动进入向导
curl -fsSL https://openclaw.ai/install.sh | bash -s -- --no-onboard
```

安装完成后，可先验证命令是否可用：

```bash
openclaw --help
```

### 第二步：启动接入向导

```bash
openclaw onboard
```

### 第三步：在向导中选择自定义服务商

当向导进入模型和鉴权步骤时，选择 `Custom provider`，然后按下面的值填写：

| 字段 | 填写值 |
|---|---|
| 兼容类型 | `Anthropic-compatible`或者 `OpenAI-compatible` |
| 基础地址 | 见下方说明 |
| 模型名 | `claude-sonnet-4-6` 或者 `gpt-5.4`（完整列表见[支持的模型](#支持的模型)）|
| Provider ID | `dragoncode` |
| API Key | 你创建的 Dragon Code API Key |

> **基础地址填写说明：**
> - 选择 `Anthropic-compatible` 时，填写：`https://your-domain.example`
> - 选择 `OpenAI-compatible` 时，需要在末尾加上 `/v1`，填写：`https://your-domain.example/v1`

### 第四步：完成向导并验证

首次接入时，网关端口、绑定方式和后台常驻服务都可以先保留默认值。完成向导后，依次运行：

```bash
# 检查配置格式是否正确
openclaw doctor

# 查看当前状态和加载到的模型
openclaw status

# 打开本地控制台，发一条消息验证是否能正常回复
openclaw dashboard
```

如果控制台里发消息能收到模型回复，说明接入已经成功。

---

## 3. 脚本化接入

如果你要把 OpenClaw 初始化写进脚本、服务器部署流程或企业镜像，推荐使用非交互模式。

### 第一步：导出 API Key

```bash
# 先把 Dragon Code API Key 写入环境变量
export CUSTOM_API_KEY="YOUR_DRAGONCODE_API_KEY"
```

### 第二步：执行非交互接入命令

```bash
# 非交互式接入：将 Dragon Code 配置为自定义服务商
openclaw onboard --non-interactive \
  --mode local \
  --auth-choice custom-api-key \
  --custom-base-url "https://your-domain.example" \
  --custom-model-id "gpt-5.4" \
  --custom-provider-id "dragoncode" \
  --custom-compatibility openai \
  --secret-input-mode ref \
  --gateway-port 18789 \
  --gateway-bind loopback
```

### 第三步：确认环境变量名称

如果你使用了 `--secret-input-mode ref`，OpenClaw 当前版本通常会把 API Key 以环境变量引用方式保存。推荐直接使用上面的 `CUSTOM_API_KEY`，这样和官方自动化文档保持一致。

如果你已经生成了配置，也可以打开 `~/.openclaw/openclaw.json` 检查实际引用的变量名；如果你的版本写入的是别的变量名，再按实际配置补环境变量即可，例如：

```bash
export OPENAI_API_KEY="YOUR_DRAGONCODE_API_KEY"
```

---

## 4. 手动检查或手动写入配置

如果你不想跑向导，也可以直接检查或编辑 `~/.openclaw/openclaw.json`。

下面是一份可直接参考的配置示例，请把模型名和 API Key 变量替换成你自己的实际值：

```json5
{
  agents: {
    defaults: {
      // 默认模型必须写成“服务商 ID/模型名”格式，不能只写模型名
      model: { primary: "dragoncode/gpt-5.4" },
    },
  },
  models: {
    providers: {
      dragoncode: {
        baseUrl: "https://your-domain.example",
        // 推荐引用环境变量，避免把密钥明文写进配置文件
        apiKey: "${CUSTOM_API_KEY}",
        api: "openai-completions",
        models: [
          {
            // 这里替换成 Dragon Code 控制台里实际可用的模型名
            id: "gpt-5.4",
            name: "gpt-5.4",
          },
        ],
      },
    },
  },
}
```

> **最容易写错的地方**：默认模型必须写成 `dragoncode/模型名`，例如 `dragoncode/gpt-5.4`，不能只写 `gpt-5.4`。

---

## 5. 成功标准

满足下面几项，基本就说明接入已经打通：

- 基础地址填写的是 `https://your-domain.example`
- API Key 仍然有效，未过期、未停用、未耗尽额度
- 默认模型写成了 `dragoncode/你的模型名`
- `openclaw doctor` 和 `openclaw status` 没有报配置错误
- 浏览器控制台能正常发消息并收到回复

---

## 6. 常见问题

### 明明填了地址，还是连不上

优先检查基础地址是不是 `https://your-domain.example`。不要带 /

### OpenClaw 能启动，但发消息时报模型不存在

通常先查这两项：

- 你填写的模型名是否就是 Dragon Code 当前实际开放的调用名
- 默认模型是否写成了 `dragoncode/模型名`，而不是只写模型名

### 使用脚本化命令时提示 API Key 缺失

这通常是环境变量没有真正导出成功，或者变量名和 OpenClaw 配置里引用的不一致。

可以直接打开 `~/.openclaw/openclaw.json`，确认 `apiKey` 字段引用的是哪个环境变量，然后在当前 shell 中补上对应变量。按照本文推荐路径，优先检查 `CUSTOM_API_KEY`。

### 我已经配好 OpenClaw，为什么还不能在 WhatsApp 或 Telegram 里用

这通常不是模型接入问题，而是频道还没有配置完成。本页只负责把 OpenClaw 和 Dragon Code 之间的模型调用打通；频道配置请继续参考 [OpenClaw 官方 Channels 文档](https://docs.openclaw.ai/channels)。

---

## 7. 下一步

- 还没创建 Key：回到 [Dragon Code 控制台](https://your-domain.example/keys) 先创建 API Key
- 想先配置其他开发工具：继续查看 [Claude Code快速开始指南](claude-code-quickstart) 和 [Codex快速开始指南](codex-quickstart)
- 需要安装运行环境：查看 [Node.js 环境安装指南](nodejs-setup)
- 遇到常见接入问题：查看 [常见问题](faq)
