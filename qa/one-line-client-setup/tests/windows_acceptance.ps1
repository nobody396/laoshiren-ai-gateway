param(
  [Parameter(Mandatory = $true)]
  [string]$CommandPath
)

$ErrorActionPreference = 'Stop'
$command = [IO.File]::ReadAllText((Resolve-Path $CommandPath)).TrimEnd([char[]]"`r`n")
if ($command.Contains("`r") -or $command.Contains("`n")) {
  throw 'Rendered customer command is not one physical line.'
}

$script:readHostCalls = 0
$script:catalogMode = 'full'
$script:testKey = 'sk-owned-windows-qa-only'
$script:groupModels = @('gpt-5.4', 'gpt-5.5', 'gpt-5.6-sol', 'gpt-5.6-terra')

function global:Read-Host {
  param([string]$Prompt)
  $script:readHostCalls++
  return $script:testKey
}

function global:Invoke-RestMethod {
  param(
    [Parameter(Mandatory = $true)][string]$Uri,
    [string]$Method,
    [hashtable]$Headers
  )

  if ($Uri -eq 'https://api.laoshirenai.com/api/v1/public/model-pricing') {
    $models = @($script:groupModels | ForEach-Object { [pscustomobject]@{ model = $_ } })
    return [pscustomobject]@{
      data = [pscustomobject]@{
        groups = @([pscustomobject]@{ group_id = 58; name = 'GPT 经济线路'; models = $models })
      }
    }
  }

  if ($Uri -eq 'https://api.laoshirenai.com/v1/models') {
    if ($Headers.Authorization -ne "Bearer $($script:testKey)") {
      throw 'Authorization header did not contain the prompted Key.'
    }
    $visible = @($script:groupModels)
    if ($script:catalogMode -eq 'missing') {
      $visible = @($visible | Where-Object { $_ -ne 'gpt-5.6-sol' })
    }
    return [pscustomobject]@{
      data = @($visible | ForEach-Object { [pscustomobject]@{ id = $_ } })
    }
  }

  throw "Unexpected URI: $Uri"
}

Set-ExecutionPolicy -Scope Process -ExecutionPolicy Restricted -Force

$configDir = Join-Path $HOME '.workbuddy'
$configPath = Join-Path $configDir 'models.json'
if (Test-Path $configDir) {
  Remove-Item $configDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $configDir | Out-Null

$seed = [pscustomobject]@{
  models = @([pscustomobject]@{
    id = 'keep-me'
    name = 'Unrelated Model'
    vendor = 'Other'
    apiKey = 'unrelated-secret'
    url = 'https://example.invalid/v1/chat/completions'
  })
  availableModels = @('keep-me')
  unrelatedSetting = [pscustomobject]@{ enabled = $true }
}
[IO.File]::WriteAllText(
  $configPath,
  ($seed | ConvertTo-Json -Depth 20),
  [Text.UTF8Encoding]::new($false)
)

Invoke-Expression $command
if ($script:readHostCalls -ne 1) {
  throw "Expected one Key prompt, observed $($script:readHostCalls)."
}

$config = Get-Content $configPath -Raw | ConvertFrom-Json
$expectedIds = @('gpt-5.4', 'gpt-5.5', 'gpt-5.6-sol', 'gpt-5.6-terra', 'keep-me') | Sort-Object
$actualIds = @($config.models | ForEach-Object { [string]$_.id }) | Sort-Object
if (($actualIds -join '|') -ne ($expectedIds -join '|')) {
  throw "Wrong model IDs: $($actualIds -join ', ')"
}
if (-not $config.unrelatedSetting.enabled) {
  throw 'Unrelated configuration was not preserved.'
}
$imported = @($config.models | Where-Object { $_.id -ne 'keep-me' })
if (@($imported | Where-Object { $_.apiKey -ne $script:testKey }).Count -ne 0) {
  throw 'Imported models did not receive the prompted Key.'
}
if (@($imported | Where-Object { $_.url -ne 'https://api.laoshirenai.com/v1/chat/completions' }).Count -ne 0) {
  throw 'Imported models received the wrong endpoint.'
}
if (@($imported | Where-Object { $_.onlyReasoning -ne $false -or $_.useCustomProtocol -ne $false }).Count -ne 0) {
  throw 'Imported models received the wrong reasoning/protocol toggles.'
}
if (@($imported | Where-Object { $_.reasoning.defaultEffort -ne 'medium' -or $_.reasoning.canDisableThinking -ne $false }).Count -ne 0) {
  throw 'Imported models received the wrong reasoning defaults.'
}
if (@($imported | Where-Object { $_.maxInputTokens -ne 1050000 -or $_.maxOutputTokens -ne 128000 }).Count -ne 0) {
  throw 'Imported models received the wrong token limits.'
}
$sol = @($imported | Where-Object { $_.id -eq 'gpt-5.6-sol' })[0]
$gpt54 = @($imported | Where-Object { $_.id -eq 'gpt-5.4' })[0]
if (@($imported | Where-Object { $_.name -ne $_.id }).Count -ne 0) {
  throw 'Imported model display names are not concise model IDs.'
}
if (($sol.reasoning.supportedEfforts -join '|') -ne 'low|medium|high|xhigh|max') {
  throw 'gpt-5.6-sol reasoning levels are wrong.'
}
if (($gpt54.reasoning.supportedEfforts -join '|') -ne 'low|medium|high|xhigh') {
  throw 'gpt-5.4 reasoning levels are wrong.'
}
$expectedVisible = $expectedIds
$actualVisible = @($config.availableModels | ForEach-Object { [string]$_ }) | Sort-Object
if (($actualVisible -join '|') -ne ($expectedVisible -join '|')) {
  throw "Wrong availableModels: $($actualVisible -join ', ')"
}

$firstHash = (Get-FileHash $configPath -Algorithm SHA256).Hash
$firstBackupCount = @(Get-ChildItem "$configPath.bak-*" -ErrorAction SilentlyContinue).Count
Invoke-Expression $command
$secondHash = (Get-FileHash $configPath -Algorithm SHA256).Hash
$secondBackupCount = @(Get-ChildItem "$configPath.bak-*" -ErrorAction SilentlyContinue).Count
if ($firstHash -ne $secondHash) {
  throw 'Second execution changed the semantic configuration.'
}
if ($firstBackupCount -ne $secondBackupCount) {
  throw 'Second execution created a backup despite no configuration change.'
}

$beforeMissing = [IO.File]::ReadAllText($configPath)
$script:catalogMode = 'missing'
$missingFailed = $false
try {
  Invoke-Expression $command
} catch {
  $missingFailed = $true
}
if (-not $missingFailed) {
  throw 'A Key missing one group model did not fail closed.'
}
if ([IO.File]::ReadAllText($configPath) -ne $beforeMissing) {
  throw 'Authorization mismatch changed the existing configuration.'
}

$script:catalogMode = 'full'
[IO.File]::WriteAllText($configPath, '{not-json', [Text.UTF8Encoding]::new($false))
$malformedBefore = [IO.File]::ReadAllText($configPath)
$malformedFailed = $false
try {
  Invoke-Expression $command
} catch {
  $malformedFailed = $true
}
if (-not $malformedFailed) {
  throw 'Malformed existing JSON did not fail closed.'
}
if ([IO.File]::ReadAllText($configPath) -ne $malformedBefore) {
  throw 'Malformed existing JSON was modified.'
}
if (Test-Path "$configPath.tmp") {
  throw 'Failure left a temporary file behind.'
}

[IO.File]::WriteAllText($configPath, '', [Text.UTF8Encoding]::new($false))
Invoke-Expression $command
$arrayConfig = Get-Content $configPath -Raw | ConvertFrom-Json
$arrayIds = @($arrayConfig | ForEach-Object { [string]$_.id }) | Sort-Object
if (($arrayIds -join '|') -ne (($script:groupModels | Sort-Object) -join '|')) {
  throw "GUI-created empty file was not recovered as a model array: $($arrayIds -join ', ')"
}

Write-Output "WINDOWS_ACCEPTANCE_OK shell=$($PSVersionTable.PSEdition) version=$($PSVersionTable.PSVersion)"
