# Codex App Windows 下载

这页提供 **Codex App for Windows** 的下载和安装说明。这里说的是桌面版 **Codex App**，不是 `npm install -g @openai/codex` 安装的 **Codex CLI**。

## 首选下载：Codex App 缓存镜像

国内网络访问 Microsoft Store 有时不稳定，Windows 用户可以优先使用缓存镜像里的 MSIX 安装包：

[下载 Codex App Windows x64 MSIX](https://github.com/Wangnov/codex-app-mirror/releases/download/codex-app-force-20260613-033831/OpenAI.Codex_26.609.4994.0_x64__2p2nqsd0c76g0.Msix)

当前镜像信息：

| 项目 | 值 |
| --- | --- |
| 产品 | Codex App for Windows |
| 类型 | Windows 桌面 App，不是 Codex CLI |
| 文件 | `OpenAI.Codex_26.609.4994.0_x64__2p2nqsd0c76g0.Msix` |
| 架构 | x64 |
| 版本 | `26.609.4994.0` |
| 大小 | 约 552 MB |
| SHA256 | `547618a744149221078a27febdfff65c924b46ff85ab2fe1595180e128be8d85` |
| 镜像 Release | [codex-app-mirror](https://github.com/Wangnov/codex-app-mirror/releases/tag/codex-app-force-20260613-033831) |
| 官方 Store ProductId | `9PLM9XGG6VKS` |

这个缓存包来自 Codex App 的 Microsoft Store 包元数据镜像。它不是第三方 Codex 客户端，也不是 Codex CLI。

## PowerShell 安装命令

下载 MSIX 后，在文件所在目录打开 PowerShell：

```powershell
Add-AppxPackage -Path .\OpenAI.Codex_26.609.4994.0_x64__2p2nqsd0c76g0.Msix
```

安装完成后，可以从开始菜单打开 `Codex`，也可以用 PowerShell 启动：

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

## 机器可读版本信息

老实人AI 维护了一个轻量版本清单，方便后续同步下载页：

[Codex App Windows latest.json](/downloads/codex-app-windows/latest.json)

清单会记录当前推荐镜像、版本、文件名、SHA256、官方 ProductId 和安装命令。
