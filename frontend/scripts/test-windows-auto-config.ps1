Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$script:SetupPlan = $null
$script:SetupClientVersion = ''

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
  'Apply-SetupPlan',
  'Test-SetupPlanClientVersion',
  'Get-ClientVersion',
  'Write-Info',
  'Write-WarnMessage',
  'Stop-Script',
  'Ensure-Directory',
  'Backup-IfNeeded',
  'ConvertTo-TomlString',
  'Get-OpenAIV1BaseUrl',
  'Write-GrokTomlConfig',
  'Write-GeminiConfig',
  'Invoke-GrokCcSwitchImporter',
  'Get-UsableClientCommand',
  'Resolve-SystemNpmCmd',
  'Test-UsableSystemNode',
  'Ensure-NodeRuntime',
  'Convert-CodexModelCatalog',
  'Write-CodexModelCatalog',
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
  $script:GrokCcSwitchCompat = $false
  $script:Tools = 'all'
  $script:NodeExe = ''
  $script:NpmCmd = ''
  $script:UseProxylessNpm = $false
  Ensure-NodeRuntime
  Assert-True ($script:NpmCmd.EndsWith('npm.cmd', [StringComparison]::OrdinalIgnoreCase)) "Ensure-NodeRuntime selected an unsafe npm shim: $script:NpmCmd"
  Invoke-NpmCommand -Arguments @('--version')

  $CodexCatalogSource = Join-Path $PSScriptRoot '..\public\auto-config\codex-model-catalog.json'
  $CodexCatalogOutput = Join-Path $FixtureDir 'cyber-codex-model-catalog.json'
  $script:CatalogOpenAIDefaultModel = 'stale-default'
  Convert-CodexModelCatalog `
    -SourcePath $CodexCatalogSource `
    -AuthorizedModels @('gpt-5.6-sol', 'gpt-daybreak-blue-latest') `
    -OutputPath $CodexCatalogOutput
  $CyberCatalog = Get-Content -LiteralPath $CodexCatalogOutput -Raw | ConvertFrom-Json
  $CyberModelIds = @($CyberCatalog.models | ForEach-Object { [string]$_.slug })
  Assert-True (($CyberModelIds -join ',') -eq 'gpt-5.6-sol,gpt-daybreak-blue-latest') "Cyber Codex model catalog mismatch: $($CyberModelIds -join ',')"
  Assert-True ($script:CatalogOpenAIDefaultModel -eq 'gpt-5.6-sol') 'Cyber Codex default model was not set to Sol'
  Assert-True (-not ($CyberModelIds -contains 'gpt-5.6-terra')) 'Unsupported Terra leaked into the Cyber Codex catalog'
  $DaybreakModel = @($CyberCatalog.models | Where-Object { $_.slug -eq 'gpt-daybreak-blue-latest' })[0]
  Assert-True ($DaybreakModel.display_name -eq 'GPT Daybreak Blue Latest') 'Daybreak display name was not synthesized'
  Assert-True (-not [string]::IsNullOrWhiteSpace([string]$DaybreakModel.base_instructions)) 'Daybreak is missing base instructions'
  Assert-True ($DaybreakModel.visibility -eq 'list') 'Daybreak is not visible in the Codex model selector'
  Assert-True ([int]$DaybreakModel.context_window -eq 1050000) 'Daybreak context window is incorrect'

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

  $GrokDir = Join-Path $FixtureDir 'grok-home'
  $GrokConfigPath = Join-Path $GrokDir 'config.toml'
  New-Item -ItemType Directory -Path $GrokDir -Force | Out-Null
  $OriginalGrokConfig = @'
[models]
default = "unrelated"

[preferences]
theme = "dark"

[model."unrelated"]
model = "unrelated"
base_url = "https://unrelated.example/v1"
api_key = "keep-me"

[model."grok-4.5"]
name = "stale"
'@
  [IO.File]::WriteAllText($GrokConfigPath, $OriginalGrokConfig, [Text.UTF8Encoding]::new($false))
  $CatalogGrokDefaultModel = 'grok-4.6'
  $CatalogGrokManagedModels = @(
    @{ Id = 'grok-4.5'; DisplayName = 'Grok 4.5'; ContextWindow = 500000 },
    @{ Id = 'grok-4.6'; DisplayName = 'Grok 4.6'; ContextWindow = 500000 }
  )
  $CatalogGrokManagedModelSections = @('model.grok-4.5', 'model."grok-4.5"', 'model.grok-4.6', 'model."grok-4.6"')
  $script:BaseUrl = 'https://api.example.com'
  $script:GrokApiKey = 'test-owned-key'

  Write-GrokTomlConfig
  $FirstGrokConfig = [IO.File]::ReadAllText($GrokConfigPath)
  $NormalizedGrokConfig = $FirstGrokConfig.Replace("`r`n", "`n")
  Assert-True ($NormalizedGrokConfig.Contains('default = "grok-4.6"')) 'Grok 4.6 was not selected as the default'
  Assert-True ($NormalizedGrokConfig.Contains("[preferences]`ntheme = `"dark`"")) 'Unrelated Grok preferences were overwritten'
  Assert-True ($NormalizedGrokConfig.Contains('[model."unrelated"]')) 'Unrelated Grok provider was overwritten'
  Assert-True ($NormalizedGrokConfig.Contains('api_key = "keep-me"')) 'Unrelated Grok credential was overwritten'
  Assert-True (([regex]::Matches($NormalizedGrokConfig, [regex]::Escape('[model."grok-4.5"]'))).Count -eq 1) 'Grok 4.5 was not written exactly once'
  Assert-True (([regex]::Matches($NormalizedGrokConfig, [regex]::Escape('[model."grok-4.6"]'))).Count -eq 1) 'Grok 4.6 was not written exactly once'
  Assert-True ($NormalizedGrokConfig.Contains('name = "Grok 4.5"')) 'Grok 4.5 display name is missing'
  Assert-True ($NormalizedGrokConfig.Contains('name = "Grok 4.6"')) 'Grok 4.6 display name is missing'
  Assert-True (-not $NormalizedGrokConfig.Contains('老实人AI')) 'Provider/group branding leaked into model names'
  Assert-True ([IO.File]::ReadAllText("$GrokConfigPath.bak") -eq $OriginalGrokConfig) 'The original Grok backup was not preserved'
  Assert-True (-not (Get-ChildItem -LiteralPath $GrokDir -Filter 'config.toml.tmp.*')) 'Atomic Grok write left temporary files behind'

  Write-GrokTomlConfig
  Assert-True ([IO.File]::ReadAllText($GrokConfigPath) -eq $FirstGrokConfig) 'Grok config repair is not idempotent'
  Assert-True ([IO.File]::ReadAllText("$GrokConfigPath.bak") -eq $OriginalGrokConfig) 'A retry replaced the original Grok backup'

  $GeminiDir = Join-Path $FixtureDir 'gemini-home'
  $GeminiEnvPath = Join-Path $GeminiDir '.env'
  $GeminiSettingsPath = Join-Path $GeminiDir 'settings.json'
  New-Item -ItemType Directory -Path $GeminiDir -Force | Out-Null
  $OriginalGeminiEnv = "GEMINI_API_KEY=old-key`nOTHER_KEY=keep-me`nGOOGLE_GENAI_USE_VERTEXAI=true`n"
  [IO.File]::WriteAllText($GeminiEnvPath, $OriginalGeminiEnv, [Text.UTF8Encoding]::new($false))
  $OriginalGeminiSettings = @'
{
  "security": { "auth": { "selectedType": "oauth-personal", "other": 1 } },
  "model": { "name": "old-model", "extra": true },
  "modelConfigs": {
    "overrides": [
      { "match": { "model": "user-model" }, "generateContentConfig": { "temperature": 0.1 } },
      { "match": { "model": "gemini-3.7-flash" }, "generateContentConfig": { "thinkingConfig": { "thinkingLevel": "LOW" } } }
    ]
  },
  "theme": "dark"
}
'@
  [IO.File]::WriteAllText($GeminiSettingsPath, $OriginalGeminiSettings, [Text.UTF8Encoding]::new($false))
  $CatalogGeminiDefaultModel = 'gemini-3.7-flash'
  $CatalogGeminiManagedModels = @('gemini-3.1-pro', 'gemini-3.7-flash')
  $script:BaseUrl = 'https://api.example.com'
  $script:GeminiApiKey = 'test-owned-key'

  Write-GeminiConfig
  $FirstGeminiEnv = [IO.File]::ReadAllText($GeminiEnvPath)
  $NormalizedGeminiEnv = $FirstGeminiEnv.Replace("`r`n", "`n")
  Assert-True ($NormalizedGeminiEnv.Contains('GEMINI_API_KEY=test-owned-key')) 'Gemini API key was not written to .env'
  Assert-True ($NormalizedGeminiEnv.Contains('OTHER_KEY=keep-me')) 'Unrelated Gemini .env entry was overwritten'
  Assert-True ($NormalizedGeminiEnv.Contains('GOOGLE_GEMINI_BASE_URL=https://api.example.com')) 'Gemini base URL was not written to .env'
  Assert-True ($NormalizedGeminiEnv.Contains('GOOGLE_GENAI_USE_VERTEXAI=false')) 'Gemini VertexAI flag was not forced off'
  Assert-True ($NormalizedGeminiEnv.Contains('GEMINI_MODEL=gemini-3.7-flash')) 'Gemini default model was not written to .env'
  Assert-True (-not $NormalizedGeminiEnv.Contains('old-key')) 'Stale Gemini API key survived the .env merge'
  Assert-True ([IO.File]::ReadAllText("$GeminiEnvPath.bak") -eq $OriginalGeminiEnv) 'The original Gemini .env backup was not preserved'

  $FirstGeminiSettings = [IO.File]::ReadAllText($GeminiSettingsPath)
  $ParsedGeminiSettings = $FirstGeminiSettings | ConvertFrom-Json
  Assert-True ($ParsedGeminiSettings.security.auth.selectedType -eq 'gemini-api-key') 'Gemini auth type was not switched to gemini-api-key'
  Assert-True ($ParsedGeminiSettings.security.auth.other -eq 1) 'Unrelated Gemini security field was overwritten'
  Assert-True ($ParsedGeminiSettings.model.name -eq 'gemini-3.7-flash') 'Gemini default model was not selected in settings.json'
  Assert-True ($ParsedGeminiSettings.model.extra -eq $true) 'Unrelated Gemini model field was overwritten'
  Assert-True ($ParsedGeminiSettings.theme -eq 'dark') 'Unrelated Gemini setting was overwritten'
  $GeminiOverrideModels = @($ParsedGeminiSettings.modelConfigs.overrides | ForEach-Object { [string]$_.match.model })
  Assert-True (($GeminiOverrideModels -join ',') -eq 'user-model,gemini-3.1-pro,gemini-3.7-flash') "Gemini overrides mismatch: $($GeminiOverrideModels -join ',')"
  $UserGeminiOverride = @($ParsedGeminiSettings.modelConfigs.overrides | Where-Object { $_.match.model -eq 'user-model' })[0]
  Assert-True ($UserGeminiOverride.generateContentConfig.temperature -eq 0.1) 'User-owned Gemini override was overwritten'
  foreach ($ManagedGeminiOverride in @($ParsedGeminiSettings.modelConfigs.overrides | Where-Object { $_.match.model -ne 'user-model' })) {
    Assert-True ($ManagedGeminiOverride.generateContentConfig.thinkingConfig.thinkingLevel -eq 'HIGH') 'Managed Gemini override is missing thinkingLevel=HIGH'
  }
  Assert-True (-not $FirstGeminiSettings.Contains('老实人AI')) 'Provider/group branding leaked into Gemini settings'
  Assert-True ([IO.File]::ReadAllText("$GeminiSettingsPath.bak") -eq $OriginalGeminiSettings) 'The original Gemini settings backup was not preserved'
  Assert-True (-not (Get-ChildItem -LiteralPath $GeminiDir -Filter '*.tmp.*')) 'Gemini config write left temporary files behind'

  Write-GeminiConfig
  Assert-True ([IO.File]::ReadAllText($GeminiEnvPath) -eq $FirstGeminiEnv) 'Gemini .env repair is not idempotent'
  Assert-True ([IO.File]::ReadAllText($GeminiSettingsPath) -eq $FirstGeminiSettings) 'Gemini settings repair is not idempotent'
  Assert-True ([IO.File]::ReadAllText("$GeminiEnvPath.bak") -eq $OriginalGeminiEnv) 'A retry replaced the original Gemini .env backup'
  Assert-True ([IO.File]::ReadAllText("$GeminiSettingsPath.bak") -eq $OriginalGeminiSettings) 'A retry replaced the original Gemini settings backup'

  $CcSwitchDir = Join-Path $FixtureDir 'cc-switch-home'
  $CcSwitchDb = Join-Path $CcSwitchDir 'cc-switch.db'
  $CcSwitchSettings = Join-Path $CcSwitchDir 'settings.json'
  New-Item -ItemType Directory -Path $CcSwitchDir -Force | Out-Null
  [IO.File]::WriteAllText($CcSwitchSettings, '{"currentProviderGrokbuild":"existing-grok","keep":true}', [Text.UTF8Encoding]::new($false))
  $SeedDbScript = Join-Path $FixtureDir 'seed-cc-switch.cjs'
  [IO.File]::WriteAllText($SeedDbScript, @'
const { DatabaseSync } = require('node:sqlite')
const db = new DatabaseSync(process.env.CC_SWITCH_DB)
db.exec(`
  CREATE TABLE providers (
    id TEXT NOT NULL, app_type TEXT NOT NULL, name TEXT NOT NULL,
    settings_config TEXT NOT NULL, website_url TEXT, category TEXT,
    created_at INTEGER, sort_index INTEGER, notes TEXT, icon TEXT, icon_color TEXT,
    meta TEXT NOT NULL DEFAULT '{}', is_current BOOLEAN NOT NULL DEFAULT 0,
    in_failover_queue BOOLEAN NOT NULL DEFAULT 0,
    cost_multiplier TEXT NOT NULL DEFAULT '1.0', limit_daily_usd TEXT,
    limit_monthly_usd TEXT, provider_type TEXT, PRIMARY KEY (id, app_type)
  );
`)
const insert = db.prepare(`
  INSERT INTO providers (id, app_type, name, settings_config, created_at, sort_index, meta, is_current)
  VALUES (?, ?, ?, ?, ?, ?, '{}', ?)
`)
const sentinel = JSON.stringify({ config: 'unrelated-secret-sentinel' })
insert.run('existing-grok', 'grokbuild', 'Existing Grok', sentinel, 1, 0, 1)
insert.run('existing-claude', 'claude', 'Existing Claude', sentinel, 2, 0, 1)
db.close()
'@, [Text.UTF8Encoding]::new($false))
  $PreviousCcSwitchDb = $env:CC_SWITCH_DB
  $PreviousCcSwitchSettings = $env:CC_SWITCH_SETTINGS_PATH
  try {
    $env:CC_SWITCH_DB = $CcSwitchDb
    $env:CC_SWITCH_SETTINGS_PATH = $CcSwitchSettings
    & $script:NodeExe --no-warnings $SeedDbScript
    Assert-True ($LASTEXITCODE -eq 0) 'Failed to create the Windows CC Switch database fixture'

    $script:BaseUrl = 'https://api.example.com'
    $script:GrokApiKey = 'test-owned-key'
    $ImporterPath = Join-Path $PSScriptRoot '..\public\auto-config\import-grok-cc-switch-provider.cjs'
    Invoke-GrokCcSwitchImporter -ImporterPath $ImporterPath
    Invoke-GrokCcSwitchImporter -ImporterPath $ImporterPath

    $VerifyDbScript = Join-Path $FixtureDir 'verify-cc-switch.cjs'
    [IO.File]::WriteAllText($VerifyDbScript, @'
const fs = require('node:fs')
const path = require('node:path')
const { DatabaseSync } = require('node:sqlite')
const db = new DatabaseSync(process.env.CC_SWITCH_DB)
const rows = db.prepare('SELECT id, app_type, name, settings_config, is_current FROM providers ORDER BY app_type, id').all()
db.close()
if (rows.length !== 3) throw new Error(`unexpected provider count: ${rows.length}`)
const existingGrok = rows.find((row) => row.id === 'existing-grok')
const existingClaude = rows.find((row) => row.id === 'existing-claude')
const imported = rows.find((row) => row.id === 'laoshirenai-grok-group')
if (JSON.parse(existingGrok.settings_config).config !== 'unrelated-secret-sentinel') throw new Error('existing Grok provider changed')
if (JSON.parse(existingClaude.settings_config).config !== 'unrelated-secret-sentinel') throw new Error('existing Claude provider changed')
if (Number(existingGrok.is_current) !== 0 || Number(existingClaude.is_current) !== 1) throw new Error('unrelated current state changed')
if (!imported || imported.name !== 'Grok 分组' || Number(imported.is_current) !== 1) throw new Error('neutral Grok provider missing')
const config = JSON.parse(imported.settings_config).config
for (const expected of ['default = "grok-4.6"', '[model."grok-4.5"]', '[model."grok-4.6"]', 'name = "Grok 4.5"', 'description = "Grok 4.6"']) {
  if (!config.includes(expected)) throw new Error(`missing config: ${expected}`)
}
if (config.includes('unrelated-secret-sentinel')) throw new Error('unrelated config leaked into imported provider')
const settings = JSON.parse(fs.readFileSync(process.env.CC_SWITCH_SETTINGS_PATH, 'utf8'))
if (settings.currentProviderGrokbuild !== 'laoshirenai-grok-group' || settings.keep !== true) throw new Error('settings were not preserved')
const backupRoot = path.join(path.dirname(process.env.CC_SWITCH_DB), 'backups', 'laoshirenai-grok')
if (fs.readdirSync(backupRoot).length !== 1) throw new Error('idempotent retry created another backup')
'@, [Text.UTF8Encoding]::new($false))
    & $script:NodeExe --no-warnings $VerifyDbScript
    Assert-True ($LASTEXITCODE -eq 0) 'Windows CC Switch Provider import regression failed'
  } finally {
    $env:CC_SWITCH_DB = $PreviousCcSwitchDb
    $env:CC_SWITCH_SETTINGS_PATH = $PreviousCcSwitchSettings
  }

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

  $script:InstallGeminiClient = $false
  $script:InstallClaudeClient = $true
  $script:InstallCodexClient = $false
  Assert-True (Test-NeedsNpmClientInstall) 'Claude Code must use the npm installation path'
  $script:InstallClaudeClient = $false
  $script:InstallCodexClient = $true
  Assert-True (Test-NeedsNpmClientInstall) 'Codex must use the npm installation path'
  $script:InstallCodexClient = $false
  $script:InstallGeminiClient = $true
  Assert-True (Test-NeedsNpmClientInstall) 'Gemini CLI must use the npm installation path'
  $script:InstallGeminiClient = $false
  $script:InstallGrokClient = $true
  Assert-True (-not (Test-NeedsNpmClientInstall)) 'Grok Build must remain isolated from the npm installation path'
  $script:InstallGrokClient = $false

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
    $checksumRejected = $true
  }
  Assert-True $checksumRejected 'Node.js checksum mismatch did not fail closed'
  Assert-True (-not [System.IO.File]::Exists($verifiedDownload)) 'Checksum failure left a partial file behind'

  # New multi-group plan: no real credentials or network are used here.
  $script:Tools = 'grok'
  $script:BaseUrl = 'https://api.laoshirenai.com'
  $script:GrokApiKey = 'fixture-plan-key'
  $script:CatalogGrokDefaultModel = 'gpt-5.4'
  $script:GrokDir = Join-Path $FixtureDir 'plan-grok'
  $script:GrokConfigPath = Join-Path $script:GrokDir 'config.toml'
  Ensure-Directory $script:GrokDir
  [IO.File]::WriteAllText($script:GrokConfigPath, "[model.`"user-model`"]`nmodel=`"keep`"`n[model.`"laoshirenai/retired`"]`nmodel=`"retired`"`n")
  $Plan = [pscustomobject]@{ available = $true; target = 'grok'; os = 'windows'; client_version_key = 'cli:1.0.13'; default_model = 'gpt-5.4'; models = @([pscustomobject]@{ id = 'gpt-5.4'; protocol = 'responses' }, [pscustomobject]@{ id = 'claude-opus-5'; protocol = 'messages' }) }
  Apply-SetupPlan -Plan $Plan
  Write-GrokTomlConfig
  $FirstPlanConfig = [IO.File]::ReadAllText($script:GrokConfigPath)
  Assert-True ($FirstPlanConfig.Contains('[model."laoshirenai/gpt-5.4"]')) 'plan omitted Responses model'
  Assert-True ($FirstPlanConfig.Contains('[model."laoshirenai/claude-opus-5"]')) 'plan omitted Messages model'
  Assert-True ($FirstPlanConfig.Contains('api_backend = "messages"')) 'plan lost per-model protocol'
  Assert-True ($FirstPlanConfig.Contains('model="keep"')) 'plan overwrote unrelated model'
  Assert-True (-not $FirstPlanConfig.Contains('retired')) 'plan retained revoked managed model'
  Write-GrokTomlConfig
  Assert-True ([IO.File]::ReadAllText($script:GrokConfigPath) -ceq $FirstPlanConfig) 'plan rewrite is not idempotent'
  $Plan.os = 'macos'
  $RejectedOS = $false
  try { Apply-SetupPlan -Plan $Plan } catch { $RejectedOS = $true }
  Assert-True $RejectedOS 'Windows accepted a different OS plan'
  $Plan.os = 'windows'
  $Plan.models[0].id = '$(unsafe)'
  $RejectedID = $false
  try { Apply-SetupPlan -Plan $Plan } catch { $RejectedID = $true }
  Assert-True $RejectedID 'plan accepted an unsafe model ID'
  $script:SetupPlan = $null
  $script:SetupClientVersion = ''

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
