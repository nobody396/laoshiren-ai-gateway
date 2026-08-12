import type { DownloadToolID } from '@/api/resources'

export const CLAUDE_DESKTOP_WINDOWS_X64 = {
  url: 'https://downloads.claude.ai/releases/win32/x64/1.25927.0/Claude-003700efafbc2ccb4b1177a5e637b14da381799e.exe',
  sha256: 'BD45B1307385EEF00E88AA8A3CC597ECFD33BF04DA93682ACA9C334D05236C6D'
} as const

export interface WindowsInstallerSource {
  url: string
  sha256: string
}

export function buildClaudeDesktopWindowsCachePath(sha256: string): string {
  const normalized = sha256.trim().toLowerCase()
  if (!/^[a-f0-9]{64}$/.test(normalized)) {
    throw new Error('Claude Desktop 安装包缺少有效的 SHA256')
  }
  return `/downloads/claude-desktop/windows-x64/${normalized}/Claude-Setup.exe`
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
