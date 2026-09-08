本文说明如何在 **ZCode App** 中接入老实人AI的 Claude 模型。配置完成后，ZCode 会通过老实人AI的 Anthropic Messages 接口调用模型。

生产 Base URL：

```text
https://api.laoshirenai.com
```

> 本教程按 ZCode App 3.11.2 的官方模型配置界面编写，并在本机 ZCode 3.10.1 中验证过同一套 Anthropic 自定义供应商字段。

## 准备工作

开始前确认：

1. 已从[ZCode 官方下载页面](https://zcode.z.ai/cn/docs/install)安装并启动 ZCode；
2. 已在[老实人AI API 密钥页面](https://laoshirenai.com/keys)选择 Claude 分组并创建有效 Key；
3. 当前 Key 有可用额度。

## 第一步：查询可用模型

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

从返回结果中复制一个准确的 Claude 模型 ID，下面用 `YOUR_MODEL_ID` 表示。

## 第二步：添加自定义供应商

在 ZCode 中按下面的顺序操作：

1. 点击对话框中的模型名称，打开模型选择器；
2. 点击底部的 **管理模型**；
3. 在供应商列表底部点击 **添加供应商**；
4. 按下表填写；
5. 保存并打开供应商的启用开关。

| 配置项 | 填写内容 |
| --- | --- |
| 供应商名称 | `lsrai` |
| API Format / 协议 | `Anthropic` |
| Base URL | `https://api.laoshirenai.com` |
| API Key | 老实人AI API Key |

Base URL 只填写根地址，不要追加 `/v1/messages`。ZCode 会根据 Anthropic 协议自动拼接请求路径。

本教程不需要添加任何 WebSocket 配置，也不要在供应商配置文件中手工加入 `responses_websockets_v2`。

## 第三步：添加模型

1. 在刚创建的 `lsrai` 供应商中点击 **添加模型**；
2. 模型 ID 填写第一步查到的 `YOUR_MODEL_ID`；
3. 最大输出 Token 保持空白；
4. 上下文窗口先保持默认值；
5. 保存模型。

不要凭显示名称猜模型 ID，必须复制当前 Key 的 `/v1/models` 返回值。

## 第四步：选择并启动

回到对话框的模型选择器，选择刚添加到 `lsrai` 供应商下的模型，然后新建一个会话。

输入：

```text
请只回复：lsrai ZCode 连接成功
```

同时满足以下两项才算配置成功：

1. ZCode 正常返回结果；
2. 老实人AI的使用记录中出现这次请求。

## 配置保存位置

ZCode 会把模型供应商配置保存到：

| 系统 | 文件位置 |
| --- | --- |
| macOS / Linux | `~/.zcode/v2/config.json` |
| Windows | `%USERPROFILE%\.zcode\v2\config.json` |

日常修改请使用 ZCode 的 **管理模型** 界面。不要用整份文件覆盖现有配置，否则可能同时删除其他供应商和设置。

## 切换模型

1. 重新查询当前 Key 的 `/v1/models`；
2. 在 `lsrai` 供应商中添加或修改模型 ID；
3. 保存并重新打开模型选择器；
4. 选择新模型并新建会话。

## 常见问题

| 现象 | 可能原因 | 处理方法 |
| --- | --- | --- |
| 返回 `401` | API Key 错误、停用或复制不完整 | 重新复制当前有效 Key |
| 返回 `404 page not found` | Base URL 填成了完整接口路径 | 改为 `https://api.laoshirenai.com` |
| 提示模型不存在 | 模型 ID 不属于当前 Key | 重新查询 `/v1/models` |
| 保存后看不到模型 | 供应商没有启用或模型没有保存 | 打开启用开关并重新打开模型选择器 |
| 修改后仍使用旧模型 | 旧会话保留了之前的模型 | 新建会话并重新选择模型 |
| Claude 模型请求格式错误 | API Format 选择错误 | Claude 分组必须选择 `Anthropic` |

## 安全提示

- API Key 会保存在本机 ZCode 配置中，不要分享配置文件；
- 不要把 `~/.zcode/v2/config.json` 提交到 Git；
- Key 泄露后立即在老实人AI停用并重新创建。

配置步骤依据：[ZCode 官方安装文档](https://zcode.z.ai/cn/docs/install)、[ZCode 官方连接模型文档](https://zcode.z.ai/cn/docs/configuration)。
