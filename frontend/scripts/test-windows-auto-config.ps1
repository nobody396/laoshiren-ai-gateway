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
  'Invoke-NpmCommand',
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

  foreach ($Client in @('claude', 'codex', 'grok')) {
    $CmdPath = Join-Path $FixtureDir "$Client.cmd"
    $Ps1Path = Join-Path $FixtureDir "$Client.ps1"
    Set-Content -LiteralPath $CmdPath -Encoding Ascii -Value "@echo off`r`necho $Client-test 1.0.0`r`nexit /b 0`r`n"
    Set-Content -LiteralPath $Ps1Path -Encoding Ascii -Value "throw '$Client.ps1 must not run'`r`n"
  }
  $env:Path = "$FixtureDir;$OriginalPath"

  foreach ($Client in @('claude', 'codex', 'grok')) {
    $ResolvedClient = Get-UsableClientCommand -CommandName $Client
    Assert-True ($ResolvedClient.EndsWith("$Client.cmd", [StringComparison]::OrdinalIgnoreCase)) "Unsafe $Client shim selected: $ResolvedClient"
  }

  $NpmLog = Join-Path $FixtureDir 'npm-arguments.log'
  $FakeNpm = Join-Path $FixtureDir 'npm.cmd'
  Set-Content -LiteralPath $FakeNpm -Encoding Ascii -Value "@echo off`r`necho %*>>$NpmLog`r`nexit /b 0`r`n"
  $script:NpmCmd = $FakeNpm
  Invoke-NpmCommand -Arguments @('install', '-g', '@anthropic-ai/claude-code@latest')
  Invoke-NpmCommand -Arguments @('install', '-g', '@openai/codex@latest')
  $NpmCalls = Get-Content -LiteralPath $NpmLog -Raw
  Assert-True ($NpmCalls.Contains('@anthropic-ai/claude-code@latest')) 'Claude Code npm installation command was not executed through npm.cmd'
  Assert-True ($NpmCalls.Contains('@openai/codex@latest')) 'Codex npm installation command was not executed through npm.cmd'

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

  Write-Host 'WINDOWS_AUTO_CONFIG_ACCEPTANCE_OK claude=cmd codex=cmd grok=native npm_policy=Restricted'
} finally {
  $env:Path = $OriginalPath
  Set-ExecutionPolicy -Scope Process -ExecutionPolicy $OriginalExecutionPolicy -Force
  Remove-Item -LiteralPath $FixtureDir -Recurse -Force -ErrorAction SilentlyContinue
}
