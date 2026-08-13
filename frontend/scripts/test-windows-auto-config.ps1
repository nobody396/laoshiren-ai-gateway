Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Assert-True {
  param(
    [bool]$Condition,
    [string]$Message
  )

  if (-not $Condition) {
    throw $Message
  }
}

$InstallerPath = Join-Path $PSScriptRoot '..\public\auto-config\install.ps1'
$InstallerBytes = [IO.File]::ReadAllBytes($InstallerPath)
Assert-True ($InstallerBytes.Length -ge 3 -and $InstallerBytes[0] -eq 0xEF -and $InstallerBytes[1] -eq 0xBB -and $InstallerBytes[2] -eq 0xBF) 'install.ps1 must use UTF-8 BOM so Windows PowerShell 5.1 decodes Chinese text correctly'
$Tokens = $null
$ParseErrors = $null
$Ast = [System.Management.Automation.Language.Parser]::ParseFile(
  $InstallerPath,
  [ref]$Tokens,
  [ref]$ParseErrors
)
Assert-True ($ParseErrors.Count -eq 0) "install.ps1 contains PowerShell parse errors: $ParseErrors"

$RequiredFunctions = @(
  'Write-Info',
  'Get-UsableClientCommand',
  'Resolve-SystemNpmCmd',
  'Test-UsableSystemNode',
  'Ensure-NodeRuntime',
  'Get-NodeReleaseChecksum',
  'Download-VerifiedFileWithFallback',
  'Get-VerifiedSameSiteAsset',
  'Download-VerifiedAsset',
  'Detect-BrokenLocalProxy',
  'Invoke-NpmCommand',
  'Remove-ManagedPowerShellShims',
  'Test-NeedsNpmClientInstall'
)
foreach ($Name in $RequiredFunctions) {
  $FunctionAst = $Ast.FindAll({
      param($Node)
      $Node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
        $Node.Name -eq $Name
    }, $true) | Select-Object -First 1
  Assert-True ($null -ne $FunctionAst) "install.ps1 is missing function: $Name"
  Invoke-Expression $FunctionAst.Extent.Text
}

$OriginalExecutionPolicy = Get-ExecutionPolicy -Scope Process
$OriginalPath = $env:Path
$OriginalHttpProxy = $env:HTTP_PROXY
$OriginalHttpsProxy = $env:HTTPS_PROXY
$OriginalAllProxy = $env:ALL_PROXY
$FixtureDir = Join-Path ([IO.Path]::GetTempPath()) ("laoshirenai-auto-config-test-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $FixtureDir -Force | Out-Null

try {
  Set-ExecutionPolicy -Scope Process -ExecutionPolicy Restricted -Force
  Assert-True ((Get-ExecutionPolicy -Scope Process) -eq 'Restricted') 'Failed to enable Restricted execution policy for the regression test'

  $NodeCommand = Get-Command node -CommandType Application -ErrorAction Stop |
    Select-Object -First 1
  $ResolvedNpm = Resolve-SystemNpmCmd -NodeCommand $NodeCommand
  Assert-True ($ResolvedNpm.EndsWith('npm.cmd', [StringComparison]::OrdinalIgnoreCase)) "Unsafe npm shim selected: $ResolvedNpm"
  Assert-True (-not $ResolvedNpm.EndsWith('npm.ps1', [StringComparison]::OrdinalIgnoreCase)) "npm.ps1 must never be selected: $ResolvedNpm"

  $MinNodeMajor = 20
  $script:NodeExe = ''
  $script:NpmCmd = ''
  $script:UseProxylessNpm = $false
  Ensure-NodeRuntime
  Assert-True ($script:NpmCmd.EndsWith('npm.cmd', [StringComparison]::OrdinalIgnoreCase)) "Ensure-NodeRuntime selected an unsafe npm shim: $script:NpmCmd"
  Invoke-NpmCommand -Arguments @('--version')

  foreach ($Client in @('claude', 'codex')) {
    $CmdPath = Join-Path $FixtureDir "$Client.cmd"
    $Ps1Path = Join-Path $FixtureDir "$Client.ps1"
    Set-Content -LiteralPath $CmdPath -Encoding Ascii -Value "@echo off`r`necho $Client-test 1.0.0`r`nexit /b 0`r`n"
    Set-Content -LiteralPath $Ps1Path -Encoding Ascii -Value "throw '$Client.ps1 must not run'`r`n"
  }
  $GrokBinDir = Join-Path $FixtureDir 'grok-bin'
  New-Item -ItemType Directory -Path $GrokBinDir -Force | Out-Null
  Set-Content -LiteralPath (Join-Path $GrokBinDir 'grok.cmd') -Encoding Ascii -Value "@echo off`r`necho grok-test 1.0.0`r`nexit /b 0`r`n"
  Set-Content -LiteralPath (Join-Path $GrokBinDir 'grok.ps1') -Encoding Ascii -Value "throw 'grok.ps1 must not run'`r`n"
  Set-Content -LiteralPath (Join-Path $FixtureDir 'keep.ps1') -Encoding Ascii -Value "Write-Output 'no matching cmd'`r`n"
  $env:Path = "$FixtureDir;$GrokBinDir;$OriginalPath"

  foreach ($Client in @('claude', 'codex', 'grok')) {
    $ResolvedClient = Get-UsableClientCommand -CommandName $Client
    Assert-True ($ResolvedClient.EndsWith("$Client.cmd", [StringComparison]::OrdinalIgnoreCase)) "Unsafe $Client shim selected: $ResolvedClient"
  }

  # Reproduce the customer path: typing a bare command under Restricted resolves
  # the npm-generated .ps1 shim first. The installer must remove only managed
  # shims that have a same-name .cmd launcher, then the bare command must run.
  foreach ($Client in @('claude', 'codex')) {
    $UnsafeCommand = Get-Command $Client -ErrorAction Stop
    Assert-True ($UnsafeCommand.Path.EndsWith("$Client.ps1", [StringComparison]::OrdinalIgnoreCase)) "Fixture did not reproduce unsafe bare $Client resolution: $($UnsafeCommand.Path)"
    $PolicyBlocked = $false
    try {
      & $Client --version | Out-Null
    } catch {
      $PolicyBlocked = $_.Exception.GetType().Name -eq 'PSSecurityException'
    }
    Assert-True $PolicyBlocked "Bare $Client was not blocked by Restricted execution policy before repair"
  }

  $NpmPrefix = $FixtureDir
  $NodeCurrentDir = Join-Path $FixtureDir 'node-current'
  Remove-ManagedPowerShellShims
  foreach ($Client in @('claude', 'codex')) {
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $FixtureDir "$Client.ps1"))) "$Client.ps1 was not removed"
    $BareCommand = Get-Command $Client -ErrorAction Stop
    Assert-True ($BareCommand.Path.EndsWith("$Client.cmd", [StringComparison]::OrdinalIgnoreCase)) "Bare $Client still resolves to an unsafe launcher: $($BareCommand.Path)"
    & $Client --version | Out-Null
    Assert-True ($LASTEXITCODE -eq 0) "Bare $Client failed under Restricted execution policy"
  }
  Assert-True (Test-Path -LiteralPath (Join-Path $FixtureDir 'keep.ps1')) 'A managed PowerShell script without a matching .cmd must remain untouched'
  Assert-True (Test-Path -LiteralPath (Join-Path $GrokBinDir 'grok.ps1')) 'Unmanaged Grok fixture should remain untouched'

  $NpmLog = Join-Path $FixtureDir 'npm-arguments.log'
  $FakeNpm = Join-Path $FixtureDir 'npm.cmd'
  Set-Content -LiteralPath $FakeNpm -Encoding Ascii -Value "@echo off`r`necho ARGS=%* HTTP_PROXY=%HTTP_PROXY% HTTPS_PROXY=%HTTPS_PROXY% ALL_PROXY=%ALL_PROXY%>>$NpmLog`r`nexit /b 0`r`n"
  $script:NpmCmd = $FakeNpm

  $env:HTTP_PROXY = 'http://127.0.0.1:9'
  $env:HTTPS_PROXY = 'http://localhost:9'
  $env:ALL_PROXY = 'socks5://[::1]:9'
  Detect-BrokenLocalProxy
  Assert-True $script:UseProxylessNpm 'Dead local proxy variables were not detected'
  Invoke-NpmCommand -Arguments @('install', '-g', '@anthropic-ai/claude-code@latest')
  Invoke-NpmCommand -Arguments @('install', '-g', '@openai/codex@latest')
  $NpmCalls = Get-Content -LiteralPath $NpmLog -Raw
  Assert-True ($NpmCalls.Contains('@anthropic-ai/claude-code@latest')) 'Claude Code npm installation command was not executed through npm.cmd'
  Assert-True ($NpmCalls.Contains('@openai/codex@latest')) 'Codex npm installation command was not executed through npm.cmd'
  Assert-True (-not $NpmCalls.Contains('127.0.0.1:9')) 'Dead HTTP proxy leaked into npm.cmd'
  Assert-True (-not $NpmCalls.Contains('localhost:9')) 'Dead HTTPS proxy leaked into npm.cmd'
  Assert-True (-not $NpmCalls.Contains('[::1]:9')) 'Dead ALL_PROXY leaked into npm.cmd'
  Assert-True ($env:HTTP_PROXY -eq 'http://127.0.0.1:9') 'HTTP proxy was not restored after npm.cmd'
  Assert-True ($env:HTTPS_PROXY -eq 'http://localhost:9') 'HTTPS proxy was not restored after npm.cmd'
  Assert-True ($env:ALL_PROXY -eq 'socks5://[::1]:9') 'ALL_PROXY was not restored after npm.cmd'

  $env:HTTP_PROXY = ''
  $env:HTTPS_PROXY = ''
  $env:ALL_PROXY = ''
  Detect-BrokenLocalProxy
  Assert-True (-not $script:UseProxylessNpm) 'Proxyless environment was incorrectly classified as a broken proxy'

  $FailingNpm = Join-Path $FixtureDir 'npm-fail.cmd'
  Set-Content -LiteralPath $FailingNpm -Encoding Ascii -Value "@echo off`r`nexit /b 7`r`n"
  $script:NpmCmd = $FailingNpm
  $FailureObserved = $false
  try {
    Invoke-NpmCommand -Arguments @('--version')
  } catch {
    $FailureObserved = $_.Exception.Message.Contains('7')
  }
  Assert-True $FailureObserved 'A non-zero npm.cmd exit code was not converted into a retryable PowerShell error'

  $script:InstallClaudeClient = $true
  $script:InstallCodexClient = $false
  Assert-True (Test-NeedsNpmClientInstall) 'Claude Code must use the npm installation path'
  $script:InstallClaudeClient = $false
  $script:InstallCodexClient = $true
  Assert-True (Test-NeedsNpmClientInstall) 'Codex must use the npm installation path'
  $script:InstallClaudeClient = $false
  $script:InstallCodexClient = $false
  $script:InstallGrokClient = $true
  Assert-True (-not (Test-NeedsNpmClientInstall)) 'Grok Build must remain isolated from the npm installation path'

  $fixtureSHA256 = (Get-FileHash -LiteralPath (Join-Path $env:SystemRoot 'System32\whoami.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
  $nodeZipName = 'node-v24.0.0-win-x64.zip'
  $nodeFixtureRoot = Join-Path $FixtureDir 'node-fixture'
  $nodeVersionDir = Join-Path $nodeFixtureRoot 'v24.0.0'
  New-Item -ItemType Directory -Path $nodeVersionDir -Force | Out-Null
  $DefaultNodeDistPrimary = $nodeFixtureRoot
  $DefaultNodeDistFallback = $nodeFixtureRoot
  Set-Content -LiteralPath (Join-Path $nodeVersionDir 'SHASUMS256.txt') -Encoding Ascii -Value "$fixtureSHA256  $nodeZipName`n"
  $resolvedSHA256 = Get-NodeReleaseChecksum -Version 'v24.0.0' -ZipName $nodeZipName
  Assert-True ($resolvedSHA256 -eq $fixtureSHA256) 'Node.js checksum metadata was not resolved correctly'

  $verifiedDownload = Join-Path $FixtureDir 'verified-download.exe'
  $verifiedSource = Join-Path $FixtureDir 'verified-source.exe'
  Copy-Item -LiteralPath (Join-Path $env:SystemRoot 'System32\whoami.exe') -Destination $verifiedSource -Force
  Download-VerifiedFileWithFallback `
    -OutputPath $verifiedDownload `
    -Urls @($verifiedSource) `
    -ExpectedSHA256 $fixtureSHA256
  Assert-True (Test-Path -LiteralPath $verifiedDownload -PathType Leaf) 'Verified local fixture download was not retained'
  Remove-Item -LiteralPath $verifiedDownload -Force

  $checksumRejected = $false
  try {
    Download-VerifiedFileWithFallback `
      -OutputPath $verifiedDownload `
      -Urls @($verifiedSource) `
      -ExpectedSHA256 ('0' * 64)
  } catch {
    $checksumRejected = $_.Exception.Message.Contains('SHA256')
  }
  Assert-True $checksumRejected 'Node.js checksum mismatch did not fail closed'
  Assert-True (-not (Test-Path -LiteralPath $verifiedDownload)) 'Checksum failure left a partial file behind'

  $global:LASTEXITCODE = 0
  Write-Host "WINDOWS_AUTO_CONFIG_ACCEPTANCE_OK runtime=$($PSVersionTable.PSVersion) edition=$($PSVersionTable.PSEdition) claude=bare-cmd codex=bare-cmd grok=same-site git=same-site node=sha256 npm_policy=Restricted"
} finally {
  $env:Path = $OriginalPath
  $env:HTTP_PROXY = $OriginalHttpProxy
  $env:HTTPS_PROXY = $OriginalHttpsProxy
  $env:ALL_PROXY = $OriginalAllProxy
  Set-ExecutionPolicy -Scope Process -ExecutionPolicy $OriginalExecutionPolicy -Force
  Remove-Item -LiteralPath $FixtureDir -Recurse -Force -ErrorAction SilentlyContinue
}
