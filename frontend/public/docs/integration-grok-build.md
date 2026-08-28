# Grok Build

**协议：** Grok / xAI 兼容 Responses

**配置状态：** 已提供一次性配置命令

## 1. 创建 Key

打开 [API 密钥](https://laoshirenai.com/keys)，选择 Grok 可用方案并创建 Key。

## 2. 运行配置命令

在该 Key 右侧点击 **一键配置**。

### Windows PowerShell

复制页面生成的 PowerShell 命令，在普通用户 PowerShell 中运行。

### macOS / Linux

复制页面生成的终端命令，在 Zsh 或 Bash 中运行。

## 3. 验证

```bash
grok --version
grok -m grok-4.6 -p "只回复 OK"
```

如果模型目录不再包含 `grok-4.6`，改用当前 Key 的模型列表返回值。

## 4. 回滚

首次修改前的配置最多保留一个：

```text
~/.grok/config.toml.bak
```

新版回滚命令完成真实系统验收前，不提供未经验证的删除命令。

## 常见错误

- `401`：Key 无效或未绑定 Grok 可用方案。
- 模型不存在：从 `/v1/models` 复制当前模型 ID。
- 找不到 `grok`：重新打开终端后检查版本。
- 请求超时：记录北京时间和完整错误，不要反复安装。
