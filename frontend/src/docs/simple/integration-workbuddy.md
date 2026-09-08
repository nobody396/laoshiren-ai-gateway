本文说明如何在 **WorkBuddy 桌面版** 中接入老实人AI模型。配置完成后，WorkBuddy 会通过老实人AI的 OpenAI Chat Completions 接口调用模型。

完整接口地址：

```text
https://api.laoshirenai.com/v1/chat/completions
```

> WorkBuddy 官方当前版本为 5.5.3。本机 WorkBuddy 5.5.1 已验证能够从 `~/.workbuddy/models.json` 读取老实人AI自定义模型并完成应用内回复；Windows 请优先使用本文的图形界面配置，不要使用旧版一键导入命令。

## 准备工作

开始前确认：

1. 已从[WorkBuddy 官方页面](https://copilot.tencent.com/work/)下载并安装 WorkBuddy；
2. 已在[老实人AI API 密钥页面](https://laoshirenai.com/keys)创建支持 Chat Completions 的有效 Key；
3. 当前 Key 有可用额度。

WorkBuddy 桌面版支持 macOS 和 Windows。Linux 目前仅提供部分国产系统应用商店版本，本教程不对其配置路径作保证。

## 第一步：查询可用模型

macOS：

```bash
export LSRAI_API_KEY="YOUR_API_KEY"
curl -sS 'https://api.laoshirenai.com/v1/models' -H "Authorization: Bearer $LSRAI_API_KEY"
```

Windows PowerShell：

```powershell
$env:LSRAI_API_KEY = "YOUR_API_KEY"
(Invoke-RestMethod -Uri 'https://api.laoshirenai.com/v1/models' -Headers @{ Authorization = "Bearer $env:LSRAI_API_KEY" }).data.id
```

从返回结果中复制一个准确的模型 ID，下面用 `YOUR_MODEL_ID` 表示。

## 第二步：添加自定义模型

1. 打开 WorkBuddy 的 **设置**；
2. 进入 **模型**；
3. 点击 **添加模型**；
4. 接入方式选择 **自定义 / Custom**；
5. 按下表填写。

| 配置项 | 填写内容 |
| --- | --- |
| 模型名称 | `lsrai/YOUR_MODEL_ID` |
| 模型 ID | `YOUR_MODEL_ID` |
| URL | `https://api.laoshirenai.com/v1/chat/completions` |
| API Key | 老实人AI API Key |

## 第三步：设置高级选项

打开高级配置，并按下面设置：

| 配置项 | 建议值 |
| --- | --- |
| 自定义协议 | 开启 |
| 工具调用 | 开启 |
| 图片输入 | 首次配置先关闭 |
| 推理模式 | 首次配置先关闭 |
| 最大输入 Token | `128000` |
| 最大输出 Token | `8192` |

开启 **自定义协议** 后，WorkBuddy 会直接请求上面填写的完整 URL，不再自动追加路径。不要把 URL 改成只有 `/v1` 的 Base URL。

本教程只使用 Chat Completions，不支持 Responses、Anthropic Messages 或 Gemini GenerateContent，也不需要添加任何 WebSocket 配置。

先用最保守的图片和推理设置确认基础连接。只有模型目录明确说明支持对应能力时，再回来开启图片输入或推理模式。

## 第四步：保存并选择模型

1. 保存自定义模型；
2. 回到对话页面的模型选择器；
3. 在自定义模型分组中选择 `lsrai/YOUR_MODEL_ID`；
4. 新建一个会话。

输入：

```text
请只回复：lsrai WorkBuddy 连接成功
```

同时满足以下两项才算配置成功：

1. WorkBuddy 正常返回结果；
2. 老实人AI的使用记录中出现这次请求。

## 配置保存位置

WorkBuddy 桌面版会把自定义模型保存到：

| 系统 | 文件位置 |
| --- | --- |
| macOS | `~/.workbuddy/models.json` |
| Windows | `%USERPROFILE%\.workbuddy\models.json` |

请通过 WorkBuddy 的模型设置界面修改它。不要把桌面版文件误写成 `~/.codebuddy/models.json`；后者属于内置 CodeBuddy Code CLI，不是 WorkBuddy 桌面版的主要配置文件。

## 切换模型

1. 重新查询当前 Key 的 `/v1/models`；
2. 在 **设置 → 模型** 中添加或编辑模型；
3. 同时更新模型名称和模型 ID；
4. 保存后重新选择模型并新建会话。

## 常见问题

| 现象 | 可能原因 | 处理方法 |
| --- | --- | --- |
| 返回 `401` | API Key 错误、停用或没有保存 | 重新复制当前有效 Key 并保存 |
| 返回 `404` | URL 不完整或自定义协议状态不匹配 | 开启自定义协议并使用完整 `/v1/chat/completions` 地址 |
| 模型不出现在选择器 | 配置写进了 `.codebuddy` 或没有保存 | 通过 WorkBuddy 设置界面重新添加 |
| 提示模型不存在 | 模型 ID 不属于当前 Key | 重新查询 `/v1/models` |
| 普通问答成功但工具失败 | 工具调用能力没有开启 | 在高级配置中开启工具调用 |
| 图片请求失败 | 当前模型或分组不支持图片 | 关闭图片输入并新建会话 |

## 安全提示

- WorkBuddy 会把 API Key 保存在本机模型配置中，不要分享该文件；
- 不要把 `.workbuddy/models.json` 提交到 Git；
- Key 泄露后立即在老实人AI停用并重新创建。

配置步骤依据：[WorkBuddy 官方下载页面](https://copilot.tencent.com/work/)、[WorkBuddy 官方模型配置文档](https://www.codebuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Model)。
