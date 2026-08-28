# Codex

**协议：** OpenAI Responses

**配置状态：** 已提供一次性配置命令

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 GPT / Codex 可用方案并创建 Key。

## 2. 运行配置命令

在该 Key 右侧点击 **一键配置**。

### Windows PowerShell

复制页面生成的 PowerShell 命令，在普通用户 PowerShell 中运行。

### macOS / Linux

复制页面生成的终端命令，在 Zsh 或 Bash 中运行。

脚本会合并 `~/.codex/config.toml`，写入 Responses Provider，并保留其他配置。

## 3. 验证

完全退出旧进程，重新打开终端：

```bash
codex --version
codex exec --sandbox read-only "只回复 OK，不要修改文件"
```

任务必须完整结束，并在使用记录中出现 `/v1/responses`。

## 4. 回滚

首次修改前的配置最多保留一个 `.bak`。新版回滚命令完成真实系统验收前，不提供未经验证的删除命令。

## 常见错误

- Codex 登录页要求官方账号：检查 `model_provider` 是否已切换。
- `401`：Key 无效或凭据文件未写入。
- `400 model not supported`：改用当前 Key 的 `/v1/models` 返回模型。
- 修改配置后仍走旧路由：完全退出 Codex，再新建任务。
