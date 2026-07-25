Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$ScriptVersion = '1.0.0'
$MinimumVersion = [Version]'3.16.5'
$ReleaseUrl = 'https://github.com/farion1231/cc-switch/releases/latest'

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
    Write-Host "已为你打开 CC Switch 官方下载页。"
  } catch {
    Write-Host "请打开下载页: $ReleaseUrl"
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

Write-Host ""
Write-Host "CC Switch 自动诊断修复 v$ScriptVersion" -ForegroundColor White
Write-Host "它只检查本机 CC Switch 版本和 ccswitch:// 协议，不会读取或上传 API Key。"
Write-Host ""

$RunningOnWindows = $env:OS -eq 'Windows_NT'
if (-not $RunningOnWindows) {
  throw '这条命令只适用于 Windows。macOS 请使用页面提供的“Mac”命令。'
}

$Candidates = [System.Collections.Generic.List[string]]::new()
$ProtocolCommand = ''
$ProtocolKey = 'Registry::HKEY_CLASSES_ROOT\ccswitch\shell\open\command'

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

Add-Candidate -List $Candidates -Path (Join-Path $env:LOCALAPPDATA 'Programs\CC Switch\cc-switch.exe')

# Portable ZIP users often leave CC Switch in Downloads or on the Desktop.
# Check the root and its recent first-level folders so they do not need to
# understand install locations or manually register the protocol.
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
    Where-Object { $_.DisplayName -like 'CC Switch*' } |
    ForEach-Object {
      if ($_.InstallLocation) {
        Add-Candidate -List $Candidates -Path (Join-Path $_.InstallLocation 'cc-switch.exe')
      }
      if ($_.DisplayIcon) {
        Add-Candidate -List $Candidates -Path ($_.DisplayIcon -replace ',\d+$', '')
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
  Write-Problem '没有找到可用的 CC Switch。'
  Write-Host '如果你使用绿色便携版：请先双击打开 cc-switch.exe，再重新粘贴运行本命令。'
  Write-Host '如果还没有安装：请在下载页选择 Windows.msi 安装版，它会自动注册 Deep Link。'
  Open-ReleasePage
  Write-Host ""
  Read-Host '处理完成后按回车关闭窗口'
  return
}

$Executable = (Resolve-Path -LiteralPath $Executable).Path
Write-Host "检测到 CC Switch: $Executable"

$VersionText = (Get-Item -LiteralPath $Executable).VersionInfo.ProductVersion
$InstalledVersion = Convert-ToVersion -Value $VersionText
$NeedsUpgrade = $false

if ($InstalledVersion) {
  Write-Host "当前版本: $InstalledVersion"
  if ($InstalledVersion -lt $MinimumVersion) {
    $NeedsUpgrade = $true
    Write-Problem "版本低于 $MinimumVersion，部分一键导入功能可能无法使用。"
  }
} else {
  Write-Problem '无法读取版本号，将先修复 Deep Link。若导入仍失败，请升级最新版。'
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
    throw '协议写入后验证失败。请改用最新版 Windows.msi 安装版。'
  }
  Write-Ok 'Deep Link 已修复。'
}

if ($NeedsUpgrade) {
  Write-Problem 'Deep Link 已修复，但 CC Switch 版本太旧。'
  Write-Host '已打开官方下载页，请选择 Windows.msi 安装版覆盖安装，然后回到网页重新点击“一键导入”。'
  Open-ReleasePage
} else {
  Write-Ok '诊断修复完成。请回到网页，重新点击“一键导入”。'
}

Write-Host ""
Read-Host '按回车关闭窗口'
