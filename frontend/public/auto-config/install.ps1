Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ScriptVersion = '0.1.0'
$DefaultBaseUrl = 'https://your-domain.example'
$DefaultTools = 'all'
$DefaultNodeIndexPrimary = 'https://npmmirror.com/mirrors/node/index.json'
$DefaultNodeIndexFallback = 'https://nodejs.org/dist/index.json'
$DefaultNodeDistPrimary = 'https://npmmirror.com/mirrors/node'
$DefaultNodeDistFallback = 'https://nodejs.org/dist'
$DefaultNpmRegistry = 'https://registry.npmmirror.com'
$FallbackNpmRegistry = 'https://registry.npmjs.org'
$MinNodeMajor = 20

$DragonHome = Join-Path $HOME '.dragoncode'
$NodeInstallRoot = Join-Path $DragonHome 'node'
$NodeCurrentDir = Join-Path $NodeInstallRoot 'current'
$NpmPrefix = Join-Path $DragonHome 'npm-global'
$ClaudeSettingsPath = Join-Path $HOME '.claude\settings.json'
$CodexDir = Join-Path $HOME '.codex'
$CodexAuthPath = Join-Path $CodexDir 'auth.json'
$CodexConfigPath = Join-Path $CodexDir 'config.toml'

# 支持通过环境变量传参，解决 `irm | iex` 管道模式下无法传命令行参数的问题
$BaseUrl = if ($env:DRAGON_BASE_URL) { $env:DRAGON_BASE_URL } else { $DefaultBaseUrl }
$Tools = if ($env:DRAGON_TOOLS) { $env:DRAGON_TOOLS.ToLowerInvariant() } else { $DefaultTools }
$ClaudeApiKey = $env:DRAGON_CLAUDE_API_KEY
$CodexApiKey = $env:DRAGON_CODEX_API_KEY
$NodeVersionOverride = if ($env:DRAGON_NODE_VERSION) { $env:DRAGON_NODE_VERSION } else { '' }
$SkipClientInstall = $env:DRAGON_SKIP_CLIENT_INSTALL -eq '1'

$script:NodeExe = ''
$script:NpmCmd = ''
$script:UseProxylessNpm = $false
$script:ActiveNpmRegistry = $DefaultNpmRegistry

# 输出信息日志，方便用户了解当前执行到了哪一步。
function Write-Info {
  param([string]$Message)

  Write-Host "[INFO] $Message"
}

# 输出警告日志，用于提示脚本已进入降级路径。
function Write-WarnMessage {
  param([string]$Message)

  Write-Warning $Message
}

# 输出错误日志并中止执行，避免异常状态继续写配置。
function Stop-Script {
  param([string]$Message)

  throw $Message
}

# 统一创建目录，避免文件写入前目录不存在。
function Ensure-Directory {
  param([string]$Path)

  if (-not (Test-Path -LiteralPath $Path)) {
    New-Item -ItemType Directory -Path $Path -Force | Out-Null
  }
}

# 首次改写配置时创建备份，方便用户回滚已有设置。
function Backup-IfNeeded {
  param([string]$TargetPath)

  $BackupPath = "$TargetPath.bak"
  if ((Test-Path -LiteralPath $TargetPath) -and -not (Test-Path -LiteralPath $BackupPath)) {
    Copy-Item -LiteralPath $TargetPath -Destination $BackupPath -Force
    Write-Info "已创建备份: $BackupPath"
  }
}

# 解析参数，支持直接通过一条命令非交互执行。
function Parse-Arguments {
  param([string[]]$ArgsList)

  for ($i = 0; $i -lt $ArgsList.Count; $i++) {
    $Current = $ArgsList[$i]

    switch ($Current) {
      '--api-key' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--api-key 需要一个值' }
        $script:ClaudeApiKey = $ArgsList[$i]
      }
      '--codex-api-key' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--codex-api-key 需要一个值' }
        $script:CodexApiKey = $ArgsList[$i]
      }
      '--base-url' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--base-url 需要一个值' }
        $script:BaseUrl = $ArgsList[$i]
      }
      '--tools' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--tools 需要一个值' }
        $Value = $ArgsList[$i].ToLowerInvariant()
        if ($Value -notin @('all', 'claude', 'codex')) {
          Stop-Script '不支持的 --tools 值，可选值为 all / claude / codex'
        }
        $script:Tools = $Value
      }
      '--node-version' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--node-version 需要一个值' }
        $script:NodeVersionOverride = $ArgsList[$i]
      }
      '--skip-client-install' {
        $script:SkipClientInstall = $true
      }
      '--help' {
        @'
Dragon Code 一键安装与自动配置脚本

用法:
  # 方式一：直接执行脚本文件，支持命令行参数
  .\install.ps1 --api-key <Claude_Key> --codex-api-key <Codex_Key> --tools all

  # 方式二：管道模式（irm | iex），参数通过环境变量传入
  $env:DRAGON_CLAUDE_API_KEY='<Key>'; $env:DRAGON_CODEX_API_KEY='<Key>'; irm https://your-domain.example/auto-config/install.ps1 | iex

  # 方式三：最简管道模式（交互输入 API Key）
  irm https://your-domain.example/auto-config/install.ps1 | iex

参数:
  --api-key              Claude Code API Key
  --codex-api-key        Codex API Key
  --tools                需要配置的工具，默认 all
  --base-url             API 基础地址，默认 https://your-domain.example
  --node-version         指定 Node.js 版本，例如 v24.11.0
  --skip-client-install  仅写配置，不安装 claude/codex 包
'@ | Write-Host
        exit 0
      }
      default {
        Stop-Script "未知参数: $Current"
      }
    }
  }
}

# 从 SecureString 安全读取用户输入并转换为明文字符串。
function Read-SecureInput {
  param([string]$Prompt)

  $SecureValue = Read-Host -Prompt $Prompt -AsSecureString
  $Pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecureValue)

  try {
    return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($Pointer)
  } finally {
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($Pointer)
  }
}

# 根据用户选择的工具范围，分别提示输入 Claude 和 Codex 的 API Key。
function Prompt-ApiKeys {
  # Claude API Key
  if ($script:Tools -in @('all', 'claude') -and [string]::IsNullOrWhiteSpace($script:ClaudeApiKey)) {
    $script:ClaudeApiKey = Read-SecureInput -Prompt '请输入 Claude Code API Key'
    if ([string]::IsNullOrWhiteSpace($script:ClaudeApiKey)) {
      Stop-Script 'Claude Code API Key 不能为空'
    }
  }

  # Codex API Key
  if ($script:Tools -in @('all', 'codex') -and [string]::IsNullOrWhiteSpace($script:CodexApiKey)) {
    $script:CodexApiKey = Read-SecureInput -Prompt '请输入 Codex API Key'
    if ([string]::IsNullOrWhiteSpace($script:CodexApiKey)) {
      Stop-Script 'Codex API Key 不能为空'
    }
  }
}

# 判断系统自带 Node.js 是否可复用，避免重复下载安装。
function Test-UsableSystemNode {
  $NodeCommand = Get-Command node -ErrorAction SilentlyContinue
  $NpmCommand = Get-Command npm -ErrorAction SilentlyContinue

  if ($null -eq $NodeCommand -or $null -eq $NpmCommand) {
    return $false
  }

  $VersionText = & $NodeCommand.Source --version
  $Major = [int](($VersionText -replace '^v', '').Split('.')[0])
  return $Major -ge $MinNodeMajor
}

# 从远端索引解析最新 LTS 版本，避免脚本内部硬编码 Node 版本。
function Resolve-NodeVersion {
  if (-not [string]::IsNullOrWhiteSpace($script:NodeVersionOverride)) {
    if ($script:NodeVersionOverride.StartsWith('v')) {
      return $script:NodeVersionOverride
    }
    return "v$($script:NodeVersionOverride)"
  }

  foreach ($IndexUrl in @($DefaultNodeIndexPrimary, $DefaultNodeIndexFallback)) {
    try {
      $Items = Invoke-RestMethod -Uri $IndexUrl
      $LtsItem = $Items | Where-Object { $_.lts -ne $false } | Select-Object -First 1
      if ($null -ne $LtsItem -and -not [string]::IsNullOrWhiteSpace($LtsItem.version)) {
        return $LtsItem.version
      }
    } catch {
      Write-WarnMessage "读取 Node 版本索引失败，尝试下一个地址: $IndexUrl"
    }
  }

  Stop-Script '无法获取 Node.js 最新 LTS 版本'
}

# 下载文件时优先使用国内镜像，失败后自动回退到官方源。
function Download-FileWithFallback {
  param(
    [string]$OutputPath,
    [string[]]$Urls
  )

  foreach ($Url in $Urls) {
    try {
      Invoke-WebRequest -Uri $Url -OutFile $OutputPath
      return
    } catch {
      Write-WarnMessage "下载失败，尝试下一个地址: $Url"
    }
  }

  Stop-Script '下载失败，请检查网络后重试'
}

# 下载并安装用户目录下的 Node.js 运行时，避免依赖管理员权限。
function Install-LocalNode {
  $Version = Resolve-NodeVersion
  $ArchName = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'x64' }
  } else {
    Stop-Script '当前仅支持 64 位 Windows'
  }

  $InstallDir = Join-Path $NodeInstallRoot "$Version\win-$ArchName"
  $ZipName = "node-$Version-win-$ArchName.zip"

  if (-not (Test-Path -LiteralPath (Join-Path $InstallDir 'node.exe'))) {
    $TempDir = Join-Path ([IO.Path]::GetTempPath()) ("dragon-auto-config-" + [guid]::NewGuid().ToString('N'))
    Ensure-Directory $TempDir

    $ZipPath = Join-Path $TempDir $ZipName
    Write-Info "正在下载 Node.js $Version (win-$ArchName)"

    Download-FileWithFallback -OutputPath $ZipPath -Urls @(
      "$DefaultNodeDistPrimary/$Version/$ZipName",
      "$DefaultNodeDistFallback/$Version/$ZipName"
    )

    Expand-Archive -Path $ZipPath -DestinationPath $TempDir -Force
    $ExtractedDir = Join-Path $TempDir "node-$Version-win-$ArchName"

    Ensure-Directory (Split-Path -Parent $InstallDir)
    if (Test-Path -LiteralPath $InstallDir) {
      Remove-Item -LiteralPath $InstallDir -Recurse -Force
    }
    Move-Item -LiteralPath $ExtractedDir -Destination $InstallDir
    Remove-Item -LiteralPath $TempDir -Recurse -Force
  }

  Ensure-Directory $NodeInstallRoot
  if (Test-Path -LiteralPath $NodeCurrentDir) {
    Remove-Item -LiteralPath $NodeCurrentDir -Recurse -Force
  }
  Copy-Item -LiteralPath $InstallDir -Destination $NodeCurrentDir -Recurse -Force

  $script:NodeExe = Join-Path $NodeCurrentDir 'node.exe'
  $script:NpmCmd = Join-Path $NodeCurrentDir 'npm.cmd'
}

# 统一确定本次执行使用的 Node/npm 路径。
function Ensure-NodeRuntime {
  if (Test-UsableSystemNode) {
    $script:NodeExe = (Get-Command node).Source
    $script:NpmCmd = (Get-Command npm).Source
    Write-Info "检测到可用系统 Node.js: $(& $script:NodeExe --version)"
    return
  }

  Write-WarnMessage '未检测到可用的 Node.js，开始安装本地运行时'
  Install-LocalNode
  Write-Info "本地 Node.js 已就绪: $(& $script:NodeExe --version)"
}

# 在常见安装路径和 PATH 中查找 bash.exe，返回完整路径或 $null。
function Find-GitBash {
  # 优先读取已有的环境变量
  if (-not [string]::IsNullOrWhiteSpace($env:CLAUDE_CODE_GIT_BASH_PATH) -and
      (Test-Path -LiteralPath $env:CLAUDE_CODE_GIT_BASH_PATH)) {
    return $env:CLAUDE_CODE_GIT_BASH_PATH
  }

  # 从 PATH 里的 git.exe 反推 bash.exe 位置
  $GitCmd = Get-Command git -ErrorAction SilentlyContinue
  if ($null -ne $GitCmd) {
    $BashPath = Join-Path (Split-Path -Parent (Split-Path -Parent $GitCmd.Source)) 'bin\bash.exe'
    if (Test-Path -LiteralPath $BashPath) {
      return $BashPath
    }
  }

  # 检查常见安装路径
  $CommonPaths = @(
    (Join-Path $env:ProgramFiles 'Git\bin\bash.exe'),
    (Join-Path $env:LOCALAPPDATA 'Programs\Git\bin\bash.exe')
  )
  # ProgramFiles(x86) 变量名含括号，需单独处理
  $PF86 = [Environment]::GetFolderPath('ProgramFilesX86')
  if (-not [string]::IsNullOrWhiteSpace($PF86)) {
    $CommonPaths += Join-Path $PF86 'Git\bin\bash.exe'
  }

  foreach ($Path in $CommonPaths) {
    if (Test-Path -LiteralPath $Path) {
      return $Path
    }
  }

  return $null
}

# 下载并静默安装 Git for Windows，优先从 npmmirror 镜像下载，失败后降级到 GitHub。
function Install-Git {
  Write-Info '正在安装 Git for Windows'

  # 从 GitHub API 获取最新版本元数据（仅元数据，不走 GitHub 下载）
  $ArchSuffix = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { '64-bit' }
  $MirrorBase = 'https://registry.npmmirror.com/-/binary/git-for-windows'

  try {
    $Release = Invoke-RestMethod -Uri 'https://api.github.com/repos/git-for-windows/git/releases/latest'
    $Asset = $Release.assets | Where-Object { $_.name -match "Git-.*-$ArchSuffix\.exe$" } | Select-Object -First 1
    if ($null -eq $Asset) { Stop-Script '无法找到 Git 安装包下载地址' }

    # npmmirror 镜像优先，GitHub 作为降级备选
    $MirrorUrl = "$MirrorBase/$($Release.tag_name)/$($Asset.name)"
    $GitHubUrl = $Asset.browser_download_url
  } catch {
    Stop-Script "无法获取 Git 最新版本信息: $_"
  }

  $TempDir = Join-Path ([IO.Path]::GetTempPath()) ("dragon-git-" + [guid]::NewGuid().ToString('N'))
  Ensure-Directory $TempDir
  $InstallerPath = Join-Path $TempDir 'git-installer.exe'

  Write-Info "正在下载 Git 安装包 ($($Asset.name))"
  Download-FileWithFallback -OutputPath $InstallerPath -Urls @($MirrorUrl, $GitHubUrl)

  Write-Info '正在静默安装 Git'
  $Process = Start-Process -FilePath $InstallerPath `
    -ArgumentList '/VERYSILENT /NORESTART /NOCANCEL /SP- /CLOSEAPPLICATIONS /COMPONENTS="icons,ext\reg\shellhere,assoc,assoc_sh"' `
    -Wait -PassThru
  Remove-Item -LiteralPath $TempDir -Recurse -Force

  if ($Process.ExitCode -ne 0) {
    Stop-Script "Git 安装失败，退出码: $($Process.ExitCode)"
  }
}

# 确保 git-bash 可用，并将路径写入 CLAUDE_CODE_GIT_BASH_PATH 环境变量。
# Claude Code 在 Windows 上依赖 git-bash 运行，仅在安装 claude 时执行此步骤。
function Ensure-GitBash {
  if ($script:Tools -notin @('all', 'claude')) {
    return
  }

  $BashPath = Find-GitBash

  if ($null -eq $BashPath) {
    Install-Git

    # 安装后刷新当前会话的 PATH，使 git 命令立即可用
    $GitBinDir = Join-Path $env:ProgramFiles 'Git\bin'
    if (Test-Path -LiteralPath $GitBinDir) {
      $env:Path = "$GitBinDir;$env:Path"
    }

    $BashPath = Find-GitBash
    if ($null -eq $BashPath) {
      Stop-Script 'Git 安装完成但未找到 bash.exe，请手动设置 CLAUDE_CODE_GIT_BASH_PATH'
    }
    Write-Info "Git 安装完成: $BashPath"
  } else {
    Write-Info "检测到 git-bash: $BashPath"
  }

  # 写入用户环境变量，供 Claude Code 使用
  [Environment]::SetEnvironmentVariable('CLAUDE_CODE_GIT_BASH_PATH', $BashPath, 'User')
  $env:CLAUDE_CODE_GIT_BASH_PATH = $BashPath
}

# 判断当前代理环境变量是否指向本地代理，避免残留配置导致 npm 无法联网。
function Detect-BrokenLocalProxy {
  $ProxyValues = @(
    $env:HTTP_PROXY,
    $env:HTTPS_PROXY,
    $env:ALL_PROXY,
    $env:http_proxy,
    $env:https_proxy,
    $env:all_proxy
  )

  foreach ($Value in $ProxyValues) {
    if ([string]::IsNullOrWhiteSpace($Value)) {
      continue
    }

    if ($Value -match '127\.0\.0\.1|localhost|::1') {
      $script:UseProxylessNpm = $true
      return
    }
  }

  $script:UseProxylessNpm = $false
}

# 统一执行 npm 命令；当检测到失效本地代理时，临时清理代理环境变量再执行。
# 注意：PowerShell 哈希表 key 大小写不敏感，HTTP_PROXY 和 http_proxy 会冲突，
# 改用 [ordered]@{} 配合中性 key 名（k1~k6）保存原始值，避免 DuplicateKeyInHashLiteral 报错。
function Invoke-NpmCommand {
  param([string[]]$Arguments)

  $PreviousProxy = [ordered]@{
    k1 = $env:HTTP_PROXY
    k2 = $env:HTTPS_PROXY
    k3 = $env:ALL_PROXY
    k4 = $env:http_proxy
    k5 = $env:https_proxy
    k6 = $env:all_proxy
  }

  try {
    if ($script:UseProxylessNpm) {
      $env:HTTP_PROXY  = ''
      $env:HTTPS_PROXY = ''
      $env:ALL_PROXY   = ''
      $env:http_proxy  = ''
      $env:https_proxy = ''
      $env:all_proxy   = ''
    }

    & $script:NpmCmd @Arguments
  } finally {
    $env:HTTP_PROXY  = $PreviousProxy.k1
    $env:HTTPS_PROXY = $PreviousProxy.k2
    $env:ALL_PROXY   = $PreviousProxy.k3
    $env:http_proxy  = $PreviousProxy.k4
    $env:https_proxy = $PreviousProxy.k5
    $env:all_proxy   = $PreviousProxy.k6
  }
}

# 将所需目录追加到当前会话和用户级 PATH，保证新终端可直接使用命令。
# Windows 上 npm --prefix 安装的 .cmd 文件在 $NpmPrefix 根目录，同时也加上 $NpmPrefix\bin 兼容不同 npm 版本。
function Ensure-UserPath {
  $RequiredEntries = @(
    $NodeCurrentDir,
    $NpmPrefix,
    (Join-Path $NpmPrefix 'bin')
  ) | ForEach-Object { $_.TrimEnd('\') }

  $CurrentUserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
  if ($null -eq $CurrentUserPath) {
    $CurrentUserPath = ''
  }

  $Entries = $CurrentUserPath -split ';' | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
  # 用 @() 强制转为数组，防止单元素时被解包成字符串导致 += 变成字符串拼接
  [string[]]$Entries = @($Entries)
  foreach ($Entry in $RequiredEntries) {
    # 大小写不敏感比较，避免 Windows 路径重复追加
    if (-not ($Entries | Where-Object { $_ -ieq $Entry })) {
      $Entries += $Entry
    }
  }

  $NewPath = ($Entries | Select-Object -Unique) -join ';'
  [Environment]::SetEnvironmentVariable('Path', $NewPath, 'User')
  # 同时更新当前会话的 PATH，让后续校验步骤直接可用
  $env:Path = "$($RequiredEntries -join ';');$env:Path"
}

# 将 npm 切换到国内镜像，降低无代理环境的安装失败率。
function Ensure-NpmRegistry {
  param([string]$Registry)

  $script:ActiveNpmRegistry = $Registry
  try {
    Invoke-NpmCommand -Arguments @('config', 'set', 'registry', $script:ActiveNpmRegistry, '--location=user') | Out-Null
  } catch {
    Write-WarnMessage '设置 npm 镜像失败，后续安装将继续尝试默认配置'
  }
}

# 安装 npm 包时优先尝试国内镜像，失败后自动回退到官方 registry。
function Install-NpmPackageWithFallback {
  param([string]$PackageName)

  try {
    Invoke-NpmCommand -Arguments @('install', '-g', '--prefix', $NpmPrefix, '--registry', $script:ActiveNpmRegistry, $PackageName)
    return
  } catch {
    if ($script:ActiveNpmRegistry -ne $FallbackNpmRegistry) {
      Write-WarnMessage "从 $($script:ActiveNpmRegistry) 安装失败，切换到官方 registry 重试"
      Ensure-NpmRegistry -Registry $FallbackNpmRegistry
      Invoke-NpmCommand -Arguments @('install', '-g', '--prefix', $NpmPrefix, '--registry', $script:ActiveNpmRegistry, $PackageName)
      return
    }

    throw
  }
}

# 安装用户选择的客户端包，并全部写入用户目录而非系统目录。
function Install-RequestedClients {
  if ($script:SkipClientInstall) {
    Write-WarnMessage '已跳过客户端安装，仅写入配置文件'
    return
  }

  Ensure-Directory $NpmPrefix
  Detect-BrokenLocalProxy
  if ($script:UseProxylessNpm) {
    Write-WarnMessage '检测到本地代理环境变量，安装客户端时将临时绕过代理'
  }
  Ensure-NpmRegistry -Registry $DefaultNpmRegistry

  if ($script:Tools -in @('all', 'claude')) {
    Write-Info '正在安装 Claude Code'
    Install-NpmPackageWithFallback -PackageName '@anthropic-ai/claude-code@latest'
  }

  if ($script:Tools -in @('all', 'codex')) {
    Write-Info '正在安装 Codex'
    Install-NpmPackageWithFallback -PackageName '@openai/codex@latest'
  }
}

# 写入 Claude Code 配置，并尽量保留用户原有 JSON 字段。
function Write-ClaudeConfig {
  Backup-IfNeeded $ClaudeSettingsPath
  Ensure-Directory (Split-Path -Parent $ClaudeSettingsPath)

  if (Test-Path -LiteralPath $ClaudeSettingsPath) {
    try {
      $Config = Get-Content -LiteralPath $ClaudeSettingsPath -Raw | ConvertFrom-Json
    } catch {
      $Config = [pscustomobject]@{}
    }
  } else {
    $Config = [pscustomobject]@{}
  }

  if ($null -eq $Config) {
    $Config = [pscustomobject]@{}
  }

  # 严格模式下用 Where-Object 检查属性是否存在，避免直接访问 .Name 报错
  $HasEnv = $Config.PSObject.Properties | Where-Object { $_.Name -eq 'env' }
  if (-not $HasEnv -or $null -eq $Config.env) {
    $Config | Add-Member -NotePropertyName env -NotePropertyValue ([pscustomobject]@{}) -Force
  }

  $Config.env | Add-Member -NotePropertyName ANTHROPIC_BASE_URL -NotePropertyValue $BaseUrl -Force
  $Config.env | Add-Member -NotePropertyName ANTHROPIC_AUTH_TOKEN -NotePropertyValue $ClaudeApiKey -Force
  $Config.env | Add-Member -NotePropertyName CLAUDE_CODE_ATTRIBUTION_HEADER -NotePropertyValue '0' -Force

  $Config | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $ClaudeSettingsPath -Encoding UTF8
}

# 写入 Codex 的 auth.json，并仅覆盖 OPENAI_API_KEY 字段。
function Write-CodexAuthConfig {
  Backup-IfNeeded $CodexAuthPath
  Ensure-Directory (Split-Path -Parent $CodexAuthPath)

  if (Test-Path -LiteralPath $CodexAuthPath) {
    try {
      $Config = Get-Content -LiteralPath $CodexAuthPath -Raw | ConvertFrom-Json
    } catch {
      $Config = [pscustomobject]@{}
    }
  } else {
    $Config = [pscustomobject]@{}
  }

  if ($null -eq $Config) {
    $Config = [pscustomobject]@{}
  }

  $Config | Add-Member -NotePropertyName OPENAI_API_KEY -NotePropertyValue $CodexApiKey -Force
  # 用无 BOM 的 UTF-8 写入，Windows PowerShell 5 默认带 BOM，Codex (serde_json) 不认 BOM 会报错
  $json = $Config | ConvertTo-Json -Depth 20
  [System.IO.File]::WriteAllText($CodexAuthPath, $json, [System.Text.UTF8Encoding]::new($false))
}

# 写入 Codex 的 TOML 主配置，第一版采用备份后确定性覆盖策略。
function Write-CodexTomlConfig {
  Backup-IfNeeded $CodexConfigPath
  Ensure-Directory (Split-Path -Parent $CodexConfigPath)

  # 用无 BOM 的 UTF-8 写入，同上
  $toml = @"
model_provider = "OpenAI"
model = "gpt-5.4"
review_model = "gpt-5.4"
model_reasoning_effort = "high"
disable_response_storage = true
network_access = "enabled"

[model_providers.OpenAI]
name = "OpenAI"
base_url = "$BaseUrl"
wire_api = "responses"
requires_openai_auth = true
"@
  [System.IO.File]::WriteAllText($CodexConfigPath, $toml, [System.Text.UTF8Encoding]::new($false))
}

# 根据用户选择写入 Claude Code 配置。
function Configure-Claude {
  if ($script:Tools -in @('all', 'claude')) {
    Write-Info '正在写入 Claude Code 配置'
    Write-ClaudeConfig
  }
}

# 根据用户选择写入 Codex 配置。
function Configure-Codex {
  if ($script:Tools -in @('all', 'codex')) {
    Write-Info '正在写入 Codex 配置'
    Write-CodexAuthConfig
    Write-CodexTomlConfig
  }
}

# 通过绝对路径执行命令做一次最小自检，证明安装链路可用。
function Verify-ClientCommands {
  if ($script:SkipClientInstall) {
    return
  }

  if ($script:Tools -in @('all', 'claude')) {
    $ClaudeCmd = Join-Path $NpmPrefix 'claude.cmd'
    if (Test-Path -LiteralPath $ClaudeCmd) {
      try {
        & $ClaudeCmd --version | Out-Null
        Write-Info "Claude Code 验证通过"
      } catch {
        Write-WarnMessage "Claude Code 安装完成，但自检失败（可忽略，重新打开终端后再试）: $_"
      }
    } else {
      Write-WarnMessage "未找到 $ClaudeCmd，请重新打开终端后执行 claude --version 确认"
    }
  }

  if ($script:Tools -in @('all', 'codex')) {
    $CodexCmd = Join-Path $NpmPrefix 'codex.cmd'
    if (Test-Path -LiteralPath $CodexCmd) {
      try {
        & $CodexCmd --version | Out-Null
        Write-Info "Codex 验证通过"
      } catch {
        Write-WarnMessage "Codex 安装完成，但自检失败（可忽略，重新打开终端后再试）: $_"
      }
    } else {
      Write-WarnMessage "未找到 $CodexCmd，请重新打开终端后执行 codex --version 确认"
    }
  }
}

# 输出最终结果和下一步指引，帮助用户立即开始使用。
function Print-Summary {
  Write-Info 'Dragon Code 自动配置完成'
  Write-Host ''
  Write-Host "  - API 地址: $BaseUrl"
  Write-Host "  - 工具范围: $Tools"
  Write-Host "  - Claude 配置: $ClaudeSettingsPath"
  Write-Host "  - Codex 鉴权: $CodexAuthPath"
  Write-Host "  - Codex 配置: $CodexConfigPath"
  Write-Host ''
  Write-Host '建议重新打开 PowerShell，然后执行:'
  if ($script:Tools -in @('all', 'claude')) {
    Write-Host '  claude --version'
  }
  if ($script:Tools -in @('all', 'codex')) {
    Write-Host '  codex --version'
  }
}

# 组织整个安装流程，确保安装、配置、校验按固定顺序执行。
# 注意：通过 `irm | iex` 管道执行时，$args 始终为空，参数需通过环境变量传入。
#   命令行方式：.\install.ps1 --tools claude --api-key <key>
#   管道方式：  $env:DRAGON_TOOLS='claude'; $env:DRAGON_API_KEY='<key>'; irm ... | iex
function Main {
  # 仅在非管道（直接执行脚本）时才解析命令行参数
  if ($MyInvocation.InvocationName -ne '&' -and $args.Count -gt 0) {
    Parse-Arguments -ArgsList $args
  }
  Prompt-ApiKeys
  Ensure-NodeRuntime
  Ensure-GitBash
  Ensure-UserPath
  Install-RequestedClients
  Configure-Claude
  Configure-Codex
  Verify-ClientCommands
  Print-Summary
}

Main
