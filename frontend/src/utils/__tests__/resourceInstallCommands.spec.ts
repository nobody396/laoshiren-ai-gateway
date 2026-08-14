import { describe, expect, it } from 'vitest'

import {
  buildClaudeDesktopWindowsCachePath,
  buildImmutableResourceDownloadPath,
  buildWindowsDesktopInstallCommand,
  CLAUDE_DESKTOP_WINDOWS_X64
} from '../resourceInstallCommands'

describe('Windows desktop resource install commands', () => {
  it('downloads Claude Desktop from the domestic cache before the immutable official fallback', () => {
    const domesticCache = `https://laoshirenai.com${buildClaudeDesktopWindowsCachePath('4A7FE5BCC95F29DEDBFEEB45BC2C6B916343253BA0E0E392038968F5857C6AA9')}`
    const command = buildWindowsDesktopInstallCommand({
      tool: 'claude-desktop',
      sources: [
        { url: domesticCache, sha256: '4A7FE5BCC95F29DEDBFEEB45BC2C6B916343253BA0E0E392038968F5857C6AA9' },
        CLAUDE_DESKTOP_WINDOWS_X64
      ]
    })

    expect(command.indexOf(domesticCache)).toBeLessThan(command.indexOf(CLAUDE_DESKTOP_WINDOWS_X64.url))
    expect(domesticCache).toBe('https://laoshirenai.com/downloads/claude-desktop/windows-x64/4a7fe5bcc95f29dedbfeeb45bc2c6b916343253ba0e0e392038968f5857c6aa9/Claude-Setup.exe')
    expect(command).toContain('& curl.exe -fL --retry 5')
    expect(command).toContain('--connect-timeout 60 --max-time 1800')
    expect(command).not.toContain('/api/v1/resource-downloads/')
    expect(command).toContain(`h='${CLAUDE_DESKTOP_WINDOWS_X64.sha256}'`)
    expect(command).toContain("h='4A7FE5BCC95F29DEDBFEEB45BC2C6B916343253BA0E0E392038968F5857C6AA9'")
    expect(command).toContain('if($h -eq $s.h)')
    expect(command).toContain("if(-not $ok){ throw '安装包下载失败或文件校验不通过，已停止安装' }")
    expect(command).toContain('Start-Process -FilePath $f -Wait -PassThru')
    expect(command).not.toContain("-ArgumentList '/S'")
    expect(command).not.toContain('Invoke-WebRequest')
    expect(command.indexOf('if(-not $ok)')).toBeLessThan(command.indexOf('Start-Process'))
    expect(command).toContain('finally { Remove-Item $f -Force -ErrorAction SilentlyContinue }')
  })

  it('rejects an invalid checksum in the immutable cache path', () => {
    expect(() => buildClaudeDesktopWindowsCachePath('not-a-sha')).toThrow('SHA256')
  })

  it('builds a content-addressed same-site path for every cached installer', () => {
    expect(buildImmutableResourceDownloadPath('codex-plus-plus', 'v1.2.4', {
      id: 'codexplusplus-1.2.4-windows-x64-setup.exe',
      sha256: 'b'.repeat(64)
    })).toBe(`/downloads/codex-plus-plus/v1.2.4/${'b'.repeat(64)}/codexplusplus-1.2.4-windows-x64-setup.exe`)
  })

  it('keeps the silent installer switch for Codex++ after verification', () => {
    const command = buildWindowsDesktopInstallCommand({
      tool: 'codex-plus-plus',
      sources: [{ url: 'https://laoshirenai.com/download.exe', sha256: 'a'.repeat(64) }]
    })

    expect(command).toContain("Start-Process -FilePath $f -ArgumentList '/S' -Wait -PassThru")
    expect(command).toContain(`h='${'A'.repeat(64)}'`)
  })

  it('refuses to generate an unverifiable Windows installer command', () => {
    expect(() => buildWindowsDesktopInstallCommand({
      tool: 'claude-desktop',
      sources: [{ url: 'https://downloads.claude.ai/Claude.exe', sha256: '' }]
    })).toThrow('SHA256')
  })
})
