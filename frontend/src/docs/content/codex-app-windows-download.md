# Codex App Windows 下载

这页提供 **Codex App for Windows** 的下载和安装说明。这里说的是桌面版 **Codex App**，不是 `npm install -g @openai/codex` 安装的 **Codex CLI**。

## 首选：从老实人 AI 本站安装并自动更新

国内网络访问 Microsoft Store 或 GitHub 不稳定时，直接使用老实人 AI 缓存的最新版 MSIX。首次安装推荐复制下面一行到 PowerShell：

```powershell
Start-Process "ms-appinstaller:?source=https://laoshirenai.com/api/v1/public-downloads/codex/windows-x64/latest.appinstaller"
```

确认安装后，Windows 会登记本站更新地址，之后每天检查新版本，并在 Codex 未运行时安全更新。

如果电脑不能打开 App Installer，可改用直接下载：

[从老实人 AI 本站下载最新版 Codex App Windows x64 MSIX](/api/v1/public-downloads/codex/windows-x64/latest.msix)

下载完成后，在文件所在目录打开 PowerShell：

```powershell
Add-AppxPackage -Path .\OpenAI.Codex_*.Msix
```

本站的下载缓存会在服务启动时同步，并每 24 小时检查一次新版本。机器可读版本、文件名、大小和 SHA256 始终从动态接口读取，不再在文档里写死：

[Codex App Windows 动态 latest.json](/api/v1/public-downloads/codex/windows-x64/latest.json)

缓存包来自 Codex App 的 Microsoft Store 包元数据镜像。它不是第三方 Codex 客户端，也不是 Codex CLI。

## 安装后启动

可以从开始菜单打开 `Codex`，也可以用 PowerShell 启动：

```powershell
Start-Process "shell:AppsFolder\OpenAI.Codex_2p2nqsd0c76g0!App"
```

如果公司电脑禁用了 AppX/MSIX 旁加载或安装策略，需要管理员放行。缓存包不能绕过 Windows 设备策略。

## 官方安装方式

如果客户电脑可以正常访问 Microsoft Store，官方方式仍然可用：

```powershell
winget install Codex -s msstore
```

或使用 Store ProductId：

```powershell
winget install 9PLM9XGG6VKS -s msstore
```

官方页面：[Codex App for Windows](https://developers.openai.com/codex/app/windows)

## 和 Codex CLI 的区别

| 项目 | Codex App | Codex CLI |
| --- | --- | --- |
| 使用界面 | Windows 桌面 App | PowerShell / 终端 |
| 安装包 | MSIX / Microsoft Store | npm 包或 GitHub CLI 二进制 |
| 常见命令 | `Add-AppxPackage` / `winget install` | `npm install -g @openai/codex` |
| 适合场景 | 桌面工作区、官方登录态、App 入口 | 终端项目开发、本地命令行任务 |

如果教程里写的是 `codex --version`、`npm install -g @openai/codex`，那是在说 Codex CLI；如果下载的是 `OpenAI.Codex_...Msix`，那是在安装 Codex App。
