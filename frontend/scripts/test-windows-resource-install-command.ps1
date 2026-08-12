Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Assert-True {
  param([bool]$Condition, [string]$Message)
  if (-not $Condition) { throw $Message }
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
      & curl.exe -fL --retry 2 --retry-delay 1 --connect-timeout 5 --max-time 30 -o $Destination $url
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
New-Item -ItemType Directory -Path $fixtureDir -Force | Out-Null

try {
  Set-ExecutionPolicy -Scope Process -ExecutionPolicy Restricted -Force
  Assert-True ((Get-ExecutionPolicy -Scope Process) -eq 'Restricted') 'failed to enable Restricted execution policy'

  $fixtureExe = Join-Path $fixtureDir 'fixture-installer.exe'
  Copy-Item -LiteralPath (Join-Path $env:SystemRoot 'System32\whoami.exe') -Destination $fixtureExe -Force
  $expectedSHA256 = (Get-FileHash -LiteralPath $fixtureExe -Algorithm SHA256).Hash
  $portFile = Join-Path $fixtureDir 'server-port.txt'
  $requestLog = Join-Path $fixtureDir 'requests.log'
  $serverScript = Join-Path $fixtureDir 'server.mjs'
  Set-Content -LiteralPath $serverScript -Encoding Ascii -Value @'
import { createServer } from 'node:http';
import { appendFileSync, readFileSync, writeFileSync } from 'node:fs';
const fixture = process.env.RESOURCE_FIXTURE_PATH;
const portFile = process.env.RESOURCE_PORT_FILE;
const requestLog = process.env.RESOURCE_REQUEST_LOG;
const payload = readFileSync(fixture);
const server = createServer((request, response) => {
  appendFileSync(requestLog, `${request.url}\n`);
  if (request.url === '/ok') {
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
  $server = Start-Process -FilePath 'node.exe' -ArgumentList @('"' + $serverScript + '"') -WindowStyle Hidden -PassThru

  for ($attempt = 0; $attempt -lt 50 -and -not (Test-Path -LiteralPath $portFile); $attempt++) {
    Start-Sleep -Milliseconds 100
  }
  Assert-True (Test-Path -LiteralPath $portFile) 'local fixture server did not start'
  $port = (Get-Content -LiteralPath $portFile -Raw).Trim()
  $destination = Join-Path $fixtureDir 'Claude Setup 下载.exe'

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

  Write-Host 'WINDOWS_RESOURCE_INSTALL_ACCEPTANCE_OK fallback=passed checksum=fail-closed download=fail-closed path=unicode-spaces policy=Restricted'
} finally {
  if ($null -ne $server -and -not $server.HasExited) {
    Stop-Process -Id $server.Id -Force -ErrorAction SilentlyContinue
  }
  Remove-Item Env:RESOURCE_FIXTURE_PATH -ErrorAction SilentlyContinue
  Remove-Item Env:RESOURCE_PORT_FILE -ErrorAction SilentlyContinue
  Remove-Item Env:RESOURCE_REQUEST_LOG -ErrorAction SilentlyContinue
  Set-ExecutionPolicy -Scope Process -ExecutionPolicy $originalExecutionPolicy -Force
  Remove-Item -LiteralPath $fixtureDir -Recurse -Force -ErrorAction SilentlyContinue
}
