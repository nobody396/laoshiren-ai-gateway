# Gemini CLI

Gemini CLI 使用 **Gemini GenerateContent** 协议。本文配置已在 Gemini CLI `0.57.0` 与 `gemini-3.1-pro` 上完成文件读取、Shell、文件修改和多轮任务测试。

## 客户端原生协议

- Gemini GenerateContent：支持；包含普通 `generateContent` 与流式 `streamGenerateContent`。
- OpenAI Responses / Chat Completions / Anthropic Messages：不支持自定义 Provider 配置。
- Google 登录或 Vertex AI 属于认证与 Google 后端选择，不代表客户端支持额外的 OpenAI 或 Anthropic 协议。

## 模型兼容范围

- 当前 Gemini Key 可选择：`gemini-3.1-pro`、`gemini-3.7-flash`。
- 已完成真实 Gemini CLI Agent 闭环：`gemini-3.1-pro`。
- Gemini CLI 使用原生 GenerateContent，不能直接填写 GPT、Claude、Grok 或 Kimi 模型。

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 Gemini 分组创建 Key。

## 2. 一键配置

在 Key 右侧点击 **一键配置**，选择 Gemini CLI：

```bash
curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=0.7.14' | \
  LAOSHIRENAI_SETUP_TOKEN='ONE_TIME_SETUP_TOKEN' \
  LAOSHIRENAI_TOOLS='gemini' bash
```

Windows 使用同版本 `install.ps1`，并把 `LAOSHIRENAI_TOOLS` 设为 `gemini`。

## 3. 手动配置

当前终端：

```bash
export GEMINI_API_KEY='YOUR_API_KEY'
export GOOGLE_GEMINI_BASE_URL='https://api.laoshirenai.com'
export GOOGLE_GENAI_USE_VERTEXAI='false'
export GEMINI_MODEL='YOUR_MODEL_ID'
```

编辑 `~/.gemini/settings.json`：

```json
{
  "security": {
    "auth": {"selectedType": "gemini-api-key"}
  },
  "model": {
    "name": "YOUR_MODEL_ID"
  }
}
```

## 4. 启动使用

```bash
npx --yes @google/gemini-cli@0.57.0 --version
npx --yes @google/gemini-cli@0.57.0 --model YOUR_MODEL_ID
```

Gemini CLI 会通过 `GOOGLE_GEMINI_BASE_URL` 请求 `/v1beta/models/...`。

## 常见错误

- `Invalid auth method selected`：`settings.json` 中没有选择 `gemini-api-key`。
- `401`：`GEMINI_API_KEY` 无效或没有加载。
- 模型不可用：使用当前 Key 返回的 Gemini 模型 ID。
