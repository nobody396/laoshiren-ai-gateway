# Node.js 环境安装指南

Claude Code、Gemini CLI、Codex 等工具均依赖 Node.js 运行环境，请先完成此步骤。

## Windows

- **方法一：官方下载（推荐）**
  前往 [Node.js 官网](https://nodejs.org/zh-cn/download/) 下载 LTS 版本，双击安装包按提示完成安装。
下载以后，一直下一步直到安装完成

- **方法二：使用 Chocolatey**
  ```bash
  choco install nodejs-lts
  ```
- **方法三：使用 Scoop**
  ```bash
  scoop install nodejs-lts
  ```

## macOS

- **方法一：使用 Homebrew（推荐）**
  ```bash
  brew install node
  ```
- **方法二：官方下载**
  前往 [Node.js 官网](https://nodejs.org/zh-cn/download/) 下载 macOS 安装包。

## Linux

- **Ubuntu / Debian**
  ```bash
  curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
  sudo apt-get install -y nodejs
  ```
- **CentOS / RHEL**
  ```bash
  curl -fsSL https://rpm.nodesource.com/setup_lts.x | sudo bash -
  sudo yum install -y nodejs
  ```

## 验证安装

```bash
node --version
npm --version
```

两个命令均输出版本号即表示安装成功。

## 切换 npm 镜像源（无科学上网环境）

如果安装依赖时速度较慢或超时，执行以下命令切换到国内镜像源：

```bash
npm config set registry https://registry.npmmirror.com
```

验证是否切换成功：

```bash
npm config get registry
```

输出 `https://registry.npmmirror.com` 即表示生效。
