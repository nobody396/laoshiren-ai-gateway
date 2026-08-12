import type { DownloadToolID } from '@/api/resources'

export const CLAUDE_DESKTOP_WINDOWS_X64 = {
  url: 'https://downloads.claude.ai/releases/win32/x64/1.25927.0/Claude-003700efafbc2ccb4b1177a5e637b14da381799e.exe',
  sha256: 'BD45B1307385EEF00E88AA8A3CC597ECFD33BF04DA93682ACA9C334D05236C6D'
} as const

function powerShellQuote(value: string): string {
  return `'${value.replace(/'/g, "''")}'`
}

export function buildWindowsDesktopInstallCommand(options: {
  tool: DownloadToolID
  downloadURLs: string[]
  sha256: string
}): string {
  const urls = [...new Set(options.downloadURLs.map((url) => url.trim()).filter(Boolean))]
  if (urls.length === 0) throw new Error('Windows 安装命令缺少下载地址')

  const sha256 = options.sha256.trim().toUpperCase()
  if (!/^[A-F0-9]{64}$/.test(sha256)) throw new Error('Windows 安装命令缺少有效的 SHA256')

  const fileName = options.tool === 'claude-desktop' ? 'Claude-Setup.exe' : 'CodexPlusPlus-Setup.exe'
  const installerArguments = options.tool === 'codex-plus-plus' ? " -ArgumentList '/S'" : ''
  const urlArray = `@(${urls.map(powerShellQuote).join(',')})`

  return `$urls=${urlArray}; $f=Join-Path $env:TEMP '${fileName}'; $ok=$false; try { foreach($u in $urls){ Remove-Item $f -Force -ErrorAction SilentlyContinue; & curl.exe -fL --retry 5 --retry-delay 2 --connect-timeout 20 --max-time 1800 -o $f $u; if($LASTEXITCODE -eq 0 -and (Test-Path -LiteralPath $f -PathType Leaf)){ $h=(Get-FileHash -LiteralPath $f -Algorithm SHA256).Hash; if($h -eq '${sha256}'){ $ok=$true; break } } }; if(-not $ok){ throw '安装包下载失败或文件校验不通过，已停止安装' }; $p=Start-Process -FilePath $f${installerArguments} -Wait -PassThru; if($p.ExitCode -ne 0){ throw "安装程序退出码: $($p.ExitCode)" } } finally { Remove-Item $f -Force -ErrorAction SilentlyContinue }`
}
