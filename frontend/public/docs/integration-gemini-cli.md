# Gemini CLI

**协议：** Gemini GenerateContent

**配置状态：** 已提供一次性配置命令

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 Gemini 可用方案并创建 Key。

## 2. 运行配置命令

在该 Key 右侧点击 **一键配置**。

### Windows PowerShell

复制页面生成的 PowerShell 命令，在普通用户 PowerShell 中运行。

### macOS / Linux

复制页面生成的终端命令，在 Zsh 或 Bash 中运行。

## 3. 验证

```text
gemini --version
gemini
```

新会话发送“只回复 OK”。请求必须出现在使用记录的 `/v1beta/models/...` 路径中。

## 4. 回滚

首次修改前最多保留以下两个备份：

```text
~/.gemini/.env.bak
~/.gemini/settings.json.bak
```

每个目标文件只保留一个原始备份。

## 常见错误

- `401`：Key 无效或 Gemini 凭据没有加载。
- `400`：模型 ID 或 GenerateContent 参数错误。
- 仍走官方接口：完全退出 Gemini CLI 后新建会话。
- 模型不可见：确认 Key 属于 Gemini 可用方案。
