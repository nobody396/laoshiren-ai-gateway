import type { DownloadAsset, DownloadToolID } from '@/api/resources'

export const CLAUDE_DESKTOP_INSTALL_SCRIPT_VERSION = '1.0.0'

export interface WindowsInstallerSource {
  url: string
  sha256: string
}

export function buildClaudeDesktopWindowsInstallCommand(origin: string): string {
  const url = new URL('/auto-config/install-claude-desktop.ps1', origin)
  if (url.protocol !== 'https:' && url.hostname !== '127.0.0.1' && url.hostname !== 'localhost') {
    throw new Error('Claude Desktop 安装脚本必须使用 HTTPS')
  }
  url.searchParams.set('v', CLAUDE_DESKTOP_INSTALL_SCRIPT_VERSION)
  return `irm ${powerShellQuote(url.toString())} | iex`
}

export function buildImmutableResourceDownloadPath(
  tool: DownloadToolID,
  version: string,
  asset: Pick<DownloadAsset, 'id' | 'sha256'>
): string {
  const normalizedVersion = version.trim().toLowerCase().replace(/[^a-z0-9._-]+/g, '-').replace(/^-+|-+$/g, '')
  const normalizedSHA256 = asset.sha256.trim().toLowerCase()
  if (!normalizedVersion) throw new Error('安装包缺少有效版本号')
  if (!/^[a-f0-9]{64}$/.test(normalizedSHA256)) throw new Error('安装包缺少有效的 SHA256')
  if (!/^[a-z0-9._-]+$/.test(asset.id)) throw new Error('安装包缺少有效文件名')
  return `/downloads/${tool}/${normalizedVersion}/${normalizedSHA256}/${asset.id}`
}

function powerShellQuote(value: string): string {
  return `'${value.replace(/'/g, "''")}'`
}

export function buildWindowsDesktopInstallCommand(options: {
  tool: DownloadToolID
  sources: WindowsInstallerSource[]
}): string {
  const sources = options.sources
    .map((source) => ({ url: source.url.trim(), sha256: source.sha256.trim().toUpperCase() }))
    .filter((source, index, all) => source.url && all.findIndex((item) => item.url === source.url) === index)
  if (sources.length === 0) throw new Error('Windows 安装命令缺少下载地址')
  if (sources.some((source) => !/^[A-F0-9]{64}$/.test(source.sha256))) {
    throw new Error('Windows 安装命令缺少有效的 SHA256')
  }

  const fileName = options.tool === 'claude-desktop' ? 'Claude-Setup.exe' : 'CodexPlusPlus-Setup.exe'
  const installerArguments = options.tool === 'codex-plus-plus' ? " -ArgumentList '/S'" : ''
  const sourceArray = `@(${sources.map((source) => `@{u=${powerShellQuote(source.url)};h='${source.sha256}'}`).join(',')})`

  return `$src=${sourceArray}; $f=Join-Path $env:TEMP '${fileName}'; $ok=$false; try { foreach($s in $src){ Remove-Item $f -Force -ErrorAction SilentlyContinue; & curl.exe -fL --retry 5 --retry-delay 2 --connect-timeout 60 --max-time 1800 -o $f $s.u; if($LASTEXITCODE -eq 0 -and (Test-Path -LiteralPath $f -PathType Leaf)){ $h=(Get-FileHash -LiteralPath $f -Algorithm SHA256).Hash; if($h -eq $s.h){ $ok=$true; break } } }; if(-not $ok){ throw '安装包下载失败或文件校验不通过，已停止安装' }; $p=Start-Process -FilePath $f${installerArguments} -Wait -PassThru; if($p.ExitCode -ne 0){ throw "安装程序退出码: $($p.ExitCode)" } } finally { Remove-Item $f -Force -ErrorAction SilentlyContinue }`
}
