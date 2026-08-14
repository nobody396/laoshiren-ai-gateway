Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Assert-True {
  param([bool]$Condition, [string]$Message)
  if (-not $Condition) { throw $Message }
}


function Get-FunctionsFromPowerShellFile {
  param(
    [string]$Path,
    [string[]]$Names
  )

  $Tokens = $null
  $ParseErrors = $null
  $Ast = [System.Management.Automation.Language.Parser]::ParseFile(
    $Path,
    [ref]$Tokens,
    [ref]$ParseErrors
  )
  if ($ParseErrors.Count -ne 0) {
    throw "$Path contains PowerShell parse errors: $ParseErrors"
  }
  foreach ($Name in $Names) {
    $FunctionAst = $Ast.FindAll({
        param($Node)
        $Node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
          $Node.Name -eq $Name
      }, $true) | Select-Object -First 1
    if ($null -eq $FunctionAst) {
      throw "$Path is missing function: $Name"
    }
    $FunctionAst.Extent.Text
  }
}

function Invoke-ResourceInstallContract {
  param(
    [string[]]$URLs,
    [string]$ExpectedSHA256,
    [string]$Destination
  )

  $ok = $false
  try {
    foreach ($url in $URLs) {
      Remove-Item $Destination -Force -ErrorAction SilentlyContinue
      & curl.exe -fL --retry 2 --retry-delay 1 --connect-timeout 60 --max-time 30 -o $Destination $url
      if ($LASTEXITCODE -eq 0 -and (Test-Path -LiteralPath $Destination -PathType Leaf)) {
        $hash = (Get-FileHash -LiteralPath $Destination -Algorithm SHA256).Hash
        if ($hash -eq $ExpectedSHA256) {
          $ok = $true
          break
        }
      }
    }
    if (-not $ok) {
      throw 'download failed or checksum mismatch'
    }
    $process = Start-Process -FilePath $Destination -Wait -PassThru
    Assert-True ($process.ExitCode -eq 0) "fixture installer exited with $($process.ExitCode)"
  } finally {
    Remove-Item $Destination -Force -ErrorAction SilentlyContinue
  }
}

$originalExecutionPolicy = Get-ExecutionPolicy -Scope Process
$fixtureDir = Join-Path ([IO.Path]::GetTempPath()) ("laoshirenai resource install test with spaces-" + [guid]::NewGuid().ToString('N'))
$server = $null
$originalArchitecture = $env:PROCESSOR_ARCHITECTURE
$originalArchitectureW6432 = $env:PROCESSOR_ARCHITEW6432
New-Item -ItemType Directory -Path $fixtureDir -Force | Out-Null

try {
  foreach ($scriptPath in @(
    (Join-Path $PSScriptRoot '..\public\auto-config\install.ps1'),
    (Join-Path $PSScriptRoot '..\public\auto-config\diagnose-cc-switch.ps1'),
    (Join-Path $PSScriptRoot '..\public\auto-config\save-openai-official-provider.ps1')
  )) {
    $scriptBytes = [IO.File]::ReadAllBytes($scriptPath)
    Assert-True ($scriptBytes.Length -ge 3 -and $scriptBytes[0] -eq 0xEF -and $scriptBytes[1] -eq 0xBB -and $scriptBytes[2] -eq 0xBF) "$scriptPath must use UTF-8 BOM for Windows PowerShell 5.1"
  }
  Set-ExecutionPolicy -Scope Process -ExecutionPolicy Restricted -Force
  Assert-True ((Get-ExecutionPolicy -Scope Process) -eq 'Restricted') 'failed to enable Restricted execution policy'

  $fixtureExe = Join-Path $fixtureDir 'fixture-installer.exe'
  Copy-Item -LiteralPath (Join-Path $env:SystemRoot 'System32\whoami.exe') -Destination $fixtureExe -Force
  $expectedSHA256 = (Get-FileHash -LiteralPath $fixtureExe -Algorithm SHA256).Hash

  # The official-provider helper is another customer-facing Windows command.
  # Parse and execute its Python discovery function under Restricted so a
  # future PowerShell encoding or syntax regression cannot bypass this gate.
  $officialProviderPath = Join-Path $PSScriptRoot '..\public\auto-config\save-openai-official-provider.ps1'
  $officialProviderFunctions = Get-FunctionsFromPowerShellFile -Path $officialProviderPath -Names @('Get-PythonRunner')
  Invoke-Expression ($officialProviderFunctions -join "`n")
  Assert-True ($null -ne (Get-PythonRunner)) 'Official-provider helper could not resolve Python 3 on the Windows runner'
  $portFile = Join-Path $fixtureDir 'server-port.txt'
  $requestLog = Join-Path $fixtureDir 'requests.log'
  $serverScript = Join-Path $fixtureDir 'server.mjs'
  Set-Content -LiteralPath $serverScript -Encoding Ascii -Value @'
import { createServer } from 'node:http';
import { appendFileSync, readFileSync, writeFileSync } from 'node:fs';
const fixture = process.env.RESOURCE_FIXTURE_PATH;
const portFile = process.env.RESOURCE_PORT_FILE;
const requestLog = process.env.RESOURCE_REQUEST_LOG;
const expectedSHA256 = process.env.RESOURCE_FIXTURE_SHA256;
const payload = readFileSync(fixture);
const server = createServer((request, response) => {
  appendFileSync(requestLog, `${request.url}\n`);
  const base = `http://127.0.0.1:${server.address().port}`;
  const manifests = {
    '/git-manifest': {
      tool: 'git-for-windows', version: 'v2.51.0.windows.1', assets: [{
        name: 'Git-2.51.0-64-bit.exe', platform: 'windows', arch: 'x64', sha256: expectedSHA256,
        download_url: `${base}/downloads/git-for-windows/v2.51.0.windows.1/${expectedSHA256}/git-2.51.0-64-bit.exe`
      }]
    },
    '/grok-manifest': {
      tool: 'grok-build', version: '1.0.3', assets: [{
        name: 'grok-1.0.3-windows-x86_64.exe', platform: 'windows', arch: 'x64', sha256: expectedSHA256,
        download_url: `${base}/downloads/grok-build/1.0.3/${expectedSHA256}/grok-1.0.3-windows-x86_64.exe`
      }]
    },
    '/codex-manifest': {
      tool: 'codex', version: 'codex-app-26.803.81509', assets: [{
        name: 'OpenAI.Codex_26.803.81509.0_x64__2p2nqsd0c76g0.Msix', platform: 'windows', arch: 'x64', sha256: expectedSHA256,
        download_url: `${base}/downloads/codex/codex-app-26.803.81509/${expectedSHA256}/openai.codex_26.803.81509.0_x64_2p2nqsd0c76g0.msix`
      }]
    },
    '/cc-manifest': {
      tool: 'cc-switch', version: 'v3.19.2', assets: [{
        name: 'CC-Switch-v3.19.2-Windows.msi', platform: 'windows', arch: 'universal', sha256: expectedSHA256,
        download_url: `${base}/downloads/cc-switch/v3.19.2/${expectedSHA256}/cc-switch-v3.19.2-windows.msi`
      }]
    }
  };
  if (Object.hasOwn(manifests, request.url)) {
    const body = Buffer.from(JSON.stringify(manifests[request.url]));
    response.writeHead(200, { 'Content-Type': 'application/json', 'Content-Length': body.length });
    response.end(body);
    return;
  }
  if (request.url === '/ok' || request.url.startsWith('/downloads/')) {
    response.writeHead(200, { 'Content-Type': 'application/octet-stream', 'Content-Length': payload.length });
    response.end(payload);
    return;
  }
  response.writeHead(503, { 'Content-Type': 'text/plain' });
  response.end('temporary failure');
});
server.listen(0, '127.0.0.1', () => writeFileSync(portFile, String(server.address().port)));
process.on('SIGTERM', () => server.close(() => process.exit(0)));
'@
  $env:RESOURCE_FIXTURE_PATH = $fixtureExe
  $env:RESOURCE_PORT_FILE = $portFile
  $env:RESOURCE_REQUEST_LOG = $requestLog
  $env:RESOURCE_FIXTURE_SHA256 = $expectedSHA256.ToLowerInvariant()
  $server = Start-Process -FilePath 'node.exe' -ArgumentList @('"' + $serverScript + '"') -WindowStyle Hidden -PassThru

  for ($attempt = 0; $attempt -lt 50 -and -not (Test-Path -LiteralPath $portFile); $attempt++) {
    Start-Sleep -Milliseconds 100
  }
  Assert-True (Test-Path -LiteralPath $portFile) 'local fixture server did not start'
  $port = (Get-Content -LiteralPath $portFile -Raw).Trim()
  $destination = Join-Path $fixtureDir 'Claude Setup 下载.exe'


  $installerPath = Join-Path $PSScriptRoot '..\public\auto-config\install.ps1'
  $installerFunctions = Get-FunctionsFromPowerShellFile -Path $installerPath -Names @(
    'Write-Info',
    'Stop-Script',
    'Ensure-Directory',
    'Get-VerifiedSameSiteAsset',
    'Download-VerifiedAsset',
    'Install-Git',
    'Install-GrokBuild',
    'Install-CodexAppIfRequested'
  )
  Invoke-Expression ($installerFunctions -join "`n")

  $diagnosticPath = Join-Path $PSScriptRoot '..\public\auto-config\diagnose-cc-switch.ps1'
  $diagnosticFunctions = Get-FunctionsFromPowerShellFile -Path $diagnosticPath -Names @(
    'Write-Step',
    'Write-Ok',
    'Convert-ToVersion',
    'Get-OptionalPropertyValue',
    'Get-WindowsReleaseAssetName',
    'Get-MirrorCcSwitchAsset',
    'Install-LatestCcSwitch'
  )
  Invoke-Expression ($diagnosticFunctions -join "`n")

  Assert-True ($null -eq (Get-OptionalPropertyValue -InputObject ([pscustomobject]@{ QuietDisplayName = 'CC Switch' }) -Name 'DisplayName')) 'CC Switch diagnostic accessed a missing DisplayName property under StrictMode'
  Assert-True ((Convert-ToVersion -Value 'CC Switch v3.19.2') -eq [Version]'3.19.2') 'CC Switch diagnostic version parser failed'

  $baseUrl = "http://127.0.0.1:$port"
  $gitAsset = Get-VerifiedSameSiteAsset `
    -ManifestUrl "$baseUrl/git-manifest" `
    -DownloadPrefix "$baseUrl/downloads/git-for-windows/" `
    -Platform 'windows' `
    -Arch 'x64' `
    -NamePattern '(?i)^Git-.*-64-bit\.exe$'
  Assert-True ($gitAsset.Name -eq 'Git-2.51.0-64-bit.exe') 'Git for Windows manifest selected the wrong installer'
  $gitDestination = Join-Path $fixtureDir 'Git 安装包.exe'
  Download-VerifiedAsset -Asset $gitAsset -OutputPath $gitDestination
  Assert-True ((Get-FileHash -LiteralPath $gitDestination -Algorithm SHA256).Hash -eq $expectedSHA256) 'Git for Windows cached installer checksum verification failed'

  # Exercise the real Git installer path with its current-user, silent and
  # no-restart contract. Only Start-Process is mocked; the manifest lookup,
  # package transfer and SHA-256 verification remain real against the fixture.
  $script:GitInstallerLog = Join-Path $fixtureDir 'git-installer-arguments.log'
  function global:Start-Process {
    param(
      [string]$FilePath,
      [object]$ArgumentList,
      [switch]$Wait,
      [switch]$PassThru
    )
    $argumentsText = if ($ArgumentList -is [array]) { $ArgumentList -join ' ' } else { [string]$ArgumentList }
    Set-Content -LiteralPath $script:GitInstallerLog -Encoding UTF8 -Value "$FilePath $argumentsText"
    return [pscustomobject]@{ ExitCode = 0 }
  }
  try {
    $script:GitForWindowsManifestUrl = "$baseUrl/git-manifest"
    $script:GitForWindowsPackagePrefix = "$baseUrl/downloads/git-for-windows/"
    $env:PROCESSOR_ARCHITECTURE = 'AMD64'
    Install-Git
    $gitInstallerArgs = Get-Content -LiteralPath $script:GitInstallerLog -Raw
    Assert-True ($gitInstallerArgs.Contains('/CURRENTUSER')) 'Git for Windows installer could require administrator privileges'
    Assert-True ($gitInstallerArgs.Contains('/VERYSILENT')) 'Git for Windows installer was not silent'
    Assert-True ($gitInstallerArgs.Contains('/NORESTART')) 'Git for Windows installer could restart the customer machine'
  } finally {
    Remove-Item Function:\global:Start-Process -ErrorAction SilentlyContinue
  }

  $grokAsset = Get-VerifiedSameSiteAsset `
    -ManifestUrl "$baseUrl/grok-manifest" `
    -DownloadPrefix "$baseUrl/downloads/grok-build/" `
    -Platform 'windows' `
    -Arch 'x64' `
    -NamePattern '(?i)^grok-.*-windows-x86_64\.exe$'
  Assert-True ($grokAsset.Name -eq 'grok-1.0.3-windows-x86_64.exe') 'Grok Build manifest selected the wrong installer'
  $grokDestination = Join-Path $fixtureDir 'Grok Build.exe'
  Download-VerifiedAsset -Asset $grokAsset -OutputPath $grokDestination
  Assert-True ((Get-FileHash -LiteralPath $grokDestination -Algorithm SHA256).Hash -eq $expectedSHA256) 'Grok Build cached installer checksum verification failed'

  # Exercise the real Grok installer path and assert the customer-visible
  # command is placed in the managed per-user directory.
  $script:GrokBuildManifestUrl = "$baseUrl/grok-manifest"
  $script:GrokBuildPackagePrefix = "$baseUrl/downloads/grok-build/"
  $GrokDir = Join-Path $fixtureDir '.grok'
  $GrokBinDir = Join-Path $GrokDir 'bin'
  $GrokCommandPath = Join-Path $GrokBinDir 'grok.exe'
  Install-GrokBuild
  Assert-True (Test-Path -LiteralPath $GrokCommandPath -PathType Leaf) 'Grok Build executable was not installed'
  Assert-True (Test-Path -LiteralPath (Join-Path $GrokBinDir 'agent.exe') -PathType Leaf) 'Grok Build agent compatibility executable was not installed'
  Assert-True ((Get-FileHash -LiteralPath $GrokCommandPath -Algorithm SHA256).Hash -eq $expectedSHA256) 'Installed Grok Build executable differs from verified cache payload'

  $codexAsset = Get-VerifiedSameSiteAsset `
    -ManifestUrl "$baseUrl/codex-manifest" `
    -DownloadPrefix "$baseUrl/downloads/codex/" `
    -Platform 'windows' `
    -Arch 'x64' `
    -NamePattern '(?i)^OpenAI\.Codex_.*\.msix$'
  Assert-True ($codexAsset.Name -eq 'OpenAI.Codex_26.803.81509.0_x64__2p2nqsd0c76g0.Msix') 'Codex App manifest selected the wrong Windows package'
  $codexDestination = Join-Path $fixtureDir 'Codex App.msix'
  Download-VerifiedAsset -Asset $codexAsset -OutputPath $codexDestination
  Assert-True ((Get-FileHash -LiteralPath $codexDestination -Algorithm SHA256).Hash -eq $expectedSHA256) 'Codex App cached package checksum verification failed'

  try {
    $env:PROCESSOR_ARCHITECTURE = 'AMD64'
    $env:PROCESSOR_ARCHITEW6432 = ''
    $MirrorManifestUrl = "$baseUrl/cc-manifest"
    $MirrorPackagePrefix = "$baseUrl/downloads/cc-switch/"
    $ccAsset = Get-MirrorCcSwitchAsset
    Assert-True ($ccAsset.Name -eq 'CC-Switch-v3.19.2-Windows.msi') 'CC Switch manifest selected the wrong Windows installer'
    Assert-True ($ccAsset.SHA256 -eq $expectedSHA256.ToLowerInvariant()) 'CC Switch manifest checksum did not match the cached package'

    # Exercise the real installer function without modifying the hosted runner:
    # the cached fixture must reach msiexec with quiet/no-restart switches, and
    # a successful exit code must be observed. Path lookup is mocked only after
    # the download and hash verification have completed.
    $script:CcMsiLog = Join-Path $fixtureDir 'cc-msiexec-arguments.log'
    function global:Start-Process {
      param(
        [string]$FilePath,
        [object]$ArgumentList,
        [switch]$Wait,
        [switch]$PassThru
      )
      $argumentsText = if ($ArgumentList -is [array]) { $ArgumentList -join ' ' } else { [string]$ArgumentList }
      Set-Content -LiteralPath $script:CcMsiLog -Encoding UTF8 -Value "$FilePath $argumentsText"
      return [pscustomobject]@{ ExitCode = 0 }
    }
    function global:Wait-For-CcSwitchExit { }
    function global:Get-Item {
      param([string]$LiteralPath)
      return [pscustomobject]@{ VersionInfo = [pscustomobject]@{ ProductVersion = '3.19.2' } }
    }
    function global:Test-Path {
      param([string]$LiteralPath, [object]$PathType)
      if ($LiteralPath -eq $script:InstalledExecutable) { return $true }
      return Microsoft.PowerShell.Management\Test-Path @PSBoundParameters
    }
    function global:Resolve-Path {
      param([string]$LiteralPath)
      if ($LiteralPath -eq $script:InstalledExecutable) { return [pscustomobject]@{ Path = $LiteralPath } }
      return Microsoft.PowerShell.Management\Resolve-Path @PSBoundParameters
    }
    try {
      $script:InstalledExecutable = Join-Path $fixtureDir 'CC Switch\cc-switch.exe'
      $installedCcSwitch = Install-LatestCcSwitch -ReleaseAsset $ccAsset
      Assert-True ($installedCcSwitch.Version -eq [Version]'3.19.2') 'CC Switch post-install version was not verified'
      $ccMsiArgs = Get-Content -LiteralPath $script:CcMsiLog -Raw
      Assert-True ($ccMsiArgs.Contains('/passive')) 'CC Switch MSI was not started in passive mode'
      Assert-True ($ccMsiArgs.Contains('/norestart')) 'CC Switch MSI could restart the customer machine'
    } finally {
      Remove-Item Function:\global:Start-Process -ErrorAction SilentlyContinue
      Remove-Item Function:\global:Wait-For-CcSwitchExit -ErrorAction SilentlyContinue
      Remove-Item Function:\global:Get-Item -ErrorAction SilentlyContinue
      Remove-Item Function:\global:Test-Path -ErrorAction SilentlyContinue
      Remove-Item Function:\global:Resolve-Path -ErrorAction SilentlyContinue
    }
  } finally {
    $env:PROCESSOR_ARCHITECTURE = $originalArchitecture
    $env:PROCESSOR_ARCHITEW6432 = $originalArchitectureW6432
  }

  $untrustedManifestRejected = $false
  try {
    Get-VerifiedSameSiteAsset `
      -ManifestUrl "$baseUrl/grok-manifest" `
      -DownloadPrefix 'https://laoshirenai.com/downloads/grok-build/' `
      -Platform 'windows' `
      -Arch 'x64' `
      -NamePattern '(?i)^grok-.*-windows-x86_64\.exe$' | Out-Null
  } catch {
    $untrustedManifestRejected = $_.Exception.Message.Contains('同源校验')
  }
  Assert-True $untrustedManifestRejected 'A non-same-site installer URL was not rejected'

  Invoke-ResourceInstallContract -URLs @(
    "http://127.0.0.1:$port/fail",
    "http://127.0.0.1:$port/ok"
  ) -ExpectedSHA256 $expectedSHA256 -Destination $destination
  Assert-True (-not (Test-Path -LiteralPath $destination)) 'successful install did not clean up the installer'
  $requests = Get-Content -LiteralPath $requestLog -Raw
  Assert-True ($requests.Contains('/fail')) 'primary failure path was not exercised'
  Assert-True ($requests.Contains('/ok')) 'fallback download path was not exercised'

  $checksumFailureObserved = $false
  try {
    Invoke-ResourceInstallContract -URLs @("http://127.0.0.1:$port/ok") -ExpectedSHA256 ('0' * 64) -Destination $destination
  } catch {
    $checksumFailureObserved = $_.Exception.Message.Contains('checksum')
  }
  Assert-True $checksumFailureObserved 'checksum mismatch did not fail closed'
  Assert-True (-not (Test-Path -LiteralPath $destination)) 'checksum failure did not clean up the installer'

  $downloadFailureObserved = $false
  try {
    Invoke-ResourceInstallContract -URLs @("http://127.0.0.1:$port/fail") -ExpectedSHA256 $expectedSHA256 -Destination $destination
  } catch {
    $downloadFailureObserved = $_.Exception.Message.Contains('download failed')
  }
  Assert-True $downloadFailureObserved 'download failure did not stop before installation'
  Assert-True (-not (Test-Path -LiteralPath $destination)) 'download failure did not clean up the installer'

  $global:LASTEXITCODE = 0
  Write-Host "WINDOWS_RESOURCE_INSTALL_ACCEPTANCE_OK runtime=$($PSVersionTable.PSVersion) edition=$($PSVersionTable.PSEdition) claude-desktop=verified codex-plus-plus=verified codex-app=same-site-sha256 git=same-site-sha256 grok=same-site-sha256 cc-switch=same-site-manifest official-provider=parsed fallback=passed checksum=fail-closed download=fail-closed path=unicode-spaces policy=Restricted"
} finally {
  if ($null -ne $server -and -not $server.HasExited) {
    Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue
  }
  Remove-Item Env:RESOURCE_FIXTURE_PATH -ErrorAction SilentlyContinue
  Remove-Item Env:RESOURCE_PORT_FILE -ErrorAction SilentlyContinue
  Remove-Item Env:RESOURCE_REQUEST_LOG -ErrorAction SilentlyContinue
  Remove-Item Env:RESOURCE_FIXTURE_SHA256 -ErrorAction SilentlyContinue
  Set-ExecutionPolicy -Scope Process -ExecutionPolicy $originalExecutionPolicy -Force
  $env:PROCESSOR_ARCHITECTURE = $originalArchitecture
  $env:PROCESSOR_ARCHITEW6432 = $originalArchitectureW6432
  Remove-Item -LiteralPath $fixtureDir -Recurse -Force -ErrorAction SilentlyContinue
}
