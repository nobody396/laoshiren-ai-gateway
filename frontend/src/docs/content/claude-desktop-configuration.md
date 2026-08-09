# Claude Desktop 第三方 Provider 配置指南

本教程将指导你如何在 Claude Desktop 客户端中配置第三方 API Provider，使其通过自定义网关连接到不同的 AI 服务（如 老实人 AI、Kimi 等）。

## 前置准备

在开始配置之前，请确保：

1. 已下载并安装 Claude Desktop 客户端
2. 拥有第三方服务的 API Key
3. 了解该服务的 Gateway 地址（Base URL）

### 下载 Claude Desktop

访问官方下载页面获取最新版本：

[Claude Desktop 下载](https://code.claude.com/docs/en/desktop-quickstart)


## macOS 配置流程

### 第一步：启用开发者模式

1. 打开 Claude Desktop 客户端


2. 点击顶部菜单栏的 **Help（帮助）**，依次选择 **Troubleshooting** → **Enable Developer Mode**


3. 在确认对话框中点击 **Enable**，应用将自动重启


### 第二步：配置第三方接口

1. 重启后进入欢迎界面，**先不要登录**。保持客户端处于选中状态，此时顶部菜单栏会出现 **Developer** 选项


2. 点击 **Developer** → **Configure Provider**，打开配置面板

3. 填写第三方接口信息。以 老实人 AI 为例：


   | 配置项 | 值 |
      |--------|-----|
   | **Gateway base URL** | `https://api.laoshirenai.com` |
   | **Gateway API key** | 你的 老实人 AI API Key |

4. 点击右下角的 **Apply locally**

5. 在弹出的确认框中选择 **Relaunch now** 重启应用


6. 重启后点击 **Continue with gateway**，即可进入主界面


### 界面说明

进入主界面后，可以使用以下快捷键切换视图：

| 快捷键 | 功能 |
|--------|------|
| `Command + 1` | Cowork 界面（默认） |
| `Command + 2` | 编码界面 |

### 功能验证

配置完成后，可以通过简单对话验证连接是否成功。例如询问当前使用的模型：


### 插件与技能说明

如果你已在 Claude Code CLI 中配置并启用了插件或技能，Desktop 端会自动同步这些配置。你也可以手动在 Desktop 端上传技能文件：


### 高级配置

#### 配置最高权限

默认情况下，Claude Desktop 执行敏感操作时会弹出确认框。如需关闭确认提示，可设置最高权限：

1. 点击左下角的 **Cowork 3P \| Gateway** 状态区域，或按 `Command + ,` 打开设置


2. 在设置面板中调整权限级别



#### 保持电脑活跃

执行长时间任务时，建议开启 **Keep computer awake** 功能，防止电脑进入休眠状态导致任务中断：


> **注意**：此功能可防止电脑进入待机状态，显示屏仍可自动关闭。但**合上笔记本电脑盖子仍会使设备进入睡眠状态**。

### 配置多个 Provider

Claude Desktop 支持配置多个第三方接口，方便在不同服务间切换：

1. 重复启用开发者模式的步骤
2. 进入 **Developer** → **Configure Provider**
3. 点击右上角的下拉菜单（显示当前配置名称，如 `default`）
4. 选择 **New configuration** 创建新配置


5. 填写新的 Gateway 地址和 API Key（如 Kimi 的配置信息）


6. 应用并重启后，可在已配置的 Provider 间自由切换


## Windows 配置流程

Windows 端的配置逻辑与 macOS 基本一致，仅开发者模式的入口方式不同：

### 启用开发者模式

1. 打开 Claude Desktop，进入欢迎界面的邮箱输入页面
2. 按下 `Tab` 键切换焦点，直到左上角的菜单图标被选中
3. 按 `Enter` 键打开菜单


4. 选择 **Developer** → **Configure third-party interface**


### 配置接口信息

填写 Gateway 地址和 API Key，点击 **Apply locally** 后重启生效：


## 常见问题

| 问题 | 解决方案 |
|------|---------|
| 配置后无法连接 | 检查 Gateway URL 是否以 `https://` 开头，API Key 是否有效 |
| 切换 Provider 后失效 | 确保点击了 **Apply locally** 并重启应用 |
| 权限不足无法执行命令 | 在设置中将权限级别调整为最高 |

## 相关资源

- [Claude Desktop 官方文档](https://code.claude.com/docs/en/desktop-quickstart)
- [Claude Code 国内使用指南](/docs/claude-code-china-guide)
