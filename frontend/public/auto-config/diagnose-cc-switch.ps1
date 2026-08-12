Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ScriptVersion = '1.2.1'
$MinimumVersion = [Version]'3.16.5'
$ReleaseUrl = 'https://github.com/farion1231/cc-switch/releases/latest'
$ReleaseApiUrl = 'https://api.github.com/repos/farion1231/cc-switch/releases/latest'
$MirrorManifestUrl = 'https://laoshirenai.com/api/v1/public-downloads/cc-switch/latest.json'
$MirrorPackagePrefix = 'https://laoshirenai.com/api/v1/public-downloads/cc-switch/packages/'
$InstalledExecutable = Join-Path $env:LOCALAPPDATA 'Programs\CC Switch\cc-switch.exe'

function Write-Step {
  param([string]$Message)
  Write-Host "[CC Switch] $Message" -ForegroundColor Cyan
}

function Write-Ok {
  param([string]$Message)
  Write-Host "[完成] $Message" -ForegroundColor Green
}

function Write-Problem {
  param([string]$Message)
  Write-Host "[需要处理] $Message" -ForegroundColor Yellow
}

function Open-ReleasePage {
  if ($env:CCS_DIAGNOSTIC_NO_OPEN -eq '1') {
    Write-Host "下载地址: $ReleaseUrl"
    return
  }

  try {
    Start-Process $ReleaseUrl
    Write-Host '自动安装没有完成，已为你打开 CC Switch 官方下载页作为兜底。'
  } catch {
    Write-Host "请打开官方下载页: $ReleaseUrl"
  }
}

function Get-ExecutableFromCommand {
  param([string]$Command)

  if ([string]::IsNullOrWhiteSpace($Command)) {
    return ''
  }

  $Expanded = [Environment]::ExpandEnvironmentVariables($Command.Trim())
  $QuotedMatch = [Regex]::Match($Expanded, '^\s*"([^"]+\.exe)"')
  if ($QuotedMatch.Success) {
    return $QuotedMatch.Groups[1].Value
  }

  $PlainMatch = [Regex]::Match($Expanded, '^\s*([^\s]+\.exe)')
  if ($PlainMatch.Success) {
    return $PlainMatch.Groups[1].Value
  }

  return ''
}

function Convert-ToVersion {
  param([string]$Value)

  if ([string]::IsNullOrWhiteSpace($Value)) {
    return $null
  }

  $Match = [Regex]::Match($Value, '\d+\.\d+\.\d+(?:\.\d+)?')
  if (-not $Match.Success) {
    return $null
  }

  try {
    return [Version]$Match.Value
  } catch {
    return $null
  }
}

function Add-Candidate {
  param(
    [System.Collections.Generic.List[string]]$List,
    [string]$Path
  )

  if ([string]::IsNullOrWhiteSpace($Path)) {
    return
  }

  $Expanded = [Environment]::ExpandEnvironmentVariables($Path.Trim().Trim('"'))
  if (-not $List.Contains($Expanded)) {
    $List.Add($Expanded)
  }
}

function Get-OptionalPropertyValue {
  param(
    [object]$InputObject,
    [string]$Name
  )

  if ($null -eq $InputObject) {
    return $null
  }

  $Property = $InputObject.PSObject.Properties[$Name]
  if ($null -eq $Property) {
    return $null
  }

  return $Property.Value
}

function Get-WindowsReleaseAssetName {
  $Architecture = if ($env:PROCESSOR_ARCHITEW6432) {
    $env:PROCESSOR_ARCHITEW6432
  } else {
    $env:PROCESSOR_ARCHITECTURE
  }

  if ([string]::IsNullOrWhiteSpace($Architecture)) {
    throw '无法识别当前 Windows 架构。'
  }

  switch ($Architecture.ToUpperInvariant()) {
    'ARM64' { return 'Windows-arm64.msi' }
    'AMD64' { return 'Windows.msi' }
    default { throw "暂不支持当前 Windows 架构: $Architecture" }
  }
}

function Wait-For-CcSwitchExit {
  $Processes = @(Get-Process -Name 'cc-switch' -ErrorAction SilentlyContinue)
  if ($Processes.Count -eq 0) {
    return
  }

  Write-Step '升级前正在安全关闭 CC Switch...'
  foreach ($Process in $Processes) {
    try {
      if ($Process.MainWindowHandle -ne 0) {
        [void]$Process.CloseMainWindow()
      }
    } catch {
      # The tray process may not expose a closable main window.
    }
  }

  Start-Sleep -Seconds 3
  $Processes = @(Get-Process -Name 'cc-switch' -ErrorAction SilentlyContinue)
  if ($Processes.Count -eq 0) {
    return
  }

  Write-Problem 'CC Switch 仍在系统托盘运行。为了保护本地代理和配置，脚本不会强制结束进程。'
  Write-Host '请看屏幕右下角：找到 CC Switch 图标，右键点击，然后选择“退出”。'
  [void](Read-Host '退出 CC Switch 后按回车，脚本会自动继续升级')

  $Deadline = (Get-Date).AddSeconds(20)
  do {
    $Processes = @(Get-Process -Name 'cc-switch' -ErrorAction SilentlyContinue)
    if ($Processes.Count -eq 0) {
      return
    }
    Start-Sleep -Milliseconds 500
  } while ((Get-Date) -lt $Deadline)

  throw 'CC Switch 仍在运行。请从系统托盘完全退出后重新执行本命令。'
}

function Get-MirrorCcSwitchAsset {
  Write-Step '正在查询老实人 AI 本站缓存的 CC Switch 最新版...'
  $Manifest = Invoke-RestMethod -Uri $MirrorManifestUrl -Method Get
  $LatestVersion = Convert-ToVersion -Value ([string]$Manifest.version)
  if (-not $LatestVersion) {
    throw '本站缓存版本信息格式不正确。'
  }

  $AssetSuffix = Get-WindowsReleaseAssetName
  $ExpectedName = "CC-Switch-v$LatestVersion-$AssetSuffix"
  $Assets = @($Manifest.assets | Where-Object { $_.name -eq $ExpectedName })
  if ($Assets.Count -ne 1) {
    throw "本站缓存中没有找到唯一安装包: $ExpectedName"
  }

  $Asset = $Assets[0]
  $ExpectedHash = ([string]$Asset.sha256).Trim().ToLowerInvariant()
  $DownloadUrl = ([string]$Asset.download_url).Trim()
  if ($ExpectedHash -notmatch '^[0-9a-f]{64}$') {
    throw '本站缓存安装包没有可验证的 SHA-256 摘要。'
  }
  if (-not $DownloadUrl.StartsWith($MirrorPackagePrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw '本站缓存返回了不受信任的下载地址。'
  }

  return [PSCustomObject]@{
    Version = $LatestVersion
    Name = $ExpectedName
    DownloadUrl = $DownloadUrl
    SHA256 = $ExpectedHash
    Source = '老实人 AI 本站缓存'
  }
}

function Get-OfficialCcSwitchAsset {
  Write-Step '本站缓存暂不可用，正在查询 CC Switch 官方 GitHub 作为兜底...'
  $Headers = @{
    'Accept' = 'application/vnd.github+json'
    'User-Agent' = "laoshirenai-cc-switch-diagnostic/$ScriptVersion"
    'X-GitHub-Api-Version' = '2022-11-28'
  }
  $Release = Invoke-RestMethod -Uri $ReleaseApiUrl -Headers $Headers -Method Get
  $LatestVersion = Convert-ToVersion -Value ([string]$Release.tag_name)
  if (-not $LatestVersion) {
    throw '官方版本信息格式不正确。'
  }

  $AssetSuffix = Get-WindowsReleaseAssetName
  $ExpectedName = "CC-Switch-v$LatestVersion-$AssetSuffix"
  $Assets = @($Release.assets | Where-Object { $_.name -eq $ExpectedName })
  if ($Assets.Count -ne 1) {
    throw "官方发布中没有找到唯一安装包: $ExpectedName"
  }

  $Asset = $Assets[0]
  $DigestMatch = [Regex]::Match([string]$Asset.digest, '^sha256:([0-9a-fA-F]{64})$')
  if (-not $DigestMatch.Success) {
    throw '官方安装包没有可验证的 SHA-256 摘要。'
  }

  return [PSCustomObject]@{
    Version = $LatestVersion
    Name = $ExpectedName
    DownloadUrl = [string]$Asset.browser_download_url
    SHA256 = $DigestMatch.Groups[1].Value.ToLowerInvariant()
    Source = 'CC Switch 官方 GitHub'
  }
}

function Get-LatestCcSwitchAsset {
  try {
    return Get-MirrorCcSwitchAsset
  } catch {
    Write-Problem "本站缓存暂时不可用：$($_.Exception.Message)"
    return Get-OfficialCcSwitchAsset
  }
}

function Install-LatestCcSwitch {
  param([object]$ReleaseAsset)

  if (-not $ReleaseAsset) {
    $ReleaseAsset = Get-LatestCcSwitchAsset
  }
  $LatestVersion = $ReleaseAsset.Version
  $TemporaryMsi = Join-Path $env:TEMP "cc-switch-$LatestVersion-$([Guid]::NewGuid().ToString('N')).msi"
  $PreviousProgressPreference = $ProgressPreference

  try {
    Write-Step "正在从$($ReleaseAsset.Source)下载 CC Switch $LatestVersion..."
    $ProgressPreference = 'SilentlyContinue'
    Invoke-WebRequest -Uri $ReleaseAsset.DownloadUrl -OutFile $TemporaryMsi -UseBasicParsing
    $ActualHash = (Get-FileHash -LiteralPath $TemporaryMsi -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($ActualHash -ne $ReleaseAsset.SHA256) {
      throw '安装包校验失败，已停止安装。'
    }
    Write-Ok "$($ReleaseAsset.Source)安装包 SHA-256 校验通过。"

    Wait-For-CcSwitchExit

    Write-Step '正在自动安装最新版（当前用户，无需管理员权限）...'
    $MsiArguments = @('/i', ('"{0}"' -f $TemporaryMsi), '/passive', '/norestart')
    $Installer = Start-Process -FilePath 'msiexec.exe' -ArgumentList $MsiArguments -Wait -PassThru
    if ($Installer.ExitCode -notin @(0, 1641, 3010)) {
      throw "Windows 安装程序返回错误码 $($Installer.ExitCode)。"
    }

    $Deadline = (Get-Date).AddSeconds(15)
    while (-not (Test-Path -LiteralPath $InstalledExecutable -PathType Leaf) -and (Get-Date) -lt $Deadline) {
      Start-Sleep -Milliseconds 500
    }
    if (-not (Test-Path -LiteralPath $InstalledExecutable -PathType Leaf)) {
      throw '安装程序已结束，但没有找到新版 CC Switch。'
    }

    $VersionText = (Get-Item -LiteralPath $InstalledExecutable).VersionInfo.ProductVersion
    $VerifiedVersion = Convert-ToVersion -Value $VersionText
    if (-not $VerifiedVersion -or $VerifiedVersion -lt $LatestVersion) {
      throw '安装后版本验证失败。'
    }

    Write-Ok "CC Switch 已自动升级到 $VerifiedVersion。"
    return [PSCustomObject]@{
      Executable = (Resolve-Path -LiteralPath $InstalledExecutable).Path
      Version = $VerifiedVersion
    }
  } finally {
    $ProgressPreference = $PreviousProgressPreference
    if (Test-Path -LiteralPath $TemporaryMsi) {
      Remove-Item -LiteralPath $TemporaryMsi -Force -ErrorAction SilentlyContinue
    }
  }
}

if ($env:CCS_DIAGNOSTIC_LIBRARY_ONLY -eq '1') {
  return
}

Write-Host ''
Write-Host "CC Switch 自动诊断修复 v$ScriptVersion" -ForegroundColor White
Write-Host '它只检查本机 CC Switch 版本和 ccswitch:// 协议，不会读取或上传 API Key。'
Write-Host '缺失或版本过旧时，会优先从老实人 AI 本站缓存下载、校验并自动安装；本站不可用才访问官方 GitHub。'
Write-Host ''

$RunningOnWindows = $env:OS -eq 'Windows_NT'
if (-not $RunningOnWindows) {
  throw '这条命令只适用于 Windows。macOS 请使用页面提供的“Mac”命令。'
}

$Candidates = [System.Collections.Generic.List[string]]::new()
$ProtocolCommand = ''
$ProtocolKey = 'Registry::HKEY_CLASSES_ROOT\ccswitch\shell\open\command'
$WasUpgraded = $false
$InstalledVersion = $null

Write-Step '正在检查 Deep Link 注册...'
try {
  $ProtocolCommand = (Get-Item -LiteralPath $ProtocolKey -ErrorAction Stop).GetValue('')
} catch {
  Write-Problem '没有检测到 ccswitch:// 协议，稍后将自动修复。'
}

Write-Step '正在查找已经安装或正在运行的 CC Switch...'
try {
  Get-Process -Name 'cc-switch' -ErrorAction SilentlyContinue |
    Where-Object { $_.Path } |
    ForEach-Object { Add-Candidate -List $Candidates -Path $_.Path }
} catch {
  # Some Windows policies do not expose process paths. Continue with install locations.
}

Add-Candidate -List $Candidates -Path $InstalledExecutable

# Portable ZIP users often leave CC Switch in Downloads or on the Desktop.
$PortableRoots = @(
  (Join-Path $env:USERPROFILE 'Downloads'),
  [Environment]::GetFolderPath('Desktop')
)

foreach ($Root in $PortableRoots) {
  if ([string]::IsNullOrWhiteSpace($Root) -or -not (Test-Path -LiteralPath $Root -PathType Container)) {
    continue
  }

  Add-Candidate -List $Candidates -Path (Join-Path $Root 'cc-switch.exe')
  Get-ChildItem -LiteralPath $Root -Directory -ErrorAction SilentlyContinue |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 40 |
    ForEach-Object {
      Add-Candidate -List $Candidates -Path (Join-Path $_.FullName 'cc-switch.exe')
    }
}

$UninstallRoots = @(
  'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
  'HKLM:\Software\Microsoft\Windows\CurrentVersion\Uninstall\*',
  'HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'
)

foreach ($Root in $UninstallRoots) {
  Get-ItemProperty -Path $Root -ErrorAction SilentlyContinue |
    Where-Object {
      (Get-OptionalPropertyValue -InputObject $_ -Name 'DisplayName') -like 'CC Switch*'
    } |
    ForEach-Object {
      $InstallLocation = Get-OptionalPropertyValue -InputObject $_ -Name 'InstallLocation'
      if ($InstallLocation) {
        Add-Candidate -List $Candidates -Path (Join-Path $InstallLocation 'cc-switch.exe')
      }
      $DisplayIcon = Get-OptionalPropertyValue -InputObject $_ -Name 'DisplayIcon'
      if ($DisplayIcon) {
        Add-Candidate -List $Candidates -Path ($DisplayIcon -replace ',\d+$', '')
      }
    }
}

# Use the existing protocol handler only after running/installed locations. This
# lets the script repair a stale handler that still points to an older copy.
Add-Candidate -List $Candidates -Path (Get-ExecutableFromCommand -Command $ProtocolCommand)

$Executable = $Candidates |
  Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } |
  Select-Object -First 1

if (-not $Executable) {
  Write-Problem '没有找到可用的 CC Switch，将自动安装最新版。'
  try {
    $InstallResult = Install-LatestCcSwitch
    $Executable = $InstallResult.Executable
    $InstalledVersion = $InstallResult.Version
    $WasUpgraded = $true
  } catch {
    Write-Problem "自动安装失败: $($_.Exception.Message)"
    Open-ReleasePage
    Write-Host ''
    Read-Host '处理完成后按回车关闭窗口'
    return
  }
} else {
  $Executable = (Resolve-Path -LiteralPath $Executable).Path
  Write-Host "检测到 CC Switch: $Executable"

  $VersionText = (Get-Item -LiteralPath $Executable).VersionInfo.ProductVersion
  $InstalledVersion = Convert-ToVersion -Value $VersionText
  if ($InstalledVersion) {
    Write-Host "当前版本: $InstalledVersion"
    try {
      $LatestAsset = Get-LatestCcSwitchAsset
      if ($InstalledVersion -lt $LatestAsset.Version) {
        Write-Problem "检测到最新版 $($LatestAsset.Version)，将自动升级。"
        $InstallResult = Install-LatestCcSwitch -ReleaseAsset $LatestAsset
        $Executable = $InstallResult.Executable
        $InstalledVersion = $InstallResult.Version
        $WasUpgraded = $true
      }
    } catch {
      if ($InstalledVersion -lt $MinimumVersion) {
        Write-Problem "自动升级失败: $($_.Exception.Message)"
        Open-ReleasePage
        Write-Host ''
        Read-Host '处理完成后按回车关闭窗口'
        return
      }
      Write-Problem "暂时无法检查或安装最新版，将继续修复 Deep Link：$($_.Exception.Message)"
    }
  } else {
    Write-Problem '无法读取版本号，将先修复 Deep Link。若导入仍失败，可重新运行本命令自动安装最新版。'
  }
}

$RegisteredExecutable = Get-ExecutableFromCommand -Command $ProtocolCommand
$ProtocolIsHealthy = $false
if ($RegisteredExecutable -and (Test-Path -LiteralPath $RegisteredExecutable -PathType Leaf)) {
  try {
    $ProtocolIsHealthy = (Resolve-Path -LiteralPath $RegisteredExecutable).Path -eq $Executable
  } catch {
    $ProtocolIsHealthy = $false
  }
}

if ($ProtocolIsHealthy) {
  Write-Ok 'ccswitch:// 协议已经指向当前 CC Switch。'
} else {
  Write-Step '正在修复 ccswitch:// 协议（当前用户，无需管理员权限）...'
  $ProtocolRoot = 'HKCU:\Software\Classes\ccswitch'
  New-Item -Path "$ProtocolRoot\DefaultIcon" -Force | Out-Null
  New-Item -Path "$ProtocolRoot\shell\open\command" -Force | Out-Null
  Set-Item -LiteralPath $ProtocolRoot -Value 'URL:CC Switch Protocol'
  New-ItemProperty -LiteralPath $ProtocolRoot -Name 'URL Protocol' -Value '' -PropertyType String -Force | Out-Null
  Set-Item -LiteralPath "$ProtocolRoot\DefaultIcon" -Value "`"$Executable`",0"
  Set-Item -LiteralPath "$ProtocolRoot\shell\open\command" -Value "`"$Executable`" `"%1`""

  $VerifiedCommand = (Get-Item -LiteralPath $ProtocolKey -ErrorAction Stop).GetValue('')
  $VerifiedExecutable = Get-ExecutableFromCommand -Command $VerifiedCommand
  if (-not $VerifiedExecutable -or -not (Test-Path -LiteralPath $VerifiedExecutable -PathType Leaf)) {
    throw '协议写入后验证失败。请重新运行本命令。'
  }
  Write-Ok 'Deep Link 已修复。'
}

if ($env:CCS_DIAGNOSTIC_NO_LAUNCH -ne '1') {
  try {
    Start-Process -FilePath $Executable -ArgumentList '--register-protocol' | Out-Null
    if ($WasUpgraded) {
      Write-Ok '最新版 CC Switch 已重新打开。'
    }
  } catch {
    Write-Problem 'CC Switch 已修复，但没有自动打开；请手动打开一次。'
  }
}

Write-Ok '诊断、升级与修复完成。请回到网页，重新点击“一键导入”。'
Write-Host ''
Read-Host '按回车关闭窗口'
