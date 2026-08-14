import { describe, expect, it } from 'vitest'

import {
  buildClaudeDesktopWindowsInstallCommand,
  buildImmutableResourceDownloadPath,
  buildWindowsDesktopInstallCommand
} from '../resourceInstallCommands'

describe('Windows desktop resource install commands', () => {
  it('uses the versioned same-site Claude Desktop pair installer', () => {
    const command = buildClaudeDesktopWindowsInstallCommand('https://laoshirenai.com/resources')

    expect(command).toBe("irm 'https://laoshirenai.com/auto-config/install-claude-desktop.ps1?v=1.0.1' | iex")
    expect(command).not.toContain('downloads.claude.ai')
    expect(command).not.toContain('Claude-Setup.exe')
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
    expect(command).toContain("$ca+=@('-C','-')")
    expect(command).toContain('$a -le 5')
  })

  it('refuses to generate an unverifiable Windows installer command', () => {
    expect(() => buildWindowsDesktopInstallCommand({
      tool: 'claude-desktop',
      sources: [{ url: 'https://downloads.claude.ai/Claude.exe', sha256: '' }]
    })).toThrow('SHA256')
  })
})
