import { describe, expect, it } from 'vitest'

import {
  buildWindowsDesktopInstallCommand,
  CLAUDE_DESKTOP_WINDOWS_X64
} from '../resourceInstallCommands'

describe('Windows desktop resource install commands', () => {
  it('downloads Claude Desktop from the domestic cache before the immutable official fallback', () => {
    const domesticCache = 'https://laoshirenai.com/api/v1/resource-downloads/test-token'
    const command = buildWindowsDesktopInstallCommand({
      tool: 'claude-desktop',
      downloadURLs: [domesticCache, CLAUDE_DESKTOP_WINDOWS_X64.url],
      sha256: CLAUDE_DESKTOP_WINDOWS_X64.sha256
    })

    expect(command.indexOf(domesticCache)).toBeLessThan(command.indexOf(CLAUDE_DESKTOP_WINDOWS_X64.url))
    expect(command).toContain('& curl.exe -fL --retry 5')
    expect(command).toContain(`if($h -eq '${CLAUDE_DESKTOP_WINDOWS_X64.sha256}')`)
    expect(command).toContain("if(-not $ok){ throw '安装包下载失败或文件校验不通过，已停止安装' }")
    expect(command).toContain('Start-Process -FilePath $f -Wait -PassThru')
    expect(command).not.toContain("-ArgumentList '/S'")
    expect(command).not.toContain('Invoke-WebRequest')
    expect(command.indexOf('if(-not $ok)')).toBeLessThan(command.indexOf('Start-Process'))
    expect(command).toContain('finally { Remove-Item $f -Force -ErrorAction SilentlyContinue }')
  })

  it('keeps the silent installer switch for Codex++ after verification', () => {
    const command = buildWindowsDesktopInstallCommand({
      tool: 'codex-plus-plus',
      downloadURLs: ['https://laoshirenai.com/download.exe'],
      sha256: 'a'.repeat(64)
    })

    expect(command).toContain("Start-Process -FilePath $f -ArgumentList '/S' -Wait -PassThru")
    expect(command).toContain(`if($h -eq '${'A'.repeat(64)}')`)
  })

  it('refuses to generate an unverifiable Windows installer command', () => {
    expect(() => buildWindowsDesktopInstallCommand({
      tool: 'claude-desktop',
      downloadURLs: ['https://downloads.claude.ai/Claude.exe'],
      sha256: ''
    })).toThrow('SHA256')
  })
})
