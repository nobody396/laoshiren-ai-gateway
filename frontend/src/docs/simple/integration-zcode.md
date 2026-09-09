本文说明如何在 **ZCode App** 中接入老实人AI的 Qwen 模型。配置完成后，ZCode 会通过老实人AI的 OpenAI Responses 接口调用模型。

生产 Base URL：

```text
https://api.laoshirenai.com/v1
```

> 已按 ZCode App `3.11.2`、内置 ZCode CLI `0.16.5` 验证。Qwen 企业高速线路当前公开的六个模型都完成了真实文件读取、Shell、写入和复读闭环。

## 准备工作

开始前确认：

1. 已从[ZCode 官方下载页面](https://zcode.z.ai/cn/docs/install)安装并启动 ZCode；
2. 已在[老实人AI API 密钥页面](https://laoshirenai.com/keys)选择 **Qwen 企业高速线路**并创建有效 Key；
3. 当前 Key 有可用额度。

## 推荐：密钥页面一键配置

在密钥页面点击 **一键配置 → ZCode**，选择 macOS、Linux 或 Windows 后执行复制的命令。

命令会：

1. 查询当前 Key 的 `/v1/models`；
2. 把全部可见模型合并到 ZCode App 和 CLI 配置；
3. 保留其他供应商、MCP、Skill、Hook 和权限设置；
4. 修改前创建 `.bak` 备份并使用原子写入；
5. 对同一命令重复执行保持幂等。

命令只配置已经安装的 ZCode，不会静默下载桌面应用。执行完成后必须完全退出并重新打开 ZCode。

## 手动配置

### 第一步：查询模型

macOS / Linux：

```bash
export LSRAI_API_KEY="YOUR_API_KEY"
curl -sS 'https://api.laoshirenai.com/v1/models' -H "Authorization: Bearer $LSRAI_API_KEY"
```

Windows PowerShell：

```powershell
$env:LSRAI_API_KEY = "YOUR_API_KEY"
(Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{ Authorization = "Bearer $env:LSRAI_API_KEY" }).data.id
```

从返回结果中复制准确的模型 ID，不要根据显示名称猜测。

### 第二步：添加供应商

1. 点击对话框中的模型名称；
2. 点击 **管理模型**；
3. 点击 **添加供应商**；
4. 按下表填写；
5. 保存并启用供应商。

| 配置项 | 填写内容 |
| --- | --- |
| 供应商名称 | `lsrai` |
| API Format / 协议 | `OpenAI Responses` |
| Base URL | `https://api.laoshirenai.com/v1` |
| API Key | 老实人AI API Key |

Base URL 只填写到 `/v1`。不要填写完整的 `/v1/responses`，ZCode 会自动追加端点。本教程不需要任何 WebSocket 配置，也不要添加 `responses_websockets_v2`。

### 第三步：添加模型

在 `lsrai` 供应商中逐个添加第一步返回的模型 ID。最大输出 Token 和上下文窗口没有明确需要时保持默认，保存后重新打开模型选择器。

### 第四步：验证

选择刚添加的模型并新建会话，输入：

```text
请只回复：lsrai ZCode 连接成功
```

同时满足以下两项才算配置成功：

1. ZCode 正常返回结果；
2. 老实人AI的使用记录中出现这次请求。

## 配置保存位置

| 用途 | macOS / Linux | Windows |
| --- | --- | --- |
| ZCode App 供应商 | `~/.zcode/v2/config.json` | `%USERPROFILE%\.zcode\v2\config.json` |
| ZCode CLI | `~/.zcode/cli/config.json` | `%USERPROFILE%\.zcode\cli\config.json` |

一键配置只更新 `lsrai` 供应商和 CLI 默认模型，其他字段保持不变。不要用整份示例文件覆盖已有配置。

## 常见问题

| 现象 | 可能原因 | 处理方法 |
| --- | --- | --- |
| 返回 `401` | API Key 错误、停用或复制不完整 | 重新复制当前有效 Key |
| 返回 `404 page not found` | Base URL 填成了完整接口路径 | 改为 `https://api.laoshirenai.com/v1` |
| 提示模型不存在 | 模型 ID 不属于当前 Key | 重新查询 `/v1/models` |
| 保存后看不到模型 | 供应商没有启用或模型没有保存 | 打开启用开关并重新打开模型选择器 |
| 修改后仍使用旧模型 | 旧会话保留了之前的模型 | 新建会话并重新选择模型 |
| 请求格式错误 | API Format 选择错误 | Qwen 一键配置使用 `OpenAI Responses` |

## 安全提示

- API Key 会保存在本机 ZCode 配置中，不要分享配置文件；
- 不要把 `.zcode` 配置提交到 Git；
- Key 泄露后立即在老实人AI停用并重新创建。

配置步骤依据：[ZCode 官方安装文档](https://zcode.z.ai/cn/docs/install)、[ZCode 官方连接模型文档](https://zcode.z.ai/cn/docs/configuration)。
