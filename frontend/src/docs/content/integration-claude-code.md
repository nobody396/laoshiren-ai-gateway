# Claude Code

**协议：** Anthropic Messages

**配置状态：** 已提供一次性配置命令

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择要使用模型的可用方案并创建 Key。

## 2. 运行配置命令

在该 Key 右侧点击 **一键配置**。命令使用10分钟有效、仅可领取一次的配置凭证，不包含明文 API Key。

### Windows PowerShell

复制页面生成的 PowerShell 命令，在普通用户 PowerShell 中运行。

### macOS / Linux

复制页面生成的终端命令，在 Zsh 或 Bash 中运行。

脚本会保留现有安装；只有未检测到 Claude Code 时才安装。

## 3. 验证

完全退出旧进程，重新打开终端：

```text
claude --version
claude
/status
/model
```

`/status` 必须显示 `https://api.laoshirenai.com`；再完成一个“只回复 OK”的新任务。

## 4. 回滚

首次修改前的配置最多保留一个备份：

```text
~/.claude/settings.json.bak
```

新版回滚命令完成真实系统验收前，不提供未经验证的删除命令。

## 常见错误

- `401`：Key 无效、停用或复制不完整。
- `400 model not supported`：模型不属于当前 Key 的可用方案。
- 仍请求旧模型：完全退出 Claude Code 后新建会话。
- `500`–`504`：记录北京时间和完整错误后重试；不要重新安装客户端。
