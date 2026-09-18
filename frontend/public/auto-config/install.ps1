Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# BEGIN GENERATED MODEL CATALOG
$ScriptVersion = '0.7.25'
$CatalogOpenAIDefaultModel = 'gpt-5.6-sol'
$CatalogOpenAIContextWindow = 272000
$CatalogOpenAIAutoCompactTokenLimit = 258000
$CatalogAnthropicDefaultModel = 'claude-opus-5'
$CatalogGrokDefaultModel = 'grok-4.6'
$CatalogGrokDefaultDisplayName = 'Grok 4.6'
$CatalogGrokDefaultContextWindow = 500000
$CatalogGrokManagedModels = @(@{ Id = 'grok-4.5'; DisplayName = 'Grok 4.5'; ContextWindow = 500000 }, @{ Id = 'grok-4.6'; DisplayName = 'Grok 4.6'; ContextWindow = 500000 })
$CatalogGrokManagedModelSections = @('model.grok-4.5', 'model."grok-4.5"', 'model.grok-4.6', 'model."grok-4.6"')
$CatalogGeminiDefaultModel = 'gemini-3.7-flash'
$CatalogGeminiManagedModels = @('gemini-3.1-pro', 'gemini-3.7-flash', 'gemini-3.7-flash-high', 'gemini-3.8-flash')
$CatalogModelReasoningJson = '{"claude-fable-5-1":["low","medium","high","xhigh","max"],"claude-fable-5":["low","medium","high","xhigh","max"],"claude-haiku-4-5":[],"claude-opus-4-5":["low","medium","high","max"],"claude-opus-4-6":["low","medium","high","max"],"claude-opus-4-7":["low","medium","high","xhigh","max"],"claude-opus-4-8":["low","medium","high","xhigh","max"],"claude-opus-5":["low","medium","high","xhigh","max"],"claude-sonnet-4-6":["low","medium","high","max"],"claude-sonnet-5":["low","medium","high","xhigh","max"],"deepseek-v4-flash-0731":["low","high","max"],"deepseek-v4-pro-0813":["low","high","max"],"gemini-3.1-pro":["low","medium","high"],"gemini-3.7-flash":["low","medium","high"],"gemini-3.8-flash":["low","medium","high"],"glm-5.2":["none","minimal","low","medium","high","xhigh","max"],"glm-5.3":["low","high","max"],"gpt-5.3-codex-spark":["none"],"gpt-5.4-mini":["none","low","medium","high","xhigh"],"gpt-5.4":["none","low","medium","high","xhigh"],"gpt-5.5":["none","low","medium","high","xhigh"],"gpt-5.6-luna":["none","low","medium","high","xhigh","max"],"gpt-5.6-sol":["none","low","medium","high","xhigh","max"],"gpt-5.6-terra":["none","low","medium","high","xhigh","max"],"gpt-daybreak-blue-latest":[],"grok-4.5":["low","medium","high","xhigh"],"grok-4.6":["low","medium","high","xhigh"],"kimi-k2.7-code":["always_on"],"kimi-k3":["low","high","max"],"minimax-m3":["disabled","adaptive"],"qwen3.6-flash":["none","minimal","low","medium"],"qwen3.6-plus":["none","minimal","low","medium"],"qwen3.7-flash":["none","minimal","low","medium"],"qwen3.7-max":["none","minimal","low","medium","high","xhigh","max"],"qwen3.7-plus":["none","minimal","low","medium","high","xhigh","max"],"qwen3.8-max":["none","minimal","low","medium","high","xhigh","max"]}'
# END GENERATED MODEL CATALOG
$CatalogOpenAIReasoningEffort = 'high'
$DefaultBaseUrl = 'https://api.laoshirenai.com'
$DefaultSetupExchangeUrl = 'https://laoshirenai.com/api/v1/public-setup/exchange'
$DefaultCodexManifestUrl = 'https://laoshirenai.com/api/v1/public-downloads/codex/latest.json'
$DefaultGitForWindowsManifestUrl = 'https://laoshirenai.com/api/v1/public-downloads/git-for-windows/latest.json'
$DefaultGrokBuildManifestUrl = 'https://laoshirenai.com/api/v1/public-downloads/grok-build/latest.json'
$DefaultCodexPackagePrefix = 'https://laoshirenai.com/downloads/codex/'
$DefaultGitForWindowsPackagePrefix = 'https://laoshirenai.com/downloads/git-for-windows/'
$DefaultGrokBuildPackagePrefix = 'https://laoshirenai.com/downloads/grok-build/'
$DefaultCodexModelCatalogUrl = "https://laoshirenai.com/auto-config/codex-model-catalog.json?v=$ScriptVersion"
$DefaultGrokCcSwitchImporterUrl = "https://laoshirenai.com/auto-config/import-grok-cc-switch-provider.cjs?v=$ScriptVersion"
$DefaultCodexAppInstallerUrl = 'https://laoshirenai.com/api/v1/public-downloads/codex/windows-x64/latest.appinstaller'
$DefaultTopupUrl = 'https://laoshirenai.com/get-subscription'
$DefaultTools = 'all'
$DefaultNodeIndexPrimary = 'https://npmmirror.com/mirrors/node/index.json'
$DefaultNodeIndexFallback = 'https://nodejs.org/dist/index.json'
$DefaultNodeDistPrimary = 'https://npmmirror.com/mirrors/node'
$DefaultNodeDistFallback = 'https://nodejs.org/dist'
$DefaultNpmRegistry = 'https://registry.npmmirror.com'
$FallbackNpmRegistry = 'https://registry.npmjs.org'
$MinNodeMajor = 20

$LaoshirenaiHome = Join-Path $HOME '.laoshirenai'
$NodeInstallRoot = Join-Path $LaoshirenaiHome 'node'
$NodeCurrentDir = Join-Path $NodeInstallRoot 'current'
$NpmPrefix = Join-Path $LaoshirenaiHome 'npm-global'
$ClaudeSettingsPath = Join-Path $HOME '.claude\settings.json'
$CodexDir = Join-Path $HOME '.codex'
$CodexAuthPath = Join-Path $CodexDir 'auth.json'
$CodexConfigPath = Join-Path $CodexDir 'config.toml'
$CodexModelCatalogPath = Join-Path $CodexDir 'laoshirenai-model-catalog.json'
$GrokDir = Join-Path $HOME '.grok'
$GrokConfigPath = Join-Path $GrokDir 'config.toml'
$GrokBinDir = Join-Path $GrokDir 'bin'
$GrokCommandPath = Join-Path $GrokBinDir 'grok.exe'
$GeminiDir = Join-Path $HOME '.gemini'
$GeminiEnvPath = Join-Path $GeminiDir '.env'
$GeminiSettingsPath = Join-Path $GeminiDir 'settings.json'
$KimiDir = Join-Path $HOME '.kimi-code'
$KimiConfigPath = Join-Path $KimiDir 'config.toml'
$OpenCodeConfigRoot = if ($env:XDG_CONFIG_HOME) { $env:XDG_CONFIG_HOME } else { Join-Path $HOME '.config' }
$OpenCodeDir = Join-Path $OpenCodeConfigRoot 'opencode'
$OpenCodeConfigPath = Join-Path $OpenCodeDir 'opencode.json'
$ZCodeDir = Join-Path $HOME '.zcode'
$ZCodeAppConfigPath = Join-Path $ZCodeDir 'v2\config.json'
$ZCodeCliConfigPath = Join-Path $ZCodeDir 'cli\config.json'
$WorkBuddyDir = Join-Path $HOME '.workbuddy'
$WorkBuddyModelsPath = Join-Path $WorkBuddyDir 'models.json'

# 支持通过环境变量传参，解决 `irm | iex` 管道模式下无法传命令行参数的问题
$BaseUrl = if ($env:LAOSHIRENAI_BASE_URL) { $env:LAOSHIRENAI_BASE_URL } else { $DefaultBaseUrl }
$Tools = if ($env:LAOSHIRENAI_TOOLS) { $env:LAOSHIRENAI_TOOLS.ToLowerInvariant() } else { $DefaultTools }
$ClaudeApiKey = $env:LAOSHIRENAI_CLAUDE_API_KEY
$CodexApiKey = $env:LAOSHIRENAI_CODEX_API_KEY
$GrokApiKey = $env:LAOSHIRENAI_GROK_API_KEY
$GeminiApiKey = $env:LAOSHIRENAI_GEMINI_API_KEY
$KimiApiKey = $env:LAOSHIRENAI_KIMI_API_KEY
$OpenCodeApiKey = $env:LAOSHIRENAI_OPENCODE_API_KEY
$ZCodeApiKey = $env:LAOSHIRENAI_ZCODE_API_KEY
$WorkBuddyApiKey = $env:LAOSHIRENAI_WORKBUDDY_API_KEY
$UnifiedApiKey = $env:LAOSHIRENAI_API_KEY
if ([string]::IsNullOrWhiteSpace($ClaudeApiKey) -and -not [string]::IsNullOrWhiteSpace($UnifiedApiKey)) {
  $ClaudeApiKey = $UnifiedApiKey
}
if ([string]::IsNullOrWhiteSpace($CodexApiKey) -and -not [string]::IsNullOrWhiteSpace($UnifiedApiKey)) {
  $CodexApiKey = $UnifiedApiKey
}
if ([string]::IsNullOrWhiteSpace($GrokApiKey) -and -not [string]::IsNullOrWhiteSpace($UnifiedApiKey)) {
  $GrokApiKey = $UnifiedApiKey
}
if ([string]::IsNullOrWhiteSpace($GeminiApiKey) -and -not [string]::IsNullOrWhiteSpace($UnifiedApiKey)) {
  $GeminiApiKey = $UnifiedApiKey
}
if ([string]::IsNullOrWhiteSpace($KimiApiKey) -and -not [string]::IsNullOrWhiteSpace($UnifiedApiKey)) {
  $KimiApiKey = $UnifiedApiKey
}
if ([string]::IsNullOrWhiteSpace($OpenCodeApiKey) -and -not [string]::IsNullOrWhiteSpace($UnifiedApiKey)) {
  $OpenCodeApiKey = $UnifiedApiKey
}
if ([string]::IsNullOrWhiteSpace($ZCodeApiKey) -and -not [string]::IsNullOrWhiteSpace($UnifiedApiKey)) {
  $ZCodeApiKey = $UnifiedApiKey
}
if ([string]::IsNullOrWhiteSpace($WorkBuddyApiKey) -and -not [string]::IsNullOrWhiteSpace($UnifiedApiKey)) {
  $WorkBuddyApiKey = $UnifiedApiKey
}
$NodeVersionOverride = if ($env:LAOSHIRENAI_NODE_VERSION) { $env:LAOSHIRENAI_NODE_VERSION } else { '' }
$SkipClientInstall = $env:LAOSHIRENAI_SKIP_CLIENT_INSTALL -eq '1'
$ForceClientInstall = $env:LAOSHIRENAI_FORCE_CLIENT_INSTALL -eq '1'
$InstallCodexApp = $env:LAOSHIRENAI_INSTALL_CODEX_APP -eq '1'
$GrokCcSwitchCompat = $env:LAOSHIRENAI_GROK_CC_SWITCH_COMPAT -eq '1'
$SetupToken = if ($env:LAOSHIRENAI_SETUP_TOKEN) { $env:LAOSHIRENAI_SETUP_TOKEN } else { '' }
$SelectedModel = if ($env:LAOSHIRENAI_MODEL_ID) { $env:LAOSHIRENAI_MODEL_ID } else { '' }
$SelectedProtocol = if ($env:LAOSHIRENAI_PROTOCOL) { $env:LAOSHIRENAI_PROTOCOL } else { '' }
$SelectedReasoning = if ($env:LAOSHIRENAI_REASONING_EFFORT) { $env:LAOSHIRENAI_REASONING_EFFORT.ToLowerInvariant() } else { '' }
$script:GrokApiBackend = 'responses'
$SetupExchangeUrl = if ($env:LAOSHIRENAI_SETUP_EXCHANGE_URL) { $env:LAOSHIRENAI_SETUP_EXCHANGE_URL } else { $DefaultSetupExchangeUrl }
$CodexManifestUrl = if ($env:LAOSHIRENAI_CODEX_MANIFEST_URL) { $env:LAOSHIRENAI_CODEX_MANIFEST_URL } else { $DefaultCodexManifestUrl }
$GitForWindowsManifestUrl = if ($env:LAOSHIRENAI_GIT_FOR_WINDOWS_MANIFEST_URL) { $env:LAOSHIRENAI_GIT_FOR_WINDOWS_MANIFEST_URL } else { $DefaultGitForWindowsManifestUrl }
$GrokBuildManifestUrl = if ($env:LAOSHIRENAI_GROK_BUILD_MANIFEST_URL) { $env:LAOSHIRENAI_GROK_BUILD_MANIFEST_URL } else { $DefaultGrokBuildManifestUrl }
$CodexPackagePrefix = if ($env:LAOSHIRENAI_CODEX_PACKAGE_PREFIX) { $env:LAOSHIRENAI_CODEX_PACKAGE_PREFIX } else { $DefaultCodexPackagePrefix }
$GitForWindowsPackagePrefix = if ($env:LAOSHIRENAI_GIT_FOR_WINDOWS_PACKAGE_PREFIX) { $env:LAOSHIRENAI_GIT_FOR_WINDOWS_PACKAGE_PREFIX } else { $DefaultGitForWindowsPackagePrefix }
$GrokBuildPackagePrefix = if ($env:LAOSHIRENAI_GROK_BUILD_PACKAGE_PREFIX) { $env:LAOSHIRENAI_GROK_BUILD_PACKAGE_PREFIX } else { $DefaultGrokBuildPackagePrefix }
$GrokCcSwitchImporterUrl = if ($env:LAOSHIRENAI_GROK_CC_SWITCH_IMPORTER_URL) { $env:LAOSHIRENAI_GROK_CC_SWITCH_IMPORTER_URL } else { $DefaultGrokCcSwitchImporterUrl }
$CodexAppInstallerUrl = if ($env:LAOSHIRENAI_CODEX_APPINSTALLER_URL) { $env:LAOSHIRENAI_CODEX_APPINSTALLER_URL } else { $DefaultCodexAppInstallerUrl }
$script:BalanceReady = $true

$script:NodeExe = ''
$script:NpmCmd = ''
$script:UseProxylessNpm = $false
$script:ActiveNpmRegistry = $DefaultNpmRegistry
$script:InstallClaudeClient = $false
$script:InstallCodexClient = $false
$script:InstallGrokClient = $false
$script:InstallGeminiClient = $false
$script:InstallKimiClient = $false
$script:InstallOpenCodeClient = $false
$script:ExistingClaudeCommand = ''
$script:ExistingCodexCommand = ''
$script:ExistingGrokCommand = ''
$script:ExistingGeminiCommand = ''
$script:ExistingKimiCommand = ''
$script:ExistingOpenCodeCommand = ''
$script:ExistingCodexApp = ''

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
      '--grok-api-key' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--grok-api-key 需要一个值' }
        $script:GrokApiKey = $ArgsList[$i]
      }
      '--gemini-api-key' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--gemini-api-key 需要一个值' }
        $script:GeminiApiKey = $ArgsList[$i]
      }
      '--kimi-api-key' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--kimi-api-key 需要一个值' }
        $script:KimiApiKey = $ArgsList[$i]
      }
      '--opencode-api-key' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--opencode-api-key 需要一个值' }
        $script:OpenCodeApiKey = $ArgsList[$i]
      }
      '--zcode-api-key' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--zcode-api-key 需要一个值' }
        $script:ZCodeApiKey = $ArgsList[$i]
      }
      '--workbuddy-api-key' {
        $i++
        if ($i -ge $ArgsList.Count) { Stop-Script '--workbuddy-api-key 需要一个值' }
        $script:WorkBuddyApiKey = $ArgsList[$i]
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
        if ($Value -notin @('all', 'claude', 'codex', 'grok', 'gemini', 'kimi', 'opencode', 'zcode', 'workbuddy')) {
          Stop-Script '不支持的 --tools 值，可选值为 all / claude / codex / grok / gemini / kimi / opencode / zcode / workbuddy'
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
      '--force-client-install' {
        $script:ForceClientInstall = $true
      }
      '--install-codex-app' {
        $script:InstallCodexApp = $true
      }
      '--help' {
        @'
老实人 AI 一键安装与自动配置脚本

用法:
  # 方式一：直接执行脚本文件，支持命令行参数
  .\install.ps1 --api-key <Claude_Key> --codex-api-key <Codex_Key> --grok-api-key <Grok_Key> --tools grok

  # 方式二：管道模式（irm | iex），参数通过环境变量传入
  $env:LAOSHIRENAI_CLAUDE_API_KEY='<Key>'; $env:LAOSHIRENAI_CODEX_API_KEY='<Key>'; irm https://laoshirenai.com/auto-config/install.ps1?v=0.7.25 | iex

  # 方式三：最简管道模式（交互输入 API Key）
  irm https://laoshirenai.com/auto-config/install.ps1?v=0.7.25 | iex

参数:
  --api-key              Claude Code API Key
  --codex-api-key        Codex API Key
  --grok-api-key         Grok Build API Key
  --gemini-api-key       Gemini CLI API Key
  --kimi-api-key         Kimi Code API Key
  --opencode-api-key     OpenCode API Key
  --zcode-api-key        ZCode API Key
  --workbuddy-api-key    WorkBuddy API Key
  --tools                需要配置的工具，默认 all
  --base-url             API 基础地址，默认 https://api.laoshirenai.com
  --node-version         指定 Node.js 版本，例如 v24.11.0
  --skip-client-install  仅写配置，不安装客户端
  --force-client-install 即使检测到已有客户端，也重新安装所选 CLI
  --install-codex-app    同时安装或更新与当前 Windows 架构匹配的 Codex App
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
  if (-not [string]::IsNullOrWhiteSpace($script:SetupToken)) {
    Write-Info '检测到一次性安装凭证，将自动领取对应客户端的专用配置'
    return
  }
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

  if ($script:Tools -eq 'grok' -and [string]::IsNullOrWhiteSpace($script:GrokApiKey)) {
    $script:GrokApiKey = Read-SecureInput -Prompt '请输入 Grok Build API Key'
    if ([string]::IsNullOrWhiteSpace($script:GrokApiKey)) {
      Stop-Script 'Grok Build API Key 不能为空'
    }
  }

  if ($script:Tools -eq 'gemini' -and [string]::IsNullOrWhiteSpace($script:GeminiApiKey)) {
    $script:GeminiApiKey = Read-SecureInput -Prompt '请输入 Gemini CLI API Key'
    if ([string]::IsNullOrWhiteSpace($script:GeminiApiKey)) {
      Stop-Script 'Gemini CLI API Key 不能为空'
    }
  }
  if ($script:Tools -eq 'kimi' -and [string]::IsNullOrWhiteSpace($script:KimiApiKey)) {
    $script:KimiApiKey = Read-SecureInput -Prompt '请输入 Kimi Code API Key'
    if ([string]::IsNullOrWhiteSpace($script:KimiApiKey)) { Stop-Script 'Kimi Code API Key 不能为空' }
  }
  if ($script:Tools -eq 'opencode' -and [string]::IsNullOrWhiteSpace($script:OpenCodeApiKey)) {
    $script:OpenCodeApiKey = Read-SecureInput -Prompt '请输入 OpenCode API Key'
    if ([string]::IsNullOrWhiteSpace($script:OpenCodeApiKey)) { Stop-Script 'OpenCode API Key 不能为空' }
  }
  if ($script:Tools -eq 'zcode' -and [string]::IsNullOrWhiteSpace($script:ZCodeApiKey)) {
    $script:ZCodeApiKey = Read-SecureInput -Prompt '请输入 ZCode API Key'
    if ([string]::IsNullOrWhiteSpace($script:ZCodeApiKey)) { Stop-Script 'ZCode API Key 不能为空' }
  }
  if ($script:Tools -eq 'workbuddy' -and [string]::IsNullOrWhiteSpace($script:WorkBuddyApiKey)) {
    $script:WorkBuddyApiKey = Read-SecureInput -Prompt '请输入 WorkBuddy API Key'
    if ([string]::IsNullOrWhiteSpace($script:WorkBuddyApiKey)) { Stop-Script 'WorkBuddy API Key 不能为空' }
  }
}

# 用一次性凭证领取当前目标的专用 API Key。凭证和 Key 均不会打印到终端。
function Exchange-SetupTicket {
  if ([string]::IsNullOrWhiteSpace($script:SetupToken)) {
    return
  }

  Write-Info '正在领取一次性安装配置'
  try {
    $Response = Invoke-RestMethod -Uri $script:SetupExchangeUrl `
      -Method POST `
      -ContentType 'application/json' `
      -Body (@{ ticket = $script:SetupToken } | ConvertTo-Json -Compress)
  } catch {
    Stop-Script '一次性配置命令无效、已过期或已使用，请回到 API 密钥页或安装与下载页重新生成'
  }

  $Data = $Response.data
  if ($null -eq $Data -or
      $Data.target -notin @('claude', 'codex', 'grok', 'gemini', 'kimi', 'opencode', 'zcode', 'workbuddy') -or
      [string]::IsNullOrWhiteSpace([string]$Data.api_key) -or
      [string]::IsNullOrWhiteSpace([string]$Data.base_url) -or
      (-not [string]::IsNullOrWhiteSpace([string]$Data.client_id) -and ([string]::IsNullOrWhiteSpace([string]$Data.model_id) -or [string]::IsNullOrWhiteSpace([string]$Data.protocol)))) {
    Stop-Script '服务器返回的一键安装配置格式无效'
  }
  if ([string]$Data.target -ne $script:Tools) {
    Stop-Script '安装凭证与当前工具不匹配，请重新生成'
  }

  $script:BaseUrl = [string]$Data.base_url
  $script:SelectedModel = [string]$Data.model_id
  $script:SelectedProtocol = [string]$Data.protocol
  if ($Data.target -eq 'claude') {
    $script:ClaudeApiKey = [string]$Data.api_key
    if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel)) { $script:CatalogAnthropicDefaultModel = $script:SelectedModel }
  } elseif ($Data.target -eq 'codex') {
    $script:CodexApiKey = [string]$Data.api_key
  } elseif ($Data.target -eq 'gemini') {
    $script:GeminiApiKey = [string]$Data.api_key
    if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel)) {
      $script:CatalogGeminiDefaultModel = $script:SelectedModel
      $script:CatalogGeminiManagedModels = @($script:SelectedModel)
    }
  } elseif ($Data.target -eq 'kimi') {
    $script:KimiApiKey = [string]$Data.api_key
  } elseif ($Data.target -eq 'opencode') {
    $script:OpenCodeApiKey = [string]$Data.api_key
  } elseif ($Data.target -eq 'zcode') {
    $script:ZCodeApiKey = [string]$Data.api_key
  } elseif ($Data.target -eq 'workbuddy') {
    $script:WorkBuddyApiKey = [string]$Data.api_key
  } else {
    $script:GrokApiKey = [string]$Data.api_key
    if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel)) {
      $script:CatalogGrokDefaultModel = $script:SelectedModel
      $script:CatalogGrokDefaultDisplayName = $script:SelectedModel
      $script:GrokApiBackend = $script:SelectedProtocol
      $script:CatalogGrokManagedModels = @([pscustomobject]@{ Id = $script:SelectedModel; DisplayName = $script:SelectedModel; ContextWindow = $null })
      $script:CatalogGrokManagedModelSections = @("model.$($script:SelectedModel)", "model.`"$($script:SelectedModel)`"")
    }
  }
  $script:SetupToken = ''
  Remove-Item Env:LAOSHIRENAI_SETUP_TOKEN -ErrorAction SilentlyContinue
  Write-Info '专用配置领取成功'
}

# 手动路径（无一次性票据）下，页面表单选择通过 LAOSHIRENAI_MODEL_ID 等环境变量
# 传入。这里把它们应用到各客户端真正写入的默认值，与票据路径保持一致；
# 否则手写配置会静默落回内置默认模型与默认协议。
function Apply-ManualSelection {
  if ([string]::IsNullOrWhiteSpace($script:SelectedModel)) { return }
  switch ($script:Tools) {
    'claude' {
      $script:CatalogAnthropicDefaultModel = $script:SelectedModel
    }
    'gemini' {
      $script:CatalogGeminiDefaultModel = $script:SelectedModel
      $script:CatalogGeminiManagedModels = @($script:SelectedModel)
    }
    'grok' {
      $script:CatalogGrokDefaultModel = $script:SelectedModel
      $script:CatalogGrokDefaultDisplayName = $script:SelectedModel
      $script:CatalogGrokManagedModels = @([pscustomobject]@{ Id = $script:SelectedModel; DisplayName = $script:SelectedModel; ContextWindow = $null })
      $script:CatalogGrokManagedModelSections = @("model.$($script:SelectedModel)", "model.`"$($script:SelectedModel)`"")
      if (-not [string]::IsNullOrWhiteSpace($script:SelectedProtocol)) {
        if (@('responses', 'chat_completions', 'messages') -contains $script:SelectedProtocol) {
          $script:GrokApiBackend = $script:SelectedProtocol
        } else {
          Stop-Script "Grok Build 不支持协议: $($script:SelectedProtocol)"
        }
      }
    }
  }
}

# 查找一个真正可执行的现有 CLI。Windows npm 同时生成 .cmd 和 .ps1 shim；
# 必须优先执行 .cmd，否则 Restricted 执行策略会拦截同名 .ps1。
function Get-UsableClientCommand {
  param([string]$CommandName)

  $Command = @(
    Get-Command "$CommandName.cmd" -CommandType Application -ErrorAction SilentlyContinue
    Get-Command "$CommandName.exe" -CommandType Application -ErrorAction SilentlyContinue
    Get-Command $CommandName -CommandType Application -ErrorAction SilentlyContinue
  ) | Select-Object -First 1
  if ($null -eq $Command) {
    return ''
  }

  $CommandPath = [string]$Command.Source
  if ([string]::IsNullOrWhiteSpace($CommandPath)) {
    $PathProperty = $Command.PSObject.Properties['Path']
    if ($null -ne $PathProperty) {
      $CommandPath = [string]$PathProperty.Value
    }
  }
  if ([string]::IsNullOrWhiteSpace($CommandPath)) {
    return ''
  }

  try {
    & $CommandPath --version *> $null
    return $CommandPath
  } catch {
    Write-WarnMessage "检测到 $CommandName 命令，但它当前无法运行，将按缺失客户端处理: $CommandPath"
    return ''
  }
}

# 检测当前 Windows 用户是否已经安装官方 Codex App。
# App 与 npm 安装的 Codex CLI 是两种客户端；已有 App 时无需为了写配置再下载一份 CLI。
function Get-InstalledCodexApp {
  if ($null -ne (Get-Command Get-AppxPackage -ErrorAction SilentlyContinue)) {
    try {
      $Package = Get-AppxPackage -Name 'OpenAI.Codex' -ErrorAction SilentlyContinue |
        Select-Object -First 1
      if ($null -ne $Package) {
        return [string]$Package.PackageFullName
      }
    } catch {
      Write-WarnMessage "读取 Codex App 安装状态失败，将继续检查开始菜单: $_"
    }
  }

  if ($null -ne (Get-Command Get-StartApps -ErrorAction SilentlyContinue)) {
    try {
      $StartApp = Get-StartApps |
        Where-Object { $_.AppID -like 'OpenAI.Codex*!App' } |
        Select-Object -First 1
      if ($null -ne $StartApp) {
        return [string]$StartApp.AppID
      }
    } catch {
      Write-WarnMessage "读取开始菜单中的 Codex App 状态失败: $_"
    }
  }

  return ''
}

# 为每个所选客户端制定安装计划：默认复用可用的现有安装，仅在缺失时安装 CLI。
function Resolve-ClientInstallPlan {
  if ($script:SkipClientInstall -and $script:ForceClientInstall) {
    Stop-Script '不能同时使用 --skip-client-install 和 --force-client-install'
  }

  $script:InstallClaudeClient = $false
  $script:InstallCodexClient = $false
  $script:InstallGrokClient = $false
  $script:InstallGeminiClient = $false
  $script:InstallKimiClient = $false
  $script:InstallOpenCodeClient = $false

  if ($script:Tools -in @('all', 'claude')) {
    $script:ExistingClaudeCommand = Get-UsableClientCommand -CommandName 'claude'
    if ($script:ForceClientInstall) {
      $script:InstallClaudeClient = $true
      Write-Info '已要求强制重新安装 Claude Code CLI'
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingClaudeCommand)) {
      Write-Info "检测到现有 Claude Code CLI，跳过重复安装: $($script:ExistingClaudeCommand)"
    } elseif ($script:SkipClientInstall) {
      Write-WarnMessage '未检测到可用的 Claude Code CLI，但已按要求跳过安装'
    } else {
      $script:InstallClaudeClient = $true
    }
  }

  if ($script:Tools -in @('all', 'codex')) {
    $script:ExistingCodexCommand = Get-UsableClientCommand -CommandName 'codex'
    if (-not [string]::IsNullOrWhiteSpace($script:ExistingCodexCommand)) {
      Write-Info "检测到现有 Codex CLI，跳过重复安装: $($script:ExistingCodexCommand)"
    }
    $script:ExistingCodexApp = Get-InstalledCodexApp
    if (-not [string]::IsNullOrWhiteSpace($script:ExistingCodexApp)) {
      Write-Info '检测到现有 Codex App；App 与 CLI 将分别检查，不再互相替代'
    }

    if ($script:ForceClientInstall) {
      $script:InstallCodexClient = $true
      Write-Info '已要求强制重新安装 Codex CLI'
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingCodexCommand)) {
      $script:InstallCodexClient = $false
    } elseif ($script:SkipClientInstall) {
      Write-WarnMessage '未检测到可用的 Codex CLI，但已按要求跳过安装'
    } else {
      $script:InstallCodexClient = $true
    }
  }

  if ($script:Tools -eq 'grok') {
    $script:ExistingGrokCommand = Get-UsableClientCommand -CommandName 'grok'
    if ($script:ForceClientInstall) {
      $script:InstallGrokClient = $true
      Write-Info '已要求强制重新安装 Grok Build'
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingGrokCommand)) {
      Write-Info "检测到现有 Grok Build，跳过重复安装: $($script:ExistingGrokCommand)"
    } elseif ($script:SkipClientInstall) {
      Write-WarnMessage '未检测到可用的 Grok Build，但已按要求跳过安装'
    } else {
      $script:InstallGrokClient = $true
    }
  }

  if ($script:Tools -eq 'gemini') {
    $script:ExistingGeminiCommand = Get-UsableClientCommand -CommandName 'gemini'
    if ($script:ForceClientInstall) {
      $script:InstallGeminiClient = $true
      Write-Info '已要求强制重新安装 Gemini CLI'
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingGeminiCommand)) {
      Write-Info "检测到现有 Gemini CLI，跳过重复安装: $($script:ExistingGeminiCommand)"
    } elseif ($script:SkipClientInstall) {
      Write-WarnMessage '未检测到可用的 Gemini CLI，但已按要求跳过安装'
    } else {
      $script:InstallGeminiClient = $true
    }
  }
  if ($script:Tools -eq 'kimi') {
    $script:ExistingKimiCommand = Get-UsableClientCommand -CommandName 'kimi'
    if ($script:ForceClientInstall) {
      $script:InstallKimiClient = $true
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingKimiCommand)) {
      Write-Info "检测到现有 Kimi Code，跳过重复安装: $($script:ExistingKimiCommand)"
    } elseif ($script:SkipClientInstall) {
      Write-WarnMessage '未检测到可用的 Kimi Code，但已按要求跳过安装'
    } else {
      $script:InstallKimiClient = $true
    }
  }
  if ($script:Tools -eq 'opencode') {
    $script:ExistingOpenCodeCommand = Get-UsableClientCommand -CommandName 'opencode'
    if ($script:ForceClientInstall) {
      $script:InstallOpenCodeClient = $true
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingOpenCodeCommand)) {
      Write-Info "检测到现有 OpenCode，跳过重复安装: $($script:ExistingOpenCodeCommand)"
    } elseif ($script:SkipClientInstall) {
      Write-WarnMessage '未检测到可用的 OpenCode，但已按要求跳过安装'
    } else {
      $script:InstallOpenCodeClient = $true
    }
  }
}

function Get-ClientVersion {
  param([string]$CommandPath)
  try {
    $Text = (& $CommandPath --version 2>$null | Out-String).Trim()
    $Match = [regex]::Match($Text, '\d+(?:\.\d+){1,3}')
    if ($Match.Success) { return $Match.Value }
  } catch {}
  return ''
}

function Get-LatestPackageVersion {
  param([string]$PackagePath)
  foreach ($Registry in @($DefaultNpmRegistry, $FallbackNpmRegistry)) {
    try {
      $Result = Invoke-RestMethod -Uri "$Registry/$PackagePath/latest" -Method GET
      if (-not [string]::IsNullOrWhiteSpace([string]$Result.version)) {
        return [string]$Result.version
      }
    } catch {
      Write-WarnMessage "读取 $Registry 最新版本失败，尝试下一个地址"
    }
  }
  return ''
}

function Test-VersionOlder {
  param([string]$Current, [string]$Latest)
  try { return ([version]$Current -lt [version]$Latest) } catch { return $false }
}

function Resolve-ClientUpdatePlan {
  if ($script:SkipClientInstall -or $script:ForceClientInstall) { return }

  $Checks = @(
    @{ Label = 'Claude Code CLI'; Command = $script:ExistingClaudeCommand; Package = '@anthropic-ai%2Fclaude-code'; Flag = 'InstallClaudeClient' },
    @{ Label = 'Codex CLI'; Command = $script:ExistingCodexCommand; Package = '@openai%2Fcodex'; Flag = 'InstallCodexClient' },
    @{ Label = 'Gemini CLI'; Command = $script:ExistingGeminiCommand; Package = '@google%2Fgemini-cli'; Flag = 'InstallGeminiClient' },
    @{ Label = 'Kimi Code CLI'; Command = $script:ExistingKimiCommand; Package = '@moonshot-ai%2Fkimi-code'; Flag = 'InstallKimiClient' },
    @{ Label = 'OpenCode CLI'; Command = $script:ExistingOpenCodeCommand; Package = 'opencode-ai'; Flag = 'InstallOpenCodeClient' }
  )
  foreach ($Check in $Checks) {
    if ([string]::IsNullOrWhiteSpace([string]$Check.Command) -or (Get-Variable -Scope Script -Name $Check.Flag).Value) { continue }
    $Current = Get-ClientVersion -CommandPath $Check.Command
    $Latest = Get-LatestPackageVersion -PackagePath $Check.Package
    if ([string]::IsNullOrWhiteSpace($Current) -or [string]::IsNullOrWhiteSpace($Latest)) {
      Write-WarnMessage "无法比较 $($Check.Label) 版本，本次保留现有可用版本并继续配置测试"
    } elseif (Test-VersionOlder -Current $Current -Latest $Latest) {
      Set-Variable -Scope Script -Name $Check.Flag -Value $true
      Write-Info "检测到 $($Check.Label) 可更新: $Current -> $Latest"
    } else {
      Write-Info "$($Check.Label) 已是当前版本: $Current"
    }
  }
}

function Test-NeedsClientInstall {
  return ($script:InstallClaudeClient -or $script:InstallCodexClient -or $script:InstallGrokClient -or $script:InstallGeminiClient -or $script:InstallKimiClient -or $script:InstallOpenCodeClient)
}

function Test-NeedsNpmClientInstall {
  return ($script:InstallClaudeClient -or $script:InstallCodexClient -or $script:InstallGeminiClient -or $script:InstallKimiClient -or $script:InstallOpenCodeClient)
}

# 只解析 npm.cmd，避免 PowerShell 在 Restricted 执行策略下优先命中 npm.ps1。
function Resolve-SystemNpmCmd {
  param([System.Management.Automation.CommandInfo]$NodeCommand)

  if ($null -ne $NodeCommand -and -not [string]::IsNullOrWhiteSpace([string]$NodeCommand.Source)) {
    $SiblingNpmCmd = Join-Path (Split-Path -Parent $NodeCommand.Source) 'npm.cmd'
    if (Test-Path -LiteralPath $SiblingNpmCmd) {
      return $SiblingNpmCmd
    }
  }

  $NpmCommand = Get-Command npm.cmd -CommandType Application -ErrorAction SilentlyContinue |
    Select-Object -First 1
  if ($null -eq $NpmCommand) {
    return ''
  }
  return [string]$NpmCommand.Source
}

# 判断系统自带 Node.js 是否可复用，避免重复下载安装。
function Test-UsableSystemNode {
  $NodeCommand = Get-Command node -CommandType Application -ErrorAction SilentlyContinue |
    Select-Object -First 1
  $NpmCmd = Resolve-SystemNpmCmd -NodeCommand $NodeCommand

  if ($null -eq $NodeCommand -or [string]::IsNullOrWhiteSpace($NpmCmd)) {
    return $false
  }

  $VersionText = & $NodeCommand.Source --version
  $Major = [int](($VersionText -replace '^v', '').Split('.')[0])
  if ($Major -lt $MinNodeMajor) { return $false }
  if ($script:GrokCcSwitchCompat -and (Test-UsesGrok)) {
    & $NodeCommand.Source --no-warnings -e 'require("node:sqlite").DatabaseSync' 2>$null
    if ($LASTEXITCODE -ne 0) { return $false }
  }
  return $true
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
      Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $OutputPath
      return
    } catch {
      Write-WarnMessage "下载失败，尝试下一个地址: $Url"
    }
  }

  Stop-Script '下载失败，请检查网络后重试'
}

function Get-NodeReleaseChecksum {
  param(
    [string]$Version,
    [string]$ZipName
  )

  foreach ($Base in @($DefaultNodeDistPrimary, $DefaultNodeDistFallback)) {
    try {
      $ChecksumUrl = "$($Base.TrimEnd('/', '\'))/$Version/SHASUMS256.txt"
      if (Test-Path -LiteralPath $ChecksumUrl -PathType Leaf) {
        $Checksums = Get-Content -LiteralPath $ChecksumUrl -Raw
      } else {
        $Checksums = (Invoke-WebRequest -Uri $ChecksumUrl -UseBasicParsing).Content
      }
      $Line = @($Checksums -split "`n" | Where-Object {
        $_ -match "^([a-fA-F0-9]{64})\s+$([regex]::Escape($ZipName))$"
      }) | Select-Object -First 1
      if ($null -ne $Line -and $Line -match '^([a-fA-F0-9]{64})') {
        return $Matches[1].ToLowerInvariant()
      }
    } catch {
      Write-WarnMessage "读取 Node.js 校验文件失败，尝试下一个地址: $Base"
    }
  }

  Stop-Script "无法获取 Node.js 安装包校验值: $ZipName"
}

function Download-VerifiedFileWithFallback {
  param(
    [string]$OutputPath,
    [string[]]$Urls,
    [string]$ExpectedSHA256
  )

  foreach ($Url in $Urls) {
    try {
      Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
      if (Test-Path -LiteralPath $Url -PathType Leaf) {
        Copy-Item -LiteralPath $Url -Destination $OutputPath -Force
      } else {
        Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $OutputPath
      }
      $ActualSHA256 = (Get-FileHash -LiteralPath $OutputPath -Algorithm SHA256).Hash.ToLowerInvariant()
      if ($ActualSHA256 -eq $ExpectedSHA256.ToLowerInvariant()) {
        return
      }
      Write-WarnMessage "下载文件 SHA256 不匹配，尝试下一个地址: $Url"
    } catch {
      Write-WarnMessage "下载失败，尝试下一个地址: $Url"
    }
  }

  Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
  Stop-Script '下载失败或 SHA256 校验不通过，已停止安装'
}

function Get-VerifiedSameSiteAsset {
  param(
    [string]$ManifestUrl,
    [string]$DownloadPrefix,
    [string]$Platform,
    [string]$Arch,
    [string]$NamePattern
  )

  try {
    $Manifest = Invoke-RestMethod -Uri $ManifestUrl -Method GET
  } catch {
    Stop-Script "无法读取本站安装包清单: $_"
  }
  $Assets = @($Manifest.assets | Where-Object {
    $_.platform -eq $Platform -and
    $_.arch -eq $Arch -and
    ([string]$_.name) -match $NamePattern
  })
  if ($Assets.Count -ne 1) {
    Stop-Script "本站安装包清单没有唯一匹配项: $Platform/$Arch"
  }

  $Asset = $Assets[0]
  $DownloadUrl = ([string]$Asset.download_url).Trim()
  $ExpectedSha = ([string]$Asset.sha256).Trim().ToLowerInvariant()
  if (-not $DownloadUrl.StartsWith($DownloadPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    Stop-Script '安装包下载地址未通过本站同源校验'
  }
  if ($ExpectedSha -notmatch '^[a-f0-9]{64}$') {
    Stop-Script '安装包清单缺少有效的 SHA256'
  }

  return [pscustomobject]@{
    Version = [string]$Manifest.version
    Name = [string]$Asset.name
    DownloadUrl = $DownloadUrl
    SHA256 = $ExpectedSha
  }
}

function Download-VerifiedAsset {
  param(
    [object]$Asset,
    [string]$OutputPath
  )

  Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
  for ($Attempt = 1; $Attempt -le 5; $Attempt++) {
    $CurlArgs = @('-fL', '--connect-timeout', '60', '--max-time', '3600', '-o', $OutputPath)
    if ((Test-Path -LiteralPath $OutputPath -PathType Leaf) -and
        (Get-Item -LiteralPath $OutputPath).Length -gt 0) {
      $CurlArgs += @('-C', '-')
    }
    $CurlArgs += [string]$Asset.DownloadUrl
    & curl.exe @CurlArgs
    $CurlExitCode = $LASTEXITCODE

    if ($CurlExitCode -eq 0 -and (Test-Path -LiteralPath $OutputPath -PathType Leaf)) {
      $ActualSha = (Get-FileHash -LiteralPath $OutputPath -Algorithm SHA256).Hash.ToLowerInvariant()
      if ($ActualSha -eq $Asset.SHA256) { return }
      Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
    } elseif ($CurlExitCode -eq 33) {
      Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
    }

    if ($Attempt -lt 5) {
      Write-WarnMessage "安装包下载中断，2 秒后从断点重试（$Attempt/5）"
      Start-Sleep -Seconds 2
    }
  }
  Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
  throw '安装包下载失败或 SHA256 校验不通过，已停止安装'
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
    $TempDir = Join-Path ([IO.Path]::GetTempPath()) ("laoshirenai-auto-config-" + [guid]::NewGuid().ToString('N'))
    Ensure-Directory $TempDir
    try {
      $ZipPath = Join-Path $TempDir $ZipName
      Write-Info "正在下载 Node.js $Version (win-$ArchName)"
      $ExpectedSHA256 = Get-NodeReleaseChecksum -Version $Version -ZipName $ZipName

      Download-VerifiedFileWithFallback -OutputPath $ZipPath -ExpectedSHA256 $ExpectedSHA256 -Urls @(
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
    } finally {
      Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
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
    $NodeCommand = Get-Command node -CommandType Application -ErrorAction Stop |
      Select-Object -First 1
    $script:NodeExe = [string]$NodeCommand.Source
    $script:NpmCmd = Resolve-SystemNpmCmd -NodeCommand $NodeCommand
    Write-Info "检测到可用系统 Node.js: $(& $script:NodeExe --version)"
    return
  }

  Write-WarnMessage '未检测到可用的 Node.js，开始安装本地运行时'
  Install-LocalNode
  if ($script:GrokCcSwitchCompat -and (Test-UsesGrok)) {
    & $script:NodeExe --no-warnings -e 'require("node:sqlite").DatabaseSync' 2>$null
    if ($LASTEXITCODE -ne 0) {
      Stop-Script '当前 Node.js 不支持 CC Switch 安全导入，请移除 LAOSHIRENAI_NODE_VERSION 覆盖后重试'
    }
  }
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

# 下载并静默安装本站已同步且通过 SHA-256 校验的 Git for Windows。
function Install-Git {
  Write-Info '正在安装 Git for Windows'


  $TargetArch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'x64' }
  $NamePattern = if ($TargetArch -eq 'arm64') { '(?i)^Git-.*-arm64\.exe$' } else { '(?i)^Git-.*-64-bit\.exe$' }
  $Asset = Get-VerifiedSameSiteAsset `
    -ManifestUrl $script:GitForWindowsManifestUrl `
    -DownloadPrefix $script:GitForWindowsPackagePrefix `
    -Platform 'windows' `
    -Arch $TargetArch `
    -NamePattern $NamePattern

  $TempDir = Join-Path ([IO.Path]::GetTempPath()) ("laoshirenai-git-" + [guid]::NewGuid().ToString('N'))
  Ensure-Directory $TempDir
  $InstallerPath = Join-Path $TempDir 'git-installer.exe'
  try {
    Write-Info "正在从本站缓存下载 Git 安装包 ($($Asset.Name))"
    Download-VerifiedAsset -Asset $Asset -OutputPath $InstallerPath

    Write-Info '正在按当前用户静默安装 Git'
    $Process = Start-Process -FilePath $InstallerPath `
      -ArgumentList '/CURRENTUSER /VERYSILENT /NORESTART /NOCANCEL /SP- /CLOSEAPPLICATIONS /COMPONENTS="icons,ext\reg\shellhere,assoc,assoc_sh"' `
      -Wait -PassThru
    if ($Process.ExitCode -ne 0) {
      Stop-Script "Git 安装失败，退出码: $($Process.ExitCode)"
    }
  } finally {
    Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue
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
    if ($LASTEXITCODE -ne 0) {
      throw "npm.cmd 执行失败，退出码: $LASTEXITCODE"
    }
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
    (Join-Path $NpmPrefix 'bin'),
    $GrokBinDir
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

# npm 在 Windows 会同时生成同名 .cmd 与 .ps1 启动器。PowerShell 会优先解析
# .ps1，而 Restricted 执行策略会在 CLI 启动前将它拦截。仅清理本站用户目录中
# 有同名 .cmd 兜底的 .ps1 shim，绝不修改系统 Node.js 安装目录或用户其他文件。
function Remove-ManagedPowerShellShims {
  $ManagedDirs = @(
    $NodeCurrentDir,
    $NpmPrefix,
    (Join-Path $NpmPrefix 'bin')
  ) | Select-Object -Unique

  foreach ($Dir in $ManagedDirs) {
    if (-not (Test-Path -LiteralPath $Dir -PathType Container)) {
      continue
    }
    Get-ChildItem -LiteralPath $Dir -Filter '*.ps1' -File -ErrorAction SilentlyContinue |
      ForEach-Object {
        $CmdPath = [IO.Path]::ChangeExtension($_.FullName, '.cmd')
        if (Test-Path -LiteralPath $CmdPath -PathType Leaf) {
          Remove-Item -LiteralPath $_.FullName -Force
          Write-Info "已启用兼容 Windows 执行策略的命令入口: $([IO.Path]::GetFileName($CmdPath))"
        }
      }
  }
}

function Install-GrokBuild {
  $NativeArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()
  $Arch = if ($NativeArch -eq 'Arm64') { 'aarch64' } elseif ($NativeArch -eq 'X64') { 'x86_64' } else { '' }
  if ([string]::IsNullOrWhiteSpace($Arch)) {
    Stop-Script "Grok Build 暂不支持当前 Windows 架构: $NativeArch"
  }

  $TargetArch = if ($Arch -eq 'aarch64') { 'arm64' } else { 'x64' }
  $NamePattern = "(?i)^grok-.*-windows-$([regex]::Escape($Arch))\.exe$"
  $Asset = Get-VerifiedSameSiteAsset `
    -ManifestUrl $script:GrokBuildManifestUrl `
    -DownloadPrefix $script:GrokBuildPackagePrefix `
    -Platform 'windows' `
    -Arch $TargetArch `
    -NamePattern $NamePattern

  $DownloadsDir = Join-Path $GrokDir 'downloads'
  Ensure-Directory $DownloadsDir
  Ensure-Directory $GrokBinDir
  $DownloadPath = Join-Path $DownloadsDir "grok-windows-$Arch.exe"
  $TemporaryPath = "$DownloadPath.tmp"
  Write-Info "正在从本站缓存下载 Grok Build $($Asset.Version) (windows-$Arch)"
  try {
    Download-VerifiedAsset -Asset $Asset -OutputPath $TemporaryPath
    Move-Item -LiteralPath $TemporaryPath -Destination $DownloadPath -Force
    Copy-Item -LiteralPath $DownloadPath -Destination $GrokCommandPath -Force
    Copy-Item -LiteralPath $DownloadPath -Destination (Join-Path $GrokBinDir 'agent.exe') -Force
  } catch {
    Remove-Item -LiteralPath $TemporaryPath -Force -ErrorAction SilentlyContinue
    Stop-Script "Grok Build 下载或安装失败: $_"
  }
}

# 安装用户选择的客户端包，并全部写入用户目录而非系统目录。
function Install-RequestedClients {
  if (-not (Test-NeedsClientInstall)) {
    Write-Info '所选客户端无需安装，本次仅写入配置并测试 API Key'
    return
  }

  if (Test-NeedsNpmClientInstall) {
    Ensure-Directory $NpmPrefix
    Detect-BrokenLocalProxy
    if ($script:UseProxylessNpm) {
      Write-WarnMessage '检测到本地代理环境变量，安装客户端时将临时绕过代理'
    }
    Ensure-NpmRegistry -Registry $DefaultNpmRegistry
  }

  if ($script:InstallClaudeClient) {
    Write-Info '正在安装或更新 Claude Code'
    Install-NpmPackageWithFallback -PackageName '@anthropic-ai/claude-code@latest'
  }

  if ($script:InstallCodexClient) {
    Write-Info '正在安装或更新 Codex'
    Install-NpmPackageWithFallback -PackageName '@openai/codex@latest'
  }

  if ($script:InstallGrokClient) {
    Install-GrokBuild
  }

  if ($script:InstallGeminiClient) {
    Write-Info '正在安装或更新 Gemini CLI'
    Install-NpmPackageWithFallback -PackageName '@google/gemini-cli@latest'
  }
  if ($script:InstallKimiClient) {
    Write-Info '正在安装或更新 Kimi Code'
    Install-NpmPackageWithFallback -PackageName '@moonshot-ai/kimi-code@latest'
  }
  if ($script:InstallOpenCodeClient) {
    Write-Info '正在安装或更新 OpenCode'
    Install-NpmPackageWithFallback -PackageName 'opencode-ai@latest'
  }
}

function Install-CodexAppIfRequested {
  if (-not $script:InstallCodexApp) {
    return
  }
  if (-not (Test-UsesCodex)) {
    Stop-Script '--install-codex-app 只能与 Codex 一起使用'
  }

  $NativeArch = if (-not [string]::IsNullOrWhiteSpace($env:PROCESSOR_ARCHITEW6432)) {
    $env:PROCESSOR_ARCHITEW6432
  } else {
    $env:PROCESSOR_ARCHITECTURE
  }
  $TargetArch = if ($NativeArch -eq 'ARM64') { 'arm64' } else { 'x64' }

  Write-Info "正在读取本站 Codex App 最新版本清单 ($TargetArch)"
  $Asset = Get-VerifiedSameSiteAsset `
    -ManifestUrl $script:CodexManifestUrl `
    -DownloadPrefix $script:CodexPackagePrefix `
    -Platform 'windows' `
    -Arch $TargetArch `
    -NamePattern '(?i)^OpenAI\.Codex_.*\.msix$'

  $LatestVersion = ''
  if ([string]$Asset.name -match '^OpenAI\.Codex_([0-9]+(?:\.[0-9]+){3})_') {
    $LatestVersion = $Matches[1]
  }
  $Installed = Get-AppxPackage -Name 'OpenAI.Codex' -ErrorAction SilentlyContinue |
    Select-Object -First 1
  if ($null -ne $Installed -and
      -not [string]::IsNullOrWhiteSpace($LatestVersion) -and
      [string]$Installed.Version -eq $LatestVersion) {
    $script:ExistingCodexApp = [string]$Installed.PackageFullName
    Write-Info "Codex App 已是最新版本 ($LatestVersion)"
    Enable-CodexAppAutoUpdate -InstalledPackage $Installed -TargetArch $TargetArch
    return
  }

  $TempDir = Join-Path ([IO.Path]::GetTempPath()) ("laoshirenai-codex-app-" + [guid]::NewGuid().ToString('N'))
  Ensure-Directory $TempDir
  $PackagePath = Join-Path $TempDir ([IO.Path]::GetFileName([string]$Asset.name))
  try {
    if ($TargetArch -eq 'x64') {
      $AppInstallerPath = Join-Path $TempDir 'Codex-Windows-x64.appinstaller'
      try {
        Write-Info '正在通过本站 AppInstaller 安装 Codex App 并登记自动更新'
        Invoke-WebRequest -UseBasicParsing -Uri $CodexAppInstallerUrl -OutFile $AppInstallerPath
        Add-AppxPackage -AppInstallerFile $AppInstallerPath
        $Installed = Get-AppxPackage -Name 'OpenAI.Codex' -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -ne $Installed) {
          $script:ExistingCodexApp = [string]$Installed.PackageFullName
          Enable-CodexAppAutoUpdate -InstalledPackage $Installed -TargetArch $TargetArch
          Write-Info 'Codex App 安装完成，并已登记启动时自动检查更新'
          return
        }
        throw 'AppInstaller 执行结束但未检测到 Codex App'
      } catch {
        Write-WarnMessage "AppInstaller 安装失败，将降级为直接安装本站缓存 MSIX: $_"
      }
    }

    Write-Info "正在从本站缓存下载最新 Codex App ($TargetArch)"
    Download-VerifiedAsset -Asset $Asset -OutputPath $PackagePath

    if ($null -ne $Installed) {
      Write-WarnMessage '检测到旧版 Codex App；更新时会安全关闭正在运行的 Codex，请先保存工作'
      Add-AppxPackage -Path $PackagePath -ForceApplicationShutdown
    } else {
      Add-AppxPackage -Path $PackagePath
    }
    $script:ExistingCodexApp = Get-InstalledCodexApp
    if ([string]::IsNullOrWhiteSpace($script:ExistingCodexApp)) {
      Stop-Script 'Codex App 安装结束但未能检测到应用'
    }
    $Installed = Get-AppxPackage -Name 'OpenAI.Codex' -ErrorAction SilentlyContinue | Select-Object -First 1
    Enable-CodexAppAutoUpdate -InstalledPackage $Installed -TargetArch $TargetArch
    $VersionSuffix = if ([string]::IsNullOrWhiteSpace($LatestVersion)) { '' } else { " ($LatestVersion)" }
    Write-Info "Codex App 安装完成$VersionSuffix"
  } finally {
    Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue
  }
}

# Windows x64 安装后登记本站 AppInstaller 更新源。新系统会在 Codex App
# 启动时检查更新；旧系统不支持该命令时保持现有安装，不阻断主流程。
function Enable-CodexAppAutoUpdate {
  param($InstalledPackage, [string]$TargetArch)

  if ($TargetArch -ne 'x64' -or $null -eq $InstalledPackage) {
    return
  }
  $Command = Get-Command 'Set-AppxPackageAutoUpdateSettings' -ErrorAction SilentlyContinue
  if ($null -eq $Command) {
    Write-WarnMessage '当前 Windows 暂不支持登记 Codex App 自动更新；以后可重新运行本命令检查最新版'
    return
  }
  try {
    Set-AppxPackageAutoUpdateSettings `
      -PackageFamilyName ([string]$InstalledPackage.PackageFamilyName) `
      -AppInstallerUri $CodexAppInstallerUrl `
      -CheckOnLaunch `
      -HoursBetweenUpdateChecks 24 `
      -Confirm:$false
    Write-Info 'Codex App 已开启启动时自动检查更新'
  } catch {
    Write-WarnMessage "Codex App 自动更新登记失败；以后可重新运行本命令检查最新版: $_"
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
      throw "拒绝覆盖损坏的 Claude settings.json: $($_.Exception.Message)"
    }
  } else {
    $Config = [pscustomobject]@{}
  }

  if ($null -eq $Config -or $Config -is [array]) {
    throw '拒绝覆盖非对象 Claude settings.json'
  }

  # 严格模式下用 Where-Object 检查属性是否存在，避免直接访问 .Name 报错
  $HasEnv = $Config.PSObject.Properties | Where-Object { $_.Name -eq 'env' }
  if (-not $HasEnv -or $null -eq $Config.env) {
    $Config | Add-Member -NotePropertyName env -NotePropertyValue ([pscustomobject]@{}) -Force
  }

  $Config | Add-Member -NotePropertyName model -NotePropertyValue $CatalogAnthropicDefaultModel -Force
  $Config.PSObject.Properties.Remove('effortLevel')
  $HasModelSettings = $Config.PSObject.Properties | Where-Object { $_.Name -eq 'modelSettings' }
  if (-not $HasModelSettings -or $null -eq $Config.modelSettings) {
    $Config | Add-Member -NotePropertyName modelSettings -NotePropertyValue ([pscustomobject]@{}) -Force
  } elseif ($Config.modelSettings -is [array]) {
    throw '拒绝覆盖非对象 Claude modelSettings'
  }
  $ReasoningCatalog = $CatalogModelReasoningJson | ConvertFrom-Json
  $LevelsProperty = $ReasoningCatalog.PSObject.Properties | Where-Object { $_.Name -eq $CatalogAnthropicDefaultModel }
  $Levels = if ($null -ne $LevelsProperty) { @($LevelsProperty.Value) } else { @() }
  $ModelProperty = $Config.modelSettings.PSObject.Properties | Where-Object { $_.Name -eq $CatalogAnthropicDefaultModel }
  $ModelSettings = if ($null -ne $ModelProperty -and $null -ne $ModelProperty.Value) { $ModelProperty.Value } else { [pscustomobject]@{} }
  if ($ModelSettings -is [array]) { throw '拒绝覆盖非对象 Claude modelSettings 条目' }
  if ($Levels -contains 'high') {
    $ModelSettings | Add-Member -NotePropertyName effortLevel -NotePropertyValue 'high' -Force
  } else {
    $ModelSettings.PSObject.Properties.Remove('effortLevel')
  }
  if ($ModelSettings.PSObject.Properties.Count -gt 0) {
    $Config.modelSettings | Add-Member -NotePropertyName $CatalogAnthropicDefaultModel -NotePropertyValue $ModelSettings -Force
  } else {
    $Config.modelSettings.PSObject.Properties.Remove($CatalogAnthropicDefaultModel)
  }
  $Config.env | Add-Member -NotePropertyName ANTHROPIC_BASE_URL -NotePropertyValue $BaseUrl -Force
  $Config.env | Add-Member -NotePropertyName ANTHROPIC_AUTH_TOKEN -NotePropertyValue $ClaudeApiKey -Force
  $Config.env | Add-Member -NotePropertyName ANTHROPIC_MODEL -NotePropertyValue $CatalogAnthropicDefaultModel -Force
  $Config.env | Add-Member -NotePropertyName ANTHROPIC_DEFAULT_OPUS_MODEL -NotePropertyValue $CatalogAnthropicDefaultModel -Force
  $Config.env | Add-Member -NotePropertyName ANTHROPIC_DEFAULT_SONNET_MODEL -NotePropertyValue $CatalogAnthropicDefaultModel -Force
  $Config.env | Add-Member -NotePropertyName ANTHROPIC_DEFAULT_HAIKU_MODEL -NotePropertyValue $CatalogAnthropicDefaultModel -Force
  $Config.env | Add-Member -NotePropertyName ANTHROPIC_DEFAULT_FABLE_MODEL -NotePropertyValue $CatalogAnthropicDefaultModel -Force
  $Config.env | Add-Member -NotePropertyName CLAUDE_CODE_ATTRIBUTION_HEADER -NotePropertyValue '0' -Force
  $Config.env | Add-Member -NotePropertyName CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY -NotePropertyValue '1' -Force

  $TemporaryPath = "$ClaudeSettingsPath.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
  try {
    $Json = $Config | ConvertTo-Json -Depth 20
    [System.IO.File]::WriteAllText($TemporaryPath, "$Json`n", [System.Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $TemporaryPath -Destination $ClaudeSettingsPath -Force
  } finally {
    Remove-Item -LiteralPath $TemporaryPath -Force -ErrorAction SilentlyContinue
  }
}

# 写入 Codex 的 auth.json，并仅覆盖 OPENAI_API_KEY 字段。
function Write-CodexAuthConfig {
  Backup-IfNeeded $CodexAuthPath
  Ensure-Directory (Split-Path -Parent $CodexAuthPath)

  if (Test-Path -LiteralPath $CodexAuthPath) {
    try {
      $Config = Get-Content -LiteralPath $CodexAuthPath -Raw | ConvertFrom-Json
    } catch {
      throw "拒绝覆盖损坏的 Codex auth.json: $($_.Exception.Message)"
    }
  } else {
    $Config = [pscustomobject]@{}
  }

  if ($null -eq $Config -or $Config -is [array]) {
    throw '拒绝覆盖非对象 Codex auth.json'
  }

  $Config | Add-Member -NotePropertyName OPENAI_API_KEY -NotePropertyValue $CodexApiKey -Force
  # 用无 BOM 的 UTF-8 写入，Windows PowerShell 5 默认带 BOM，Codex (serde_json) 不认 BOM 会报错
  $json = $Config | ConvertTo-Json -Depth 20
  $TemporaryPath = "$CodexAuthPath.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
  try {
    [System.IO.File]::WriteAllText($TemporaryPath, "$json`n", [System.Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $TemporaryPath -Destination $CodexAuthPath -Force
  } finally {
    Remove-Item -LiteralPath $TemporaryPath -Force -ErrorAction SilentlyContinue
  }
}

# 把全局模板裁剪成当前 API Key 所属分组真正开放的模型。未知但已授权的
# 分组专属别名（例如 Daybreak）继承 Sol 的客户端能力模板。
function Convert-CodexModelCatalog {
  param(
    [Parameter(Mandatory = $true)][string]$SourcePath,
    [Parameter(Mandatory = $true)][string[]]$AuthorizedModels,
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [string]$PreferredModel = ''
  )

  $Source = Get-Content -LiteralPath $SourcePath -Raw | ConvertFrom-Json
  $Models = @($Source.models)
  $RequiredFields = @('slug', 'base_instructions', 'supports_reasoning_summaries', 'context_window', 'visibility')
  $HasMissingFields = @($Models | Where-Object {
    $Model = $_
    @($RequiredFields | Where-Object { -not ($Model.PSObject.Properties.Name -contains $_) }).Count -gt 0
  }).Count -gt 0
  $Authorized = @($AuthorizedModels |
    ForEach-Object { ([string]$_).Trim() } |
    Where-Object { $_ -and $_ -ne 'codex-auto-review' } |
    Select-Object -Unique)

  if (
    $Models.Count -eq 0 -or
    $HasMissingFields -or
    $Authorized.Count -eq 0
  ) {
    throw 'Codex 模型目录或分组模型列表无效'
  }
  if (-not [string]::IsNullOrWhiteSpace($PreferredModel) -and $PreferredModel -notin $Authorized) {
    throw '票据选择的模型不在当前 Key 的模型列表中'
  }

  $ById = @{}
  foreach ($Model in $Models) {
    $ById[[string]$Model.slug] = $Model
  }
  # A key may be authorized for groups this client cannot serve, so the catalog
  # is the intersection. The two guards below still fail closed: the ticket's
  # model must survive the filter, and an empty intersection is an error.
  $Filtered = New-Object System.Collections.Generic.List[object]
  foreach ($Id in $Authorized) {
    if (-not $ById.ContainsKey($Id)) { continue }
    $Model = $ById[$Id] | ConvertTo-Json -Depth 100 | ConvertFrom-Json
    $Model | Add-Member -NotePropertyName priority -NotePropertyValue ($Filtered.Count + 1) -Force
    $Filtered.Add($Model)
  }
  if ($Filtered.Count -eq 0) { throw '当前 Key 没有 Codex 可用的 Responses 模型' }
  if (-not [string]::IsNullOrWhiteSpace($PreferredModel) -and @($Filtered | Where-Object { $_.slug -eq $PreferredModel }).Count -eq 0) {
    throw '票据选择的模型不在 Codex Responses 目录中'
  }

  $FilteredMissingFields = @($Filtered | Where-Object {
    $Model = $_
    @($RequiredFields | Where-Object { -not ($Model.PSObject.Properties.Name -contains $_) }).Count -gt 0
  }).Count -gt 0
  if ($FilteredMissingFields) {
    throw '筛选后的 Codex 模型目录不完整'
  }

  $Payload = [pscustomobject]@{ models = $Filtered.ToArray() }
  $Json = $Payload | ConvertTo-Json -Depth 100
  [System.IO.File]::WriteAllText($OutputPath, "$Json`n", [System.Text.UTF8Encoding]::new($false))
  $script:CatalogOpenAIDefaultModel = if (-not [string]::IsNullOrWhiteSpace($PreferredModel)) { $PreferredModel } elseif (@($Filtered | Where-Object { $_.slug -eq 'gpt-5.6-sol' }).Count -gt 0) { 'gpt-5.6-sol' } else { [string]$Filtered[0].slug }
  $SelectedCatalogModel = @($Filtered | Where-Object { $_.slug -eq $script:CatalogOpenAIDefaultModel })[0]
  $script:CatalogOpenAIContextWindow = [int64]$SelectedCatalogModel.context_window
  $script:CatalogOpenAIAutoCompactTokenLimit = [int64]$SelectedCatalogModel.auto_compact_token_limit
  $Efforts = @($SelectedCatalogModel.supported_reasoning_levels | ForEach-Object { [string]$_.effort })
  $script:CatalogOpenAIReasoningEffort = if ($Efforts -contains 'high') { 'high' } elseif ($Efforts.Count -gt 0) { $Efforts[0] } else { '' }
}

# 写入当前分组受支持的模型目录，阻止 Codex 回退到官方缓存后展示网关不支持的模型。
function Write-CodexModelCatalog {
  Backup-IfNeeded $CodexModelCatalogPath
  Ensure-Directory $CodexDir

  $DownloadPath = "$CodexModelCatalogPath.download"
  try {
    Download-FileWithFallback -OutputPath $DownloadPath -Urls @($DefaultCodexModelCatalogUrl)
    $ApiBaseUrl = Get-OpenAIV1BaseUrl -Value $script:BaseUrl
    $Response = Invoke-RestMethod -Uri "$ApiBaseUrl/models" -Headers @{
      Authorization = "Bearer $script:CodexApiKey"
    } -Method GET
    $AuthorizedModels = @($Response.data | ForEach-Object { [string]$_.id })
    Convert-CodexModelCatalog `
      -SourcePath $DownloadPath `
      -AuthorizedModels $AuthorizedModels `
      -OutputPath $DownloadPath `
      -PreferredModel $script:SelectedModel
    Move-Item -LiteralPath $DownloadPath -Destination $CodexModelCatalogPath -Force
  } catch {
    Remove-Item -LiteralPath $DownloadPath -Force -ErrorAction SilentlyContinue
    Stop-Script "Codex 分组模型目录生成失败: $($_.Exception.Message)"
  }
}

# 只更新 Codex 的本站 owned fields 和独立 Provider block，保留用户其他设置。
function Write-CodexTomlConfig {
  Backup-IfNeeded $CodexConfigPath
  Ensure-Directory (Split-Path -Parent $CodexConfigPath)
  $OwnedRoot = @('model_provider','model','review_model','model_reasoning_effort','model_catalog_json','disable_response_storage','network_access','preferred_auth_method','model_context_window','model_auto_compact_token_limit')
  $ManagedSection = 'model_providers.laoshirenai_responses'
  $Lines = if (Test-Path -LiteralPath $CodexConfigPath) { [System.Collections.Generic.List[string]]@(Get-Content -LiteralPath $CodexConfigPath) } else { [System.Collections.Generic.List[string]]@() }
  $Kept = [System.Collections.Generic.List[string]]::new()
  $Section = ''
  $Dropping = $false
  foreach ($Line in $Lines) {
    $Trimmed = $Line.Trim()
    if ($Trimmed.StartsWith('[')) {
      if ($Trimmed -notmatch '^\[([^\]]+)\](?:\s*#.*)?$') { throw "拒绝覆盖损坏的 Codex TOML 表头: $Trimmed" }
      $Section = $Matches[1]
      $Dropping = $Section -eq $ManagedSection
    }
    if ($Dropping) { continue }
    if ($Section -eq '' -and $Trimmed -match '^([A-Za-z0-9_.-]+)\s*=' -and $Matches[1] -in $OwnedRoot) { continue }
    if ($Trimmed -in @('# BEGIN LAOSHIRENAI CODEX PROVIDER', '# END LAOSHIRENAI CODEX PROVIDER')) { continue }
    $Kept.Add($Line)
  }
  while ($Kept.Count -gt 0 -and [string]::IsNullOrWhiteSpace($Kept[$Kept.Count - 1])) { $Kept.RemoveAt($Kept.Count - 1) }
  while ($Kept.Count -gt 0 -and [string]::IsNullOrWhiteSpace($Kept[0])) { $Kept.RemoveAt(0) }
  $Output = [System.Collections.Generic.List[string]]::new()
  $Output.Add("model_provider = $(ConvertTo-TomlString 'laoshirenai_responses')")
  $Output.Add("model = $(ConvertTo-TomlString $CatalogOpenAIDefaultModel)")
  $Output.Add("review_model = $(ConvertTo-TomlString $CatalogOpenAIDefaultModel)")
  if (-not [string]::IsNullOrWhiteSpace($CatalogOpenAIReasoningEffort)) { $Output.Add("model_reasoning_effort = $(ConvertTo-TomlString $CatalogOpenAIReasoningEffort)") }
  $Output.Add('model_catalog_json = "laoshirenai-model-catalog.json"')
  $Output.Add('disable_response_storage = true')
  $Output.Add('network_access = "enabled"')
  $Output.Add('preferred_auth_method = "apikey"')
  $Output.Add("model_context_window = $CatalogOpenAIContextWindow")
  $Output.Add("model_auto_compact_token_limit = $CatalogOpenAIAutoCompactTokenLimit")
  $Output.Add('')
  foreach ($Line in $Kept) { $Output.Add($Line) }
  if ($Kept.Count -gt 0) { $Output.Add('') }
  $Output.Add('# BEGIN LAOSHIRENAI CODEX PROVIDER')
  $Output.Add("[$ManagedSection]")
  $Output.Add('name = "老实人AI Responses"')
  $Output.Add("base_url = $(ConvertTo-TomlString $BaseUrl)")
  $Output.Add('wire_api = "responses"')
  $Output.Add('requires_openai_auth = true')
  $Output.Add('# END LAOSHIRENAI CODEX PROVIDER')
  $TemporaryPath = "$CodexConfigPath.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
  try {
    [System.IO.File]::WriteAllLines($TemporaryPath, $Output, [System.Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $TemporaryPath -Destination $CodexConfigPath -Force
  } finally {
    Remove-Item -LiteralPath $TemporaryPath -Force -ErrorAction SilentlyContinue
  }
}

function ConvertTo-TomlString {
  param([string]$Value)
  return ($Value | ConvertTo-Json -Compress)
}

function Write-GrokTomlConfig {
  Backup-IfNeeded $GrokConfigPath
  Ensure-Directory $GrokDir
  $Lines = if (Test-Path -LiteralPath $GrokConfigPath) {
    [System.Collections.Generic.List[string]]@(Get-Content -LiteralPath $GrokConfigPath)
  } else {
    [System.Collections.Generic.List[string]]@()
  }

  $Kept = [System.Collections.Generic.List[string]]@()
  $DroppingModel = $false
  foreach ($Line in $Lines) {
    $Trimmed = $Line.Trim()
    if ($Trimmed.StartsWith('[') -and $Trimmed -notmatch '^\[([^\]]+)\]$') { throw "拒绝覆盖损坏的 Grok TOML 表头: $Trimmed" }
    if ($Trimmed -match '^\[([^\]]+)\]$') {
      $DroppingModel = $Matches[1] -in $CatalogGrokManagedModelSections
    }
    if (-not $DroppingModel -and $Line.Trim() -ne '# Managed by laoshirenai one-click setup') {
      $Kept.Add($Line)
    }
  }
  $Lines = $Kept

  $ModelsHeader = -1
  for ($i = 0; $i -lt $Lines.Count; $i++) {
    if ($Lines[$i].Trim() -eq '[models]') { $ModelsHeader = $i; break }
  }
  if ($ModelsHeader -lt 0) {
    $Lines.Add('')
    $Lines.Add('[models]')
    $Lines.Add("default = $(ConvertTo-TomlString $CatalogGrokDefaultModel)")
  } else {
    $End = $Lines.Count
    for ($i = $ModelsHeader + 1; $i -lt $Lines.Count; $i++) {
      if ($Lines[$i] -match '^\s*\[') { $End = $i; break }
    }
    $Replaced = $false
    for ($i = $ModelsHeader + 1; $i -lt $End; $i++) {
      if ($Lines[$i] -match '^\s*default\s*=') {
        $Lines[$i] = "default = $(ConvertTo-TomlString $CatalogGrokDefaultModel)"
        $Replaced = $true
        break
      }
    }
    if (-not $Replaced) { $Lines.Insert($ModelsHeader + 1, "default = $(ConvertTo-TomlString $CatalogGrokDefaultModel)") }
  }

  $BaseV1 = Get-OpenAIV1BaseUrl -Value $script:BaseUrl
  $GrokProtocolVariable = Get-Variable -Scope Script -Name GrokApiBackend -ErrorAction SilentlyContinue
  $GrokProtocol = if ($null -ne $GrokProtocolVariable -and -not [string]::IsNullOrWhiteSpace([string]$GrokProtocolVariable.Value)) { [string]$GrokProtocolVariable.Value } else { 'responses' }
  while ($Lines.Count -gt 0 -and [string]::IsNullOrWhiteSpace($Lines[$Lines.Count - 1])) {
    $Lines.RemoveAt($Lines.Count - 1)
  }
  $Lines.Add('')
  $Lines.Add('# Managed by laoshirenai one-click setup')
  foreach ($ModelProfile in $CatalogGrokManagedModels) {
    $Lines.Add("[model.$(ConvertTo-TomlString $ModelProfile.Id)]")
    $Lines.Add("model = $(ConvertTo-TomlString $ModelProfile.Id)")
    $Lines.Add("base_url = $(ConvertTo-TomlString $BaseV1)")
    $Lines.Add("name = $(ConvertTo-TomlString $ModelProfile.DisplayName)")
    $Lines.Add("description = $(ConvertTo-TomlString $ModelProfile.DisplayName)")
    $Lines.Add("api_key = $(ConvertTo-TomlString $script:GrokApiKey)")
    $Lines.Add("api_backend = $(ConvertTo-TomlString $GrokProtocol)")
    if ($null -ne $ModelProfile.ContextWindow -and [int64]$ModelProfile.ContextWindow -gt 0) {
      $Lines.Add("context_window = $($ModelProfile.ContextWindow)")
    }
    $Lines.Add('')
  }

  $TemporaryPath = "$GrokConfigPath.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
  $ReplacementBackupPath = "$TemporaryPath.previous"
  try {
    [System.IO.File]::WriteAllLines($TemporaryPath, $Lines, [System.Text.UTF8Encoding]::new($false))
    if (Test-Path -LiteralPath $GrokConfigPath) {
      [System.IO.File]::Replace($TemporaryPath, $GrokConfigPath, $ReplacementBackupPath)
    } else {
      [System.IO.File]::Move($TemporaryPath, $GrokConfigPath)
    }
  } finally {
    Remove-Item -LiteralPath $TemporaryPath -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $ReplacementBackupPath -Force -ErrorAction SilentlyContinue
  }
}

function Set-GrokModelsFromKey {
  $ApiBaseUrl = Get-OpenAIV1BaseUrl -Value $script:BaseUrl
  try {
    $Response = Invoke-RestMethod -Uri "$ApiBaseUrl/models" -Headers @{
      Authorization = "Bearer $script:GrokApiKey"
    } -Method GET
  } catch {
    Stop-Script "Grok Build 分组模型读取失败: $_"
  }
  $Ids = @($Response.data |
    ForEach-Object { ([string]$_.id).Trim() } |
    Where-Object { $_ -and $_ -ne 'codex-auto-review' -and $_ -match '^[A-Za-z0-9][A-Za-z0-9._:-]*$' } |
    Select-Object -Unique)
  if ($Ids.Count -eq 0) { Stop-Script 'Grok Build 分组模型目录为空' }
  if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel) -and $script:SelectedModel -notin $Ids) {
    Stop-Script '票据选择的模型已不在当前 Key 的模型列表中'
  }
  $script:CatalogGrokDefaultModel = if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel)) {
    $script:SelectedModel
  } elseif ($Ids -contains 'gpt-5.6-sol') {
    'gpt-5.6-sol'
  } else {
    $Ids[0]
  }
  $script:CatalogGrokDefaultDisplayName = $script:CatalogGrokDefaultModel
  $script:CatalogGrokManagedModels = @($Ids | ForEach-Object {
    @{ Id = $_; DisplayName = $_; ContextWindow = 0 }
  })
  $script:CatalogGrokManagedModelSections = @($Ids | ForEach-Object { "model.$_"; "model.`"$_`"" })
}

# 合并写入 Gemini CLI 的 .env 与 settings.json：.env 只更新本站管理的四个键并保留其他行，
# settings.json 只更新鉴权方式、默认模型和本站管理的 thinkingConfig 覆盖项，其余字段原样保留。
function Write-GeminiConfig {
  Backup-IfNeeded $GeminiEnvPath
  Backup-IfNeeded $GeminiSettingsPath
  Ensure-Directory $GeminiDir
  # $SelectedReasoning is assigned by the top-level selection flow; under the
  # AST-extracted Windows QA harness (or partial sourcing) it may not exist,
  # so read it defensively instead of relying on dynamic scope.
  $EffectiveReasoning = if (Get-Variable -Name 'SelectedReasoning' -ErrorAction SilentlyContinue) { [string]$SelectedReasoning } elseif ($env:LAOSHIRENAI_REASONING_EFFORT) { $env:LAOSHIRENAI_REASONING_EFFORT.ToLowerInvariant() } else { '' }
  $ThinkingLevel = if ($EffectiveReasoning -in @('low','medium','high')) { $EffectiveReasoning.ToUpperInvariant() } else { 'HIGH' }

  $ManagedEnv = [ordered]@{
    GEMINI_API_KEY = $script:GeminiApiKey
    GOOGLE_GEMINI_BASE_URL = $script:BaseUrl
    GOOGLE_GENAI_USE_VERTEXAI = 'false'
    GEMINI_MODEL = $CatalogGeminiDefaultModel
  }
  $EnvLines = [System.Collections.Generic.List[string]]::new()
  if (Test-Path -LiteralPath $GeminiEnvPath) {
    $EnvLines.AddRange([string[]]@(Get-Content -LiteralPath $GeminiEnvPath))
  }
  $SeenKeys = @{}
  for ($i = 0; $i -lt $EnvLines.Count; $i++) {
    if ($EnvLines[$i] -match '^([A-Za-z_][A-Za-z0-9_]*)=' -and $ManagedEnv.Contains($Matches[1])) {
      $Key = $Matches[1]
      $EnvLines[$i] = "$Key=$($ManagedEnv[$Key])"
      $SeenKeys[$Key] = $true
    }
  }
  foreach ($Key in $ManagedEnv.Keys) {
    if (-not $SeenKeys.ContainsKey($Key)) {
      $EnvLines.Add("$Key=$($ManagedEnv[$Key])")
    }
  }
  $EnvTemporaryPath = "$GeminiEnvPath.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
  try {
    [System.IO.File]::WriteAllLines($EnvTemporaryPath, [string[]]$EnvLines, [System.Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $EnvTemporaryPath -Destination $GeminiEnvPath -Force
  } finally {
    Remove-Item -LiteralPath $EnvTemporaryPath -Force -ErrorAction SilentlyContinue
  }

  if (Test-Path -LiteralPath $GeminiSettingsPath) {
    try {
      $Config = Get-Content -LiteralPath $GeminiSettingsPath -Raw | ConvertFrom-Json
    } catch {
      throw "拒绝覆盖损坏的 Gemini settings.json: $($_.Exception.Message)"
    }
  } else {
    $Config = [pscustomobject]@{}
  }
  if ($null -eq $Config -or $Config -is [array]) {
    throw '拒绝覆盖非对象 Gemini settings.json'
  }

  # 严格模式下用 Where-Object 检查属性是否存在，避免直接访问 .Name 报错
  $HasSecurity = $Config.PSObject.Properties | Where-Object { $_.Name -eq 'security' }
  if (-not $HasSecurity -or $null -eq $Config.security) {
    $Config | Add-Member -NotePropertyName security -NotePropertyValue ([pscustomobject]@{}) -Force
  }
  $HasAuth = $Config.security.PSObject.Properties | Where-Object { $_.Name -eq 'auth' }
  if (-not $HasAuth -or $null -eq $Config.security.auth) {
    $Config.security | Add-Member -NotePropertyName auth -NotePropertyValue ([pscustomobject]@{}) -Force
  }
  $Config.security.auth | Add-Member -NotePropertyName selectedType -NotePropertyValue 'gemini-api-key' -Force

  $HasModel = $Config.PSObject.Properties | Where-Object { $_.Name -eq 'model' }
  if (-not $HasModel -or $null -eq $Config.model) {
    $Config | Add-Member -NotePropertyName model -NotePropertyValue ([pscustomobject]@{}) -Force
  }
  $Config.model | Add-Member -NotePropertyName name -NotePropertyValue $CatalogGeminiDefaultModel -Force

  $HasModelConfigs = $Config.PSObject.Properties | Where-Object { $_.Name -eq 'modelConfigs' }
  if (-not $HasModelConfigs -or $null -eq $Config.modelConfigs) {
    $Config | Add-Member -NotePropertyName modelConfigs -NotePropertyValue ([pscustomobject]@{}) -Force
  }
  $HasOverrides = $Config.modelConfigs.PSObject.Properties | Where-Object { $_.Name -eq 'overrides' }
  $ExistingOverrides = @()
  if ($HasOverrides -and $null -ne $Config.modelConfigs.overrides) {
    $ExistingOverrides = @($Config.modelConfigs.overrides)
  }
  $KeptOverrides = @($ExistingOverrides | Where-Object {
    $Entry = $_
    $MatchProperty = $Entry.PSObject.Properties | Where-Object { $_.Name -eq 'match' }
    if ($null -eq $MatchProperty -or $null -eq $Entry.match) { return $true }
    $ModelProperty = $Entry.match.PSObject.Properties | Where-Object { $_.Name -eq 'model' }
    if ($null -eq $ModelProperty) { return $true }
    return ([string]$Entry.match.model) -notin $CatalogGeminiManagedModels
  })
  $NewOverrides = [System.Collections.Generic.List[object]]@($KeptOverrides)
  foreach ($ModelId in $CatalogGeminiManagedModels) {
    $NewOverrides.Add([pscustomobject]@{
      match = [pscustomobject]@{ model = $ModelId }
      generateContentConfig = [pscustomobject]@{
        thinkingConfig = [pscustomobject]@{ thinkingLevel = $ThinkingLevel }
      }
    })
  }
  $Config.modelConfigs | Add-Member -NotePropertyName overrides -NotePropertyValue $NewOverrides -Force

  $json = $Config | ConvertTo-Json -Depth 20
  $SettingsTemporaryPath = "$GeminiSettingsPath.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
  try {
    [System.IO.File]::WriteAllText($SettingsTemporaryPath, "$json`n", [System.Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $SettingsTemporaryPath -Destination $GeminiSettingsPath -Force
  } finally {
    Remove-Item -LiteralPath $SettingsTemporaryPath -Force -ErrorAction SilentlyContinue
  }
}

function Get-CcSwitchLaunchTarget {
  $StartApp = Get-StartApps -ErrorAction SilentlyContinue |
    Where-Object { $_.Name -eq 'CC Switch' } |
    Select-Object -First 1
  if ($null -ne $StartApp) {
    return [pscustomobject]@{ Kind = 'AppId'; Value = "shell:AppsFolder\$($StartApp.AppID)" }
  }

  $Candidates = @()
  if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
    $Candidates += Join-Path $env:LOCALAPPDATA 'Programs\CC Switch\cc-switch.exe'
  }
  if (-not [string]::IsNullOrWhiteSpace($env:ProgramFiles)) {
    $Candidates += Join-Path $env:ProgramFiles 'CC Switch\cc-switch.exe'
  }
  foreach ($Candidate in $Candidates) {
    if (Test-Path -LiteralPath $Candidate -PathType Leaf) {
      return [pscustomobject]@{ Kind = 'Executable'; Value = $Candidate }
    }
  }
  return $null
}

function Start-CcSwitchTarget {
  param([object]$Target)
  if ($Target.Kind -eq 'AppId') {
    Start-Process ([string]$Target.Value)
  } else {
    Start-Process -FilePath ([string]$Target.Value)
  }
}

function Stop-CcSwitchForImport {
  $Processes = @(Get-Process -Name 'cc-switch' -ErrorAction SilentlyContinue)
  if ($Processes.Count -eq 0) { return $true }

  foreach ($Process in $Processes) {
    try { $null = $Process.CloseMainWindow() } catch {}
  }
  for ($Attempt = 0; $Attempt -lt 20; $Attempt++) {
    if (@(Get-Process -Name 'cc-switch' -ErrorAction SilentlyContinue).Count -eq 0) { return $true }
    Start-Sleep -Milliseconds 250
  }

  # A tray-only Tauri process has no window to close. Stop it, then let SQLite
  # recover/checkpoint before the importer creates its transactional backup.
  Get-Process -Name 'cc-switch' -ErrorAction SilentlyContinue |
    Stop-Process -Force -ErrorAction SilentlyContinue
  for ($Attempt = 0; $Attempt -lt 20; $Attempt++) {
    if (@(Get-Process -Name 'cc-switch' -ErrorAction SilentlyContinue).Count -eq 0) { return $true }
    Start-Sleep -Milliseconds 250
  }
  return $false
}

function Invoke-GrokCcSwitchImporter {
  param([string]$ImporterPath)

  $ManagedModels = @($CatalogGrokManagedModels | ForEach-Object {
    @{
      id = [string]$_.Id
      display_name = [string]$_.DisplayName
      context_window = [int64]$_.ContextWindow
    }
  }) | ConvertTo-Json -Compress

  $Names = @(
    'GROK_PROVIDER_DEFAULT_MODEL',
    'GROK_PROVIDER_BASE_URL',
    'GROK_PROVIDER_API_KEY',
    'GROK_PROVIDER_MODELS_JSON'
  )
  $Previous = @{}
  foreach ($Name in $Names) {
    $Previous[$Name] = [Environment]::GetEnvironmentVariable($Name, 'Process')
  }
  try {
    [Environment]::SetEnvironmentVariable('GROK_PROVIDER_DEFAULT_MODEL', $CatalogGrokDefaultModel, 'Process')
    [Environment]::SetEnvironmentVariable('GROK_PROVIDER_BASE_URL', (Get-OpenAIV1BaseUrl -Value $script:BaseUrl), 'Process')
    [Environment]::SetEnvironmentVariable('GROK_PROVIDER_API_KEY', $script:GrokApiKey, 'Process')
    [Environment]::SetEnvironmentVariable('GROK_PROVIDER_MODELS_JSON', $ManagedModels, 'Process')
    & $script:NodeExe --no-warnings $ImporterPath | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Provider importer exited with code $LASTEXITCODE" }
  } finally {
    foreach ($Name in $Names) {
      [Environment]::SetEnvironmentVariable($Name, $Previous[$Name], 'Process')
    }
  }
}

function Open-CcSwitchIfRequested {
  if (-not $script:GrokCcSwitchCompat -or -not (Test-UsesGrok)) { return }

  $Target = Get-CcSwitchLaunchTarget
  if ($null -eq $Target) {
    Write-WarnMessage 'Grok Build 已配置好；未找到官方 CC Switch，可稍后安装后重试'
    return
  }

  $TempDir = Join-Path ([IO.Path]::GetTempPath()) ("laoshirenai-grok-cc-switch-" + [guid]::NewGuid().ToString('N'))
  Ensure-Directory $TempDir
  try {
    $ImporterPath = Join-Path $TempDir 'import-grok-cc-switch-provider.cjs'
    if (-not [string]::IsNullOrWhiteSpace($env:LAOSHIRENAI_GROK_CC_SWITCH_IMPORTER_PATH)) {
      Copy-Item -LiteralPath $env:LAOSHIRENAI_GROK_CC_SWITCH_IMPORTER_PATH -Destination $ImporterPath -Force
    } else {
      Invoke-WebRequest -UseBasicParsing -Uri $script:GrokCcSwitchImporterUrl -OutFile $ImporterPath
    }

    $DbPath = ((& $script:NodeExe --no-warnings $ImporterPath --print-db-path | Select-Object -Last 1) -as [string]).Trim()
    if (-not (Test-Path -LiteralPath $DbPath -PathType Leaf)) {
      Start-CcSwitchTarget -Target $Target
      for ($Attempt = 0; $Attempt -lt 40; $Attempt++) {
        if (Test-Path -LiteralPath $DbPath -PathType Leaf) { break }
        Start-Sleep -Milliseconds 250
      }
    }

    if (-not (Stop-CcSwitchForImport)) {
      Stop-Script 'CC Switch 正在运行且无法安全刷新，请退出后重试这一行命令'
    }

    Invoke-GrokCcSwitchImporter -ImporterPath $ImporterPath
    Start-CcSwitchTarget -Target $Target
    Write-Info '已将 Grok 分组导入官方 CC Switch，并保留其他 Provider'
  } catch {
    Stop-Script "CC Switch Provider 导入失败，本机 Grok Build 配置和原有 Provider 均已保留: $_"
  } finally {
    Remove-Item -LiteralPath $TempDir -Recurse -Force -ErrorAction SilentlyContinue
  }
}

function Test-UsesCodex {
  return $script:Tools -in @('all', 'codex')
}

function Test-UsesClaude {
  return $script:Tools -in @('all', 'claude')
}

function Test-UsesGrok {
  return $script:Tools -eq 'grok'
}

function Test-UsesGemini {
  return $script:Tools -eq 'gemini'
}

function Test-UsesKimi {
  return $script:Tools -eq 'kimi'
}

function Test-UsesOpenCode {
  return $script:Tools -eq 'opencode'
}

function Test-UsesZCode {
  return $script:Tools -eq 'zcode'
}

function Test-UsesWorkBuddy {
  return $script:Tools -eq 'workbuddy'
}

function Get-OpenAIV1BaseUrl {
  param([string]$Value)

  $NormalizedUrl = $Value.TrimEnd([char[]]'/')
  if ($NormalizedUrl.EndsWith('/v1', [StringComparison]::OrdinalIgnoreCase)) {
    return $NormalizedUrl
  }

  return "$NormalizedUrl/v1"
}

function Test-ApiKeyReadiness {
  param(
    [string]$Label,
    [string]$ApiKey
  )
  $ApiBaseUrl = Get-OpenAIV1BaseUrl -Value $script:BaseUrl
  Write-Info "正在检查 $Label 专用 Key 和账户余额"

  try {
    $Usage = Invoke-RestMethod -Uri "$ApiBaseUrl/usage" -Headers @{
      Authorization = "Bearer $ApiKey"
    } -Method GET
  } catch {
    $Status = '请求失败'
    $ResponseProperty = $_.Exception.PSObject.Properties['Response']
    if ($null -ne $ResponseProperty -and $null -ne $ResponseProperty.Value -and $ResponseProperty.Value.StatusCode) {
      $Status = "HTTP $([int]$ResponseProperty.Value.StatusCode)"
    }
    Stop-Script "$Label 专用 Key 验证失败: $ApiBaseUrl/usage 返回 $Status"
  }

  $RemainingProperty = $Usage.PSObject.Properties['remaining']
  if ($null -ne $RemainingProperty -and
      $null -ne $RemainingProperty.Value -and
      [double]$RemainingProperty.Value -le 0) {
    $script:BalanceReady = $false
    Write-WarnMessage "$Label 已安装并配置完成，但当前余额/套餐额度不足"
  }
  if ($Usage.mode -eq 'quota_limited' -and
      -not [string]::IsNullOrWhiteSpace([string]$Usage.status) -and
      $Usage.status -notin @('active', 'quota_exhausted')) {
    Stop-Script "$Label 专用 Key 当前不可用，请在网站检查 Key 状态"
  }

  try {
    $Response = Invoke-WebRequest -UseBasicParsing -Uri "$ApiBaseUrl/models" -Headers @{
      Authorization = "Bearer $ApiKey"
    } -Method GET
    if ([int]$Response.StatusCode -ne 200) {
      Stop-Script "$Label 连通性测试失败: $ApiBaseUrl/models 返回 HTTP $($Response.StatusCode)"
    }
  } catch {
    $Status = '请求失败'
    $ResponseProperty = $_.Exception.PSObject.Properties['Response']
    if ($null -ne $ResponseProperty -and $null -ne $ResponseProperty.Value -and $ResponseProperty.Value.StatusCode) {
      $Status = "HTTP $([int]$ResponseProperty.Value.StatusCode)"
    }
    Stop-Script "$Label 连通性测试失败: $ApiBaseUrl/models 返回 $Status"
  }
  Write-Info "$Label 专用 Key、余额和连通性检查通过"
}

# 对票据中精确选择的模型和协议执行一次最小真实请求；/models 仅用于发现，
# 只有这里拿到完整终态与非空文本才算配置闭环。
function Test-SelectedModelRequest {
  if ([string]::IsNullOrWhiteSpace($script:SelectedModel) -or [string]::IsNullOrWhiteSpace($script:SelectedProtocol)) { return }
  $ApiKey = switch ($script:Tools) {
    'claude' { $script:ClaudeApiKey }
    'codex' { $script:CodexApiKey }
    'grok' { $script:GrokApiKey }
    'gemini' { $script:GeminiApiKey }
    'kimi' { $script:KimiApiKey }
    'opencode' { $script:OpenCodeApiKey }
    'zcode' { $script:ZCodeApiKey }
    'workbuddy' { $script:WorkBuddyApiKey }
    default { return }
  }
  $ApiBaseUrl = Get-OpenAIV1BaseUrl -Value $script:BaseUrl
  $RootUrl = $ApiBaseUrl -replace '/v1$', ''
  $Headers = @{}
  $Body = $null
  switch ($script:SelectedProtocol) {
    'responses' {
      $Uri = "$ApiBaseUrl/responses"
      $Headers.Authorization = "Bearer $ApiKey"
      $Body = @{ model = $script:SelectedModel; input = '只回复 CONFIG_OK'; max_output_tokens = 32 }
    }
    'chat_completions' {
      $Uri = "$ApiBaseUrl/chat/completions"
      $Headers.Authorization = "Bearer $ApiKey"
      $Body = @{ model = $script:SelectedModel; messages = @(@{ role = 'user'; content = '只回复 CONFIG_OK' }); max_tokens = 32 }
    }
    'messages' {
      $Uri = "$ApiBaseUrl/messages"
      $Headers['x-api-key'] = $ApiKey
      $Headers['anthropic-version'] = '2023-06-01'
      $Body = @{ model = $script:SelectedModel; messages = @(@{ role = 'user'; content = '只回复 CONFIG_OK' }); max_tokens = 32 }
    }
    'generate_content' {
      $Uri = "$RootUrl/v1beta/models/$($script:SelectedModel):generateContent"
      $Headers['x-goog-api-key'] = $ApiKey
      $Body = @{ contents = @(@{ role = 'user'; parts = @(@{ text = '只回复 CONFIG_OK' }) }); generationConfig = @{ maxOutputTokens = 32 } }
    }
    default { Stop-Script "票据返回了不支持的协议: $($script:SelectedProtocol)" }
  }
  try {
    $Response = Invoke-RestMethod -Uri $Uri -Method POST -Headers $Headers -ContentType 'application/json' -Body ($Body | ConvertTo-Json -Depth 12 -Compress)
  } catch {
    Stop-Script "$($script:SelectedModel) 最小验证失败: $($script:SelectedProtocol) 请求失败"
  }
  $Complete = switch ($script:SelectedProtocol) {
    'responses' { $Response.status -eq 'completed' -or -not [string]::IsNullOrWhiteSpace([string]$Response.output_text) -or @($Response.output | Where-Object { $_.type -eq 'message' -and @($_.content | Where-Object { $_.type -eq 'output_text' -and -not [string]::IsNullOrWhiteSpace([string]$_.text) }).Count -gt 0 }).Count -gt 0 }
    'chat_completions' { -not [string]::IsNullOrWhiteSpace([string]$Response.choices[0].message.content) -and -not [string]::IsNullOrWhiteSpace([string]$Response.choices[0].finish_reason) }
    'messages' { @($Response.content | Where-Object { $_.type -eq 'text' -and -not [string]::IsNullOrWhiteSpace([string]$_.text) }).Count -gt 0 -and -not [string]::IsNullOrWhiteSpace([string]$Response.stop_reason) }
    'generate_content' { @($Response.candidates[0].content.parts | Where-Object { -not [string]::IsNullOrWhiteSpace([string]$_.text) }).Count -gt 0 -and -not [string]::IsNullOrWhiteSpace([string]$Response.candidates[0].finishReason) }
  }
  if (-not $Complete) { Stop-Script '最小验证响应缺少完整终态或文本' }
  Write-Info "$($script:SelectedModel) · $($script:SelectedProtocol) 最小真实请求通过"
}

function Test-ClaudeApiKey {
  if (Test-UsesClaude) {
    Test-ApiKeyReadiness -Label 'Claude Code' -ApiKey $script:ClaudeApiKey
  }
}

function Test-CodexApiKey {
  if (Test-UsesCodex) {
    Test-ApiKeyReadiness -Label 'Codex' -ApiKey $script:CodexApiKey
  }
}

function Test-GrokApiKey {
  if (Test-UsesGrok) {
    Test-ApiKeyReadiness -Label 'Grok Build' -ApiKey $script:GrokApiKey
  }
}

function Test-GeminiApiKey {
  if (Test-UsesGemini) {
    Test-ApiKeyReadiness -Label 'Gemini CLI' -ApiKey $script:GeminiApiKey
  }
}

function Test-KimiApiKey {
  if (Test-UsesKimi) { Test-ApiKeyReadiness -Label 'Kimi Code' -ApiKey $script:KimiApiKey }
}

function Test-OpenCodeApiKey {
  if (Test-UsesOpenCode) { Test-ApiKeyReadiness -Label 'OpenCode' -ApiKey $script:OpenCodeApiKey }
}

function Test-ZCodeApiKey {
  if (Test-UsesZCode) { Test-ApiKeyReadiness -Label 'ZCode' -ApiKey $script:ZCodeApiKey }
}

function Test-WorkBuddyApiKey {
  if (Test-UsesWorkBuddy) { Test-ApiKeyReadiness -Label 'WorkBuddy' -ApiKey $script:WorkBuddyApiKey }
}

function Write-KimiConfig {
  $ApiBaseUrl = Get-OpenAIV1BaseUrl -Value $script:BaseUrl
  try {
    $Response = Invoke-RestMethod -Uri "$ApiBaseUrl/models" -Headers @{ Authorization = "Bearer $script:KimiApiKey" } -Method GET
  } catch {
    Stop-Script "Kimi Code 分组模型读取失败: $_"
  }
  $Ids = @($Response.data | ForEach-Object { ([string]$_.id).Trim() } |
    Where-Object { $_ -and $_ -ne 'codex-auto-review' -and $_ -match '^[A-Za-z0-9][A-Za-z0-9._:-]*$' } | Select-Object -Unique)
  if ($Ids.Count -eq 0) { Stop-Script 'Kimi Code 分组模型目录为空' }
  if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel) -and $script:SelectedModel -notin $Ids) {
    Stop-Script '票据选择的模型已不在当前 Key 的模型列表中'
  }
  $Selected = if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel)) { $script:SelectedModel } elseif ($Ids -contains 'kimi-k3') { 'kimi-k3' } else { $Ids[0] }
  Backup-IfNeeded $KimiConfigPath
  Ensure-Directory $KimiDir
  $Text = if (Test-Path -LiteralPath $KimiConfigPath) { [IO.File]::ReadAllText($KimiConfigPath) } else { '' }
  if ($Text.Contains([char]0)) { Stop-Script '拒绝覆盖损坏的 Kimi Code config.toml' }
  $Kept = New-Object System.Collections.Generic.List[string]
  $Section = ''
  $Managed = $false
  foreach ($Line in ($Text -split "`r?`n")) {
    $Trimmed = $Line.Trim()
    if ($Trimmed -eq '# BEGIN LSRAI KIMI CODE') { $Managed = $true; continue }
    if ($Trimmed -eq '# END LSRAI KIMI CODE') { $Managed = $false; continue }
    if ($Managed) { continue }
    if ($Trimmed.StartsWith('[')) {
      if ($Trimmed -notmatch '^\[([^\]]+)\](?:\s*#.*)?$') { Stop-Script "拒绝覆盖无法解析的 Kimi Code TOML 表头: $Trimmed" }
      $Section = $Matches[1]
    }
    if ($Section -eq '' -and $Trimmed -match '^default_model\s*=') { continue }
    $Kept.Add($Line)
  }
  while ($Kept.Count -gt 0 -and [string]::IsNullOrWhiteSpace($Kept[0])) { $Kept.RemoveAt(0) }
  while ($Kept.Count -gt 0 -and [string]::IsNullOrWhiteSpace($Kept[$Kept.Count - 1])) { $Kept.RemoveAt($Kept.Count - 1) }
  $Lines = New-Object System.Collections.Generic.List[string]
  $Lines.Add("default_model = `"lsrai/$Selected`"")
  $Lines.Add('')
  $Lines.Add('# BEGIN LSRAI KIMI CODE')
  $Lines.Add('[providers.lsrai]')
  $Lines.Add('type = "openai"')
  $Lines.Add("base_url = $(ConvertTo-TomlString $ApiBaseUrl)")
  $Lines.Add("api_key = $(ConvertTo-TomlString $script:KimiApiKey)")
  $Lines.Add('')
  foreach ($Id in $Ids) {
    $Lines.Add("[models.`"lsrai/$Id`"]")
    $Lines.Add('provider = "lsrai"')
    $Lines.Add("model = $(ConvertTo-TomlString $Id)")
    $Lines.Add('max_context_size = 262144')
    $Lines.Add('')
  }
  $Lines.Add('# END LSRAI KIMI CODE')
  if ($Kept.Count -gt 0) { $Lines.Add(''); foreach ($Line in $Kept) { $Lines.Add($Line) } }
  $TemporaryPath = "$KimiConfigPath.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
  try {
    [IO.File]::WriteAllText($TemporaryPath, (($Lines -join "`n") + "`n"), [Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $TemporaryPath -Destination $KimiConfigPath -Force
  } finally { Remove-Item -LiteralPath $TemporaryPath -Force -ErrorAction SilentlyContinue }
}

function Write-OpenCodeConfig {
  $ApiBaseUrl = Get-OpenAIV1BaseUrl -Value $script:BaseUrl
  try {
    $Response = Invoke-RestMethod -Uri "$ApiBaseUrl/models" -Headers @{ Authorization = "Bearer $script:OpenCodeApiKey" } -Method GET
  } catch {
    Stop-Script "OpenCode 分组模型读取失败: $_"
  }
  $Ids = @($Response.data | ForEach-Object { ([string]$_.id).Trim() } |
    Where-Object { $_ -and $_ -ne 'codex-auto-review' -and $_ -match '^[A-Za-z0-9][A-Za-z0-9._:-]*$' } | Select-Object -Unique)
  if ($Ids.Count -eq 0) { Stop-Script 'OpenCode 分组模型目录为空' }
  if ([string]::IsNullOrWhiteSpace($script:SelectedProtocol)) { Stop-Script 'OpenCode 缺少协议选择' }
  $Packages = @{
    responses = '@ai-sdk/openai'
    chat_completions = '@ai-sdk/openai-compatible'
    messages = '@ai-sdk/anthropic'
    generate_content = '@ai-sdk/google'
  }
  if (-not $Packages.ContainsKey($script:SelectedProtocol)) { Stop-Script "OpenCode 不支持协议: $($script:SelectedProtocol)" }
  if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel) -and $script:SelectedModel -notin $Ids) {
    Stop-Script '票据选择的模型已不在当前 Key 的模型列表中'
  }
  $Selected = if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel)) { $script:SelectedModel } else { $Ids[0] }
  Backup-IfNeeded $OpenCodeConfigPath
  Ensure-Directory $OpenCodeDir
  $Config = [pscustomobject]@{}
  if (Test-Path -LiteralPath $OpenCodeConfigPath) {
    $RawConfig = Get-Content -LiteralPath $OpenCodeConfigPath -Raw
    if (-not $RawConfig.TrimStart().StartsWith('{')) { Stop-Script 'OpenCode 配置根节点必须是对象' }
    try { $Config = $RawConfig | ConvertFrom-Json } catch { Stop-Script "拒绝覆盖无法解析的 OpenCode 配置: $_" }
    if ($null -eq $Config -or $Config -is [System.Array]) { Stop-Script 'OpenCode 配置根节点必须是对象' }
  }
  $ProviderProperty = $Config.PSObject.Properties['provider']
  if ($null -eq $ProviderProperty -or $null -eq $ProviderProperty.Value) {
    $Config | Add-Member -NotePropertyName provider -NotePropertyValue ([pscustomobject]@{}) -Force
  } elseif ($Config.provider -is [System.Array] -or $Config.provider -is [string] -or $Config.provider -is [ValueType]) {
    Stop-Script 'OpenCode provider 必须是对象'
  }
  $Models = [ordered]@{}
  foreach ($Id in $Ids) { $Models[$Id] = [ordered]@{ name = $Id } }
  $RootUrl = $ApiBaseUrl -replace '/v1/?$', ''
  $ProviderBaseUrl = if ($script:SelectedProtocol -eq 'generate_content') { "$RootUrl/v1beta" } else { "$RootUrl/v1" }
  $Provider = [ordered]@{
    npm = $Packages[$script:SelectedProtocol]
    name = 'lsrai'
    options = [ordered]@{ baseURL = $ProviderBaseUrl; apiKey = $script:OpenCodeApiKey }
    models = $Models
  }
  if ($null -eq $Config.PSObject.Properties['$schema']) { $Config | Add-Member -NotePropertyName '$schema' -NotePropertyValue 'https://opencode.ai/config.json' -Force }
  $Config.provider | Add-Member -NotePropertyName lsrai -NotePropertyValue $Provider -Force
  $Config | Add-Member -NotePropertyName model -NotePropertyValue "lsrai/$Selected" -Force
  $Json = $Config | ConvertTo-Json -Depth 100
  $TemporaryPath = "$OpenCodeConfigPath.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
  try {
    [IO.File]::WriteAllText($TemporaryPath, ($Json + "`n"), [Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $TemporaryPath -Destination $OpenCodeConfigPath -Force
  } finally { Remove-Item -LiteralPath $TemporaryPath -Force -ErrorAction SilentlyContinue }
}

function Write-ZCodeConfig {
  $ApiBaseUrl = Get-OpenAIV1BaseUrl -Value $script:BaseUrl
  try {
    $Response = Invoke-RestMethod -Uri "$ApiBaseUrl/models" -Headers @{ Authorization = "Bearer $script:ZCodeApiKey" } -Method GET
  } catch {
    Stop-Script "ZCode 分组模型读取失败: $_"
  }
  $Ids = @($Response.data | ForEach-Object { ([string]$_.id).Trim() } |
    Where-Object { $_ -and $_ -ne 'codex-auto-review' -and $_ -match '^[A-Za-z0-9][A-Za-z0-9._:-]*$' } | Select-Object -Unique)
  if ($Ids.Count -eq 0) { Stop-Script 'ZCode 分组模型目录为空' }
  if ($script:SelectedProtocol -ne 'responses') { Stop-Script 'ZCode 一键配置只接受已验证的 Responses 协议' }
  if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel) -and $script:SelectedModel -notin $Ids) { Stop-Script '票据选择的模型已不在当前 Key 的模型列表中' }
  $Selected = if (-not [string]::IsNullOrWhiteSpace($script:SelectedModel)) { $script:SelectedModel } else { $Ids[0] }
  Backup-IfNeeded $ZCodeAppConfigPath
  Backup-IfNeeded $ZCodeCliConfigPath
  Ensure-Directory (Split-Path -Parent $ZCodeAppConfigPath)
  Ensure-Directory (Split-Path -Parent $ZCodeCliConfigPath)
  $App = [pscustomobject]@{}
  if (Test-Path -LiteralPath $ZCodeAppConfigPath) {
    $Raw = Get-Content -LiteralPath $ZCodeAppConfigPath -Raw
    if (-not $Raw.TrimStart().StartsWith('{')) { Stop-Script 'ZCode App 配置根节点必须是对象' }
    try { $App = $Raw | ConvertFrom-Json } catch { Stop-Script "拒绝覆盖无法解析的 ZCode App 配置: $_" }
  }
  $Cli = [pscustomobject]@{}
  if (Test-Path -LiteralPath $ZCodeCliConfigPath) {
    $Raw = Get-Content -LiteralPath $ZCodeCliConfigPath -Raw
    if (-not $Raw.TrimStart().StartsWith('{')) { Stop-Script 'ZCode CLI 配置根节点必须是对象' }
    try { $Cli = $Raw | ConvertFrom-Json } catch { Stop-Script "拒绝覆盖无法解析的 ZCode CLI 配置: $_" }
  }
  foreach ($Config in @($App, $Cli)) {
    $ProviderProperty = $Config.PSObject.Properties['provider']
    if ($null -eq $ProviderProperty -or $null -eq $ProviderProperty.Value) {
      $Config | Add-Member -NotePropertyName provider -NotePropertyValue ([pscustomobject]@{}) -Force
    } elseif ($ProviderProperty.Value -is [System.Array] -or $ProviderProperty.Value -is [string] -or $ProviderProperty.Value -is [ValueType]) {
      Stop-Script 'ZCode provider 必须是对象'
    }
  }
  $Models = [pscustomobject]@{}
  foreach ($Id in $Ids) { $Models | Add-Member -NotePropertyName $Id -NotePropertyValue ([ordered]@{ name = $Id }) -Force }
  $AppProvider = [ordered]@{
    name = 'lsrai'; kind = 'openai'; source = 'custom'; enabled = $true
    options = [ordered]@{ apiKey = $script:ZCodeApiKey; baseURL = $ApiBaseUrl; apiKeyRequired = $true }
    models = $Models
  }
  $App.provider | Add-Member -NotePropertyName lsrai -NotePropertyValue $AppProvider -Force
  $CliProvider = [ordered]@{
    kind = 'openai'
    options = [ordered]@{ apiKey = $script:ZCodeApiKey; baseURL = $ApiBaseUrl; apiKeyRequired = $true }
    models = $Models
  }
  $Cli.provider | Add-Member -NotePropertyName lsrai -NotePropertyValue $CliProvider -Force
  $ModelProperty = $Cli.PSObject.Properties['model']
  if ($null -eq $ModelProperty -or $null -eq $ModelProperty.Value) {
    $Cli | Add-Member -NotePropertyName model -NotePropertyValue ([pscustomobject]@{}) -Force
  } elseif ($ModelProperty.Value -is [System.Array] -or $ModelProperty.Value -is [string] -or $ModelProperty.Value -is [ValueType]) {
    Stop-Script 'ZCode CLI model 必须是对象'
  }
  $Cli.model | Add-Member -NotePropertyName main -NotePropertyValue "lsrai/$Selected" -Force
  foreach ($Path in @($ZCodeAppConfigPath, $ZCodeCliConfigPath)) {
    $Value = if ($Path -eq $ZCodeAppConfigPath) { $App } else { $Cli }
    $Json = $Value | ConvertTo-Json -Depth 100
    $TemporaryPath = "$Path.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
    try {
      [IO.File]::WriteAllText($TemporaryPath, ($Json + "`n"), [Text.UTF8Encoding]::new($false))
      Move-Item -LiteralPath $TemporaryPath -Destination $Path -Force
    } finally { Remove-Item -LiteralPath $TemporaryPath -Force -ErrorAction SilentlyContinue }
  }
}

function Write-WorkBuddyConfig {
  $ApiBaseUrl = Get-OpenAIV1BaseUrl -Value $script:BaseUrl
  try {
    $Response = Invoke-RestMethod -Uri "$ApiBaseUrl/models" -Headers @{ Authorization = "Bearer $script:WorkBuddyApiKey" } -Method GET
  } catch {
    Stop-Script "WorkBuddy 分组模型读取失败: $_"
  }
  $Ids = @($Response.data | ForEach-Object { ([string]$_.id).Trim() } |
    Where-Object { $_ -and $_ -ne 'codex-auto-review' -and $_ -match '^[A-Za-z0-9][A-Za-z0-9._:-]*$' } | Select-Object -Unique)
  if ($Ids.Count -eq 0) { Stop-Script 'WorkBuddy 分组模型目录为空' }
  if ($script:SelectedProtocol -ne 'chat_completions') { Stop-Script 'WorkBuddy 一键配置只接受已验证的 Chat Completions 协议' }
  Backup-IfNeeded $WorkBuddyModelsPath
  Ensure-Directory $WorkBuddyDir
  $Root = @()
  $Shape = 'array'
  if (Test-Path -LiteralPath $WorkBuddyModelsPath) {
    $Raw = Get-Content -LiteralPath $WorkBuddyModelsPath -Raw
    if (-not [string]::IsNullOrWhiteSpace($Raw)) {
      try { $Root = $Raw | ConvertFrom-Json } catch { Stop-Script "拒绝覆盖无法解析的 WorkBuddy 配置: $_" }
      if ($Raw.TrimStart().StartsWith('{')) {
        if ($null -eq $Root -or $Root -is [string] -or $Root -is [ValueType]) { Stop-Script 'WorkBuddy 配置根节点必须是数组或对象' }
        $Shape = 'object'
        $ModelsProperty = $Root.PSObject.Properties['models']
        if ($null -ne $ModelsProperty -and $null -ne $ModelsProperty.Value -and $ModelsProperty.Value -isnot [System.Array]) { Stop-Script 'WorkBuddy models 必须是数组' }
      } elseif (-not $Raw.TrimStart().StartsWith('[')) {
        Stop-Script 'WorkBuddy 配置根节点必须是数组或对象'
      }
    }
  }
  $Existing = if ($Shape -eq 'array') { @($Root) } elseif ($null -eq $Root.PSObject.Properties['models']) { @() } else { @($Root.models) }
  $ReasoningCatalog = $CatalogModelReasoningJson | ConvertFrom-Json
  $Managed = @()
  foreach ($Id in $Ids) {
    $Property = $ReasoningCatalog.PSObject.Properties[$Id]
    $Efforts = if ($null -ne $Property) { @($Property.Value | Where-Object { $_ -in @('minimal','low','medium','high','xhigh','max') }) } else { @() }
    if ($Efforts.Count -eq 0 -and $Id -eq 'gpt-5.6') { $Efforts = @($ReasoningCatalog.'gpt-5.6-sol') }
    if ($Efforts.Count -eq 0) { $Efforts = @('low','medium','high') }
    $DefaultEffort = if ($Efforts -contains 'medium') { 'medium' } else { [string]$Efforts[0] }
    $Managed += [ordered]@{
      id = $Id; name = $Id; vendor = 'OpenAI'; apiKey = $script:WorkBuddyApiKey
      url = "$ApiBaseUrl/chat/completions"
      supportsToolCall = $true; supportsImages = $false; supportsReasoning = $true
      onlyReasoning = $false; useCustomProtocol = $false
      maxInputTokens = if ($Id.StartsWith('grok-')) { 500000 } else { 1050000 }
      maxOutputTokens = 128000
      reasoning = [ordered]@{ defaultEffort = $DefaultEffort; supportedEfforts = $Efforts; canDisableThinking = $false }
    }
  }
  $Kept = @($Existing | Where-Object { $null -eq $_ -or [string]::IsNullOrWhiteSpace([string]$_.id) -or [string]$_.id -notin $Ids })
  $Models = @($Kept) + @($Managed)
  $Output = if ($Shape -eq 'array') { $Models } else { $Root | Add-Member -NotePropertyName models -NotePropertyValue $Models -Force; $Root }
  $Json = $Output | ConvertTo-Json -Depth 100
  $TemporaryPath = "$WorkBuddyModelsPath.tmp.$PID.$([guid]::NewGuid().ToString('N'))"
  try {
    [IO.File]::WriteAllText($TemporaryPath, ($Json + "`n"), [Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $TemporaryPath -Destination $WorkBuddyModelsPath -Force
  } finally { Remove-Item -LiteralPath $TemporaryPath -Force -ErrorAction SilentlyContinue }
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
    Write-CodexModelCatalog
    Write-CodexTomlConfig
  }
}

function Configure-Grok {
  if (Test-UsesGrok) {
    Write-Info '正在写入 Grok Build 原生模型配置'
    Set-GrokModelsFromKey
    Write-GrokTomlConfig
  }
}

function Configure-Gemini {
  if (Test-UsesGemini) {
    Write-Info '正在写入 Gemini CLI 配置'
    Write-GeminiConfig
  }
}

function Configure-Kimi {
  if ($script:Tools -eq 'kimi') {
    Write-Info '正在写入 Kimi Code 配置'
    Write-KimiConfig
  }
}

function Configure-OpenCode {
  if (Test-UsesOpenCode) {
    Write-Info '正在写入 OpenCode 配置'
    Write-OpenCodeConfig
  }
}

function Configure-ZCode {
  if (Test-UsesZCode) {
    Write-Info '正在写入 ZCode App 与 CLI 配置'
    Write-ZCodeConfig
  }
}

function Configure-WorkBuddy {
  if (Test-UsesWorkBuddy) {
    Write-Info '正在写入 WorkBuddy 模型配置'
    Write-WorkBuddyConfig
  }
}

# 通过绝对路径执行命令做一次最小自检；复用已有客户端时也不制造“未安装”的误报。
function Verify-ClientCommands {
  if ($script:Tools -in @('all', 'claude')) {
    $ClaudeCmd = if ($script:InstallClaudeClient) {
      Join-Path $NpmPrefix 'claude.cmd'
    } else {
      $script:ExistingClaudeCommand
    }
    if (-not [string]::IsNullOrWhiteSpace($ClaudeCmd) -and (Test-Path -LiteralPath $ClaudeCmd)) {
      try {
        & $ClaudeCmd --version | Out-Null
        Write-Info "Claude Code 验证通过"
      } catch {
        Stop-Script "Claude Code 安装验证失败: $_"
      }
    } elseif ($script:InstallClaudeClient) {
      Stop-Script "Claude Code 安装验证失败：未找到 $ClaudeCmd"
    }
  }

  if ($script:Tools -in @('all', 'codex')) {
    $CodexCmd = if ($script:InstallCodexClient) {
      Join-Path $NpmPrefix 'codex.cmd'
    } else {
      $script:ExistingCodexCommand
    }
    if (-not [string]::IsNullOrWhiteSpace($CodexCmd) -and (Test-Path -LiteralPath $CodexCmd)) {
      try {
        & $CodexCmd --version | Out-Null
        Write-Info "Codex 验证通过"
      } catch {
        Stop-Script "Codex 安装验证失败: $_"
      }
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingCodexApp)) {
      Write-Info 'Codex App 已检测到，配置文件写入完成；无需执行 CLI 版本检查'
    } elseif ($script:InstallCodexClient) {
      Stop-Script "Codex 安装验证失败：未找到 $CodexCmd"
    }
  }

  if (Test-UsesGrok) {
    $GrokCmd = if ($script:InstallGrokClient) { $GrokCommandPath } else { $script:ExistingGrokCommand }
    if (-not [string]::IsNullOrWhiteSpace($GrokCmd) -and (Test-Path -LiteralPath $GrokCmd)) {
      try {
        & $GrokCmd --version | Out-Null
        Write-Info 'Grok Build 验证通过'
      } catch {
        Stop-Script "Grok Build 安装验证失败: $_"
      }
    } elseif ($script:InstallGrokClient) {
      Stop-Script "Grok Build 安装验证失败：未找到 $GrokCmd"
    }
  }

  if (Test-UsesGemini) {
    $GeminiCmd = if ($script:InstallGeminiClient) {
      Join-Path $NpmPrefix 'gemini.cmd'
    } else {
      $script:ExistingGeminiCommand
    }
    if (-not [string]::IsNullOrWhiteSpace($GeminiCmd) -and (Test-Path -LiteralPath $GeminiCmd)) {
      try {
        & $GeminiCmd --version | Out-Null
        Write-Info 'Gemini CLI 验证通过'
      } catch {
        Stop-Script "Gemini CLI 安装验证失败: $_"
      }
    } elseif ($script:InstallGeminiClient) {
      Stop-Script "Gemini CLI 安装验证失败：未找到 $GeminiCmd"
    }
  }
  if (Test-UsesKimi) {
    $KimiCmd = if ($script:InstallKimiClient) { Join-Path $NpmPrefix 'kimi.cmd' } else { $script:ExistingKimiCommand }
    if (-not [string]::IsNullOrWhiteSpace($KimiCmd) -and (Test-Path -LiteralPath $KimiCmd)) {
      & $KimiCmd --version | Out-Null
      Write-Info 'Kimi Code 验证通过'
    } elseif ($script:InstallKimiClient) { Stop-Script "Kimi Code 安装验证失败：未找到 $KimiCmd" }
  }
  if (Test-UsesOpenCode) {
    $OpenCodeCmd = if ($script:InstallOpenCodeClient) { Join-Path $NpmPrefix 'opencode.cmd' } else { $script:ExistingOpenCodeCommand }
    if (-not [string]::IsNullOrWhiteSpace($OpenCodeCmd) -and (Test-Path -LiteralPath $OpenCodeCmd)) {
      & $OpenCodeCmd --version | Out-Null
      Write-Info 'OpenCode 验证通过'
    } elseif ($script:InstallOpenCodeClient) { Stop-Script "OpenCode 安装验证失败：未找到 $OpenCodeCmd" }
  }
}

# 输出最终结果和下一步指引，帮助用户立即开始使用。
function Print-Summary {
  Write-Info '老实人 AI 自动配置完成'
  Write-Host ''
  Write-Host "  - API 地址: $BaseUrl"
  Write-Host "  - 工具范围: $Tools"
  Write-Host "  - Claude 配置: $ClaudeSettingsPath"
  Write-Host "  - Codex 鉴权: $CodexAuthPath"
  Write-Host "  - Codex 配置: $CodexConfigPath"
  if (Test-UsesGrok) {
    Write-Host "  - Grok Build 配置: $GrokConfigPath"
  }
  if (Test-UsesGemini) {
    Write-Host "  - Gemini CLI 环境配置: $GeminiEnvPath"
    Write-Host "  - Gemini CLI 设置: $GeminiSettingsPath"
    Write-Host "  - Gemini CLI 默认模型: $CatalogGeminiDefaultModel"
  }
  if (Test-UsesKimi) { Write-Host "  - Kimi Code 配置: $KimiConfigPath" }
  if (Test-UsesOpenCode) { Write-Host "  - OpenCode 配置: $OpenCodeConfigPath" }
  if (Test-UsesZCode) {
    Write-Host "  - ZCode App 配置: $ZCodeAppConfigPath"
    Write-Host "  - ZCode CLI 配置: $ZCodeCliConfigPath"
  }
  if (Test-UsesWorkBuddy) { Write-Host "  - WorkBuddy 模型配置: $WorkBuddyModelsPath" }
  if (Test-UsesClaude) {
    Write-Host '  - Claude Code 专用 Key: 已配置'
    if ($script:InstallClaudeClient) {
      Write-Host '  - Claude Code CLI: 本次已安装'
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingClaudeCommand)) {
      Write-Host "  - Claude Code CLI: 已保留现有安装 ($($script:ExistingClaudeCommand))"
    }
  }
  if (Test-UsesCodex) {
    Write-Host '  - Codex 专用 Key: 已配置'
    if ($script:InstallCodexClient) {
      Write-Host '  - Codex CLI: 本次已安装'
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingCodexCommand)) {
      Write-Host "  - Codex CLI: 已保留现有安装 ($($script:ExistingCodexCommand))"
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingCodexApp)) {
      Write-Host '  - Codex App: 已保留现有安装，未重复下载'
    }
  }
  if (Test-UsesGrok) {
    Write-Host '  - Grok Build 专用 Key: 已配置'
    if ($script:InstallGrokClient) {
      Write-Host '  - Grok Build CLI: 本次已安装'
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingGrokCommand)) {
      Write-Host "  - Grok Build CLI: 已保留现有安装 ($($script:ExistingGrokCommand))"
    }
  }
  if (Test-UsesGemini) {
    Write-Host '  - Gemini CLI 专用 Key: 已配置'
    if ($script:InstallGeminiClient) {
      Write-Host '  - Gemini CLI: 本次已安装'
    } elseif (-not [string]::IsNullOrWhiteSpace($script:ExistingGeminiCommand)) {
      Write-Host "  - Gemini CLI: 已保留现有安装 ($($script:ExistingGeminiCommand))"
    }
  }
  if (Test-UsesKimi) { Write-Host '  - Kimi Code 专用 Key: 已配置' }
  if (Test-UsesOpenCode) { Write-Host '  - OpenCode 专用 Key: 已配置' }
  if (Test-UsesZCode) { Write-Host '  - ZCode 专用 Key: 已配置' }
  if (Test-UsesWorkBuddy) { Write-Host '  - WorkBuddy 专用 Key: 已配置' }
  Write-Host ''
  Write-Host '回滚方法（仅显示本次存在的备份）:'
  foreach ($RollbackPath in @($ClaudeSettingsPath, $CodexAuthPath, $CodexConfigPath, $CodexModelCatalogPath, $GrokConfigPath, $GeminiEnvPath, $GeminiSettingsPath, $KimiConfigPath, $OpenCodeConfigPath, $ZCodeAppConfigPath, $ZCodeCliConfigPath, $WorkBuddyModelsPath)) {
    if (Test-Path -LiteralPath "$RollbackPath.bak") {
      Write-Host "  Copy-Item -LiteralPath '$RollbackPath.bak' -Destination '$RollbackPath' -Force"
    }
  }
  Write-Host ''
  if ($script:BalanceReady) {
    Write-Host '✅ 余额/套餐额度充足，现在可以直接使用。'
  } else {
    Write-Warning '安装和配置已经完成，但余额/套餐额度不足。'
    Write-Host "请充值或购买套餐后直接打开使用：$DefaultTopupUrl"
  }
  Write-Host ''
  Write-Host '下一步:'
  if ($script:Tools -in @('all', 'claude')) {
    Write-Host '  - 重新打开 PowerShell 后执行 claude --version'
  }
  if ($script:Tools -in @('all', 'codex')) {
    if (-not [string]::IsNullOrWhiteSpace($script:ExistingCodexApp) -and
        [string]::IsNullOrWhiteSpace($script:ExistingCodexCommand) -and
        -not $script:InstallCodexClient) {
      Write-Host '  - 完全退出后重新打开 Codex App，即可使用新配置'
    } else {
      Write-Host '  - 重新打开 PowerShell 后执行 codex --version'
    }
  }
  if (Test-UsesGrok) {
    Write-Host '  - 重新打开 PowerShell 后执行 grok --version'
    Write-Host "  - 再执行 grok -m $CatalogGrokDefaultModel -p `"只回复 OK`""
  }
  if (Test-UsesGemini) {
    Write-Host '  - 重新打开 PowerShell 后执行 gemini --version'
  }
  if (Test-UsesKimi) { Write-Host '  - 重新打开 PowerShell 后执行 kimi --version' }
  if (Test-UsesOpenCode) { Write-Host '  - 重新打开 PowerShell 后执行 opencode --version' }
  if (Test-UsesZCode) { Write-Host '  - 完全退出并重新打开 ZCode' }
  if (Test-UsesWorkBuddy) { Write-Host '  - 完全退出并重新打开 WorkBuddy' }
}

# 组织整个安装流程，确保安装、配置、校验按固定顺序执行。
# 注意：通过 `irm | iex` 管道执行时，$args 始终为空，参数需通过环境变量传入。
#   命令行方式：.\install.ps1 --tools claude --api-key <key>
#   管道方式：  $env:LAOSHIRENAI_TOOLS='claude'; $env:LAOSHIRENAI_API_KEY='<key>'; irm ... | iex
function Main {
  Write-Info "老实人 AI 自动配置脚本 v$ScriptVersion"
  # 仅在非管道（直接执行脚本）时才解析命令行参数
  if ($MyInvocation.InvocationName -ne '&' -and $args.Count -gt 0) {
    Parse-Arguments -ArgsList $args
  }
  Prompt-ApiKeys
  if (-not [string]::IsNullOrWhiteSpace($script:SetupToken)) {
    Exchange-SetupTicket
  } else {
    Apply-ManualSelection
  }
  Resolve-ClientInstallPlan
  Resolve-ClientUpdatePlan
  if ((Test-NeedsNpmClientInstall) -or ($script:GrokCcSwitchCompat -and (Test-UsesGrok))) {
    Ensure-NodeRuntime
  }
  if (Test-NeedsNpmClientInstall) {
    Ensure-GitBash
  }
  if (Test-NeedsClientInstall) {
    Ensure-UserPath
  } else {
    Write-Info '检测到所选客户端已存在或已要求跳过安装；不下载 Node.js、不修改 PATH'
  }
  Install-RequestedClients
  Remove-ManagedPowerShellShims
  Install-CodexAppIfRequested
  Configure-Claude
  Configure-Codex
  Configure-Grok
  Configure-Gemini
  Configure-Kimi
  Configure-OpenCode
  Configure-ZCode
  Configure-WorkBuddy
  Test-ClaudeApiKey
  Test-CodexApiKey
  Test-GrokApiKey
  Test-GeminiApiKey
  Test-KimiApiKey
  Test-OpenCodeApiKey
  Test-ZCodeApiKey
  Test-WorkBuddyApiKey
  Test-SelectedModelRequest
  Verify-ClientCommands
  Open-CcSwitchIfRequested
  Print-Summary
}

Main
