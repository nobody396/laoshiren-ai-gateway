$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$script:ClaudeDesktopManifestUrl = 'https://laoshirenai.com/api/v1/public-downloads/claude-desktop/latest.json'
$script:ClaudeDesktopPackagePrefix = 'https://laoshirenai.com/downloads/claude-desktop/'

function Write-Step {
  param([string]$Message)
  Write-Host "[Claude Desktop] $Message" -ForegroundColor Cyan
}

function Get-NativeArchitecture {
  $Architecture = if (-not [string]::IsNullOrWhiteSpace($env:PROCESSOR_ARCHITEW6432)) {
    $env:PROCESSOR_ARCHITEW6432
  } else {
    $env:PROCESSOR_ARCHITECTURE
  }
  switch ($Architecture.ToUpperInvariant()) {
    'AMD64' { return 'x64' }
    'ARM64' { return 'arm64' }
    default { throw "当前仅支持 Windows x64/ARM64，检测到：$Architecture" }
  }
}

function Assert-SameSiteClaudeAsset {
  param([object]$Asset)
  if ($null -eq $Asset -or [string]::IsNullOrWhiteSpace([string]$Asset.download_url)) {
    throw '安装包清单缺少本站下载地址'
  }
  $Download = [Uri]([string]$Asset.download_url)
  $Prefix = [Uri]$script:ClaudeDesktopPackagePrefix
  $SameOrigin = $Download.Scheme -eq $Prefix.Scheme -and $Download.Host -eq $Prefix.Host -and $Download.Port -eq $Prefix.Port
  if (-not $SameOrigin -or -not $Download.AbsolutePath.StartsWith($Prefix.AbsolutePath, [StringComparison]::Ordinal)) {
    throw "安装包未通过本站同源校验：$Download"
  }
}

function Get-ClaudeDesktopAssetPair {
  param([string]$Architecture)
  Write-Step '正在读取本站最新版清单...'
  $Manifest = Invoke-RestMethod -Uri $script:ClaudeDesktopManifestUrl -TimeoutSec 60
  $Assets = @($Manifest.assets)
  $Installer = @($Assets | Where-Object {
      $_.platform -eq 'windows' -and $_.arch -eq $Architecture -and
      $_.role -eq 'installer' -and ([string]$_.name).EndsWith('.msix', [StringComparison]::OrdinalIgnoreCase)
    })
  $Component = @($Assets | Where-Object {
      $_.platform -eq 'windows' -and $_.arch -eq $Architecture -and
      $_.role -eq 'claude-desktop-code' -and ([string]$_.name).EndsWith('.exe', [StringComparison]::OrdinalIgnoreCase)
    })
  if ($Installer.Count -ne 1 -or $Component.Count -ne 1) {
    throw "最新版清单没有唯一的 $Architecture 安装包与 Code 本地组件，请稍后重试"
  }
  foreach ($Asset in @($Installer[0], $Component[0])) {
    Assert-SameSiteClaudeAsset -Asset $Asset
    if ([string]$Asset.sha256 -notmatch '^[A-Fa-f0-9]{64}$' -or [int64]$Asset.size -le 0) {
      throw "安装包清单校验信息无效：$($Asset.name)"
    }
  }
  if ([string]$Component[0].component_version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+') {
    throw 'Code 本地组件版本无效'
  }
  if ([string]$Component[0].upstream_sha256 -notmatch '^[A-Fa-f0-9]{64}$') {
    throw 'Code 本地组件缺少官方压缩包校验值'
  }
  return [pscustomobject]@{ Installer = $Installer[0]; Component = $Component[0] }
}

function Download-VerifiedClaudeAsset {
  param([object]$Asset, [string]$OutputPath)
  Assert-SameSiteClaudeAsset -Asset $Asset
  Remove-Item -LiteralPath $OutputPath -Force -ErrorAction SilentlyContinue
  Write-Step "正在通过本站缓存下载 $($Asset.name)..."
  & curl.exe -fL --retry 5 --retry-delay 2 --connect-timeout 60 --max-time 3600 -o $OutputPath ([string]$Asset.download_url)
  if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $OutputPath -PathType Leaf)) {
    throw "下载失败：$($Asset.name)"
  }
  $File = Get-Item -LiteralPath $OutputPath
  if ($File.Length -ne [int64]$Asset.size) {
    throw "文件大小校验失败：$($Asset.name)"
  }
  $Actual = (Get-FileHash -LiteralPath $OutputPath -Algorithm SHA256).Hash
  if ($Actual -ne ([string]$Asset.sha256).ToUpperInvariant()) {
    throw "SHA256 校验失败：$($Asset.name)"
  }
}

function Write-Utf8NoBom {
  param([string]$Path, [string]$Content)
  [IO.File]::WriteAllText($Path, $Content, (New-Object Text.UTF8Encoding($false)))
}

function Install-ClaudeDesktopCodeComponent {
  param([object]$Asset, [string]$SourcePath)
  $Root = Join-Path $env:LOCALAPPDATA 'Claude-3p\claude-code'
  $Version = [string]$Asset.component_version
  $Target = Join-Path $Root $Version
  $Stage = "$Target.stage.$([guid]::NewGuid().ToString('N'))"
  $Backup = "$Target.backup.$([guid]::NewGuid().ToString('N'))"
  New-Item -ItemType Directory -Path $Stage -Force | Out-Null
  try {
    Copy-Item -LiteralPath $SourcePath -Destination (Join-Path $Stage 'claude.exe') -Force
    $PayloadHash = ([string]$Asset.sha256).ToLowerInvariant()
    $Payload = @{ sha256 = $PayloadHash; size = [int64]$Asset.size } | ConvertTo-Json -Compress
    Write-Utf8NoBom -Path (Join-Path $Stage '.payload') -Content $Payload
    Write-Utf8NoBom -Path (Join-Path $Stage '.verified') -Content ([string]$Asset.upstream_sha256).ToLowerInvariant()
    if ((Get-FileHash -LiteralPath (Join-Path $Stage 'claude.exe') -Algorithm SHA256).Hash -ne $PayloadHash.ToUpperInvariant()) {
      throw 'Code 本地组件写入后校验失败'
    }

    New-Item -ItemType Directory -Path $Root -Force | Out-Null
    if (Test-Path -LiteralPath $Target) {
      Move-Item -LiteralPath $Target -Destination $Backup
    }
    try {
      Move-Item -LiteralPath $Stage -Destination $Target
    } catch {
      if (Test-Path -LiteralPath $Backup) {
        Move-Item -LiteralPath $Backup -Destination $Target
      }
      throw
    }
    Remove-Item -LiteralPath $Backup -Recurse -Force -ErrorAction SilentlyContinue
  } finally {
    Remove-Item -LiteralPath $Stage -Recurse -Force -ErrorAction SilentlyContinue
  }
  return $Target
}

function Install-ClaudeDesktop {
  $Architecture = Get-NativeArchitecture
  $Pair = Get-ClaudeDesktopAssetPair -Architecture $Architecture
  $TempRoot = Join-Path ([IO.Path]::GetTempPath()) ("laoshirenai-claude-desktop-" + [guid]::NewGuid().ToString('N'))
  New-Item -ItemType Directory -Path $TempRoot -Force | Out-Null
  $MSIX = Join-Path $TempRoot 'Claude-Desktop.msix'
  $Code = Join-Path $TempRoot 'claude.exe'
  try {
    Download-VerifiedClaudeAsset -Asset $Pair.Installer -OutputPath $MSIX
    Download-VerifiedClaudeAsset -Asset $Pair.Component -OutputPath $Code
    Get-Process -Name 'Claude' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Write-Step '正在安装或升级 Claude Desktop...'
    Add-AppxPackage -Path $MSIX -ForceApplicationShutdown
    Write-Step '正在准备 Code 本地组件...'
    $InstalledComponent = Install-ClaudeDesktopCodeComponent -Asset $Pair.Component -SourcePath $Code
    if (-not (Test-Path -LiteralPath (Join-Path $InstalledComponent 'claude.exe') -PathType Leaf)) {
      throw 'Code 本地组件安装后不存在'
    }
    Write-Host ''
    Write-Host '安装完成。下一步请打开 CC Switch，在 Claude Desktop 页面导入并启用配置，然后使用普通聊天或 Code 的 Local 模式。' -ForegroundColor Green
    Write-Host '中国大陆无魔法、无代理环境下不能使用 Cowork；Cowork 依赖 Anthropic 官方云端工作区。' -ForegroundColor Yellow
  } finally {
    Remove-Item -LiteralPath $TempRoot -Recurse -Force -ErrorAction SilentlyContinue
  }
}

Install-ClaudeDesktop
