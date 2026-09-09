import { describe, expect, it } from 'vitest'
import { clientAutoConfigVersion } from '@/generated/modelCatalog'

import {
  buildClientAutoConfigCommand,
  buildClientManualConfigCommand,
  getClientAutoConfigName,
  getClientAutoConfigTarget
} from '@/utils/clientAutoConfig'

describe('client auto-config target selection', () => {
  it('maps OpenAI groups to Codex', () => {
    expect(getClientAutoConfigTarget('openai')).toBe('codex')
  })

  it('maps Anthropic-compatible groups to Claude Code', () => {
    expect(getClientAutoConfigTarget('anthropic')).toBe('claude')
    expect(getClientAutoConfigTarget('antigravity')).toBe('claude')
  })

  it('maps native Grok groups to Grok Build', () => {
    expect(getClientAutoConfigTarget('grok')).toBe('grok')
  })

  it('maps Gemini groups to Gemini CLI', () => {
    expect(getClientAutoConfigTarget('gemini')).toBe('gemini')
  })

  it('does not offer an incompatible client setup', () => {
    expect(getClientAutoConfigTarget('gpt-image')).toBeNull()
    expect(getClientAutoConfigTarget()).toBeNull()
  })

  it('returns reader-facing client names', () => {
    expect(getClientAutoConfigName('claude')).toBe('Claude Code')
    expect(getClientAutoConfigName('codex')).toBe('Codex')
    expect(getClientAutoConfigName('grok')).toBe('Grok Build')
    expect(getClientAutoConfigName('gemini')).toBe('Gemini CLI')
    expect(getClientAutoConfigName('kimi')).toBe('Kimi Code')
    expect(getClientAutoConfigName('opencode')).toBe('OpenCode')
    expect(getClientAutoConfigName('zcode')).toBe('ZCode')
    expect(getClientAutoConfigName('workbuddy')).toBe('WorkBuddy')
  })
})

describe('client auto-config commands', () => {
  it('builds a digest-verified manual Grok/Gemini command from visible fields', () => {
    const shell = buildClientManualConfigCommand({
      target: 'grok',
      apiKey: 'fixture-key',
      baseUrl: 'https://api.laoshirenai.com',
      modelId: 'gpt-5.6-sol',
      protocol: 'responses',
      isWindows: false,
    })
    const windows = buildClientManualConfigCommand({
      target: 'gemini',
      apiKey: 'fixture-key',
      baseUrl: 'https://api.laoshirenai.com',
      modelId: 'gemini-3.7-flash',
      protocol: 'generate_content',
      reasoningEffort: 'low',
      isWindows: true,
    })
    expect(shell).toContain("LAOSHIRENAI_GROK_API_KEY='fixture-key'")
    expect(shell).toContain("LAOSHIRENAI_MODEL_ID='gpt-5.6-sol'")
    expect(shell).toContain("LAOSHIRENAI_PROTOCOL='responses'")
    expect(shell).toContain('shasum -a 256')
    expect(windows).toContain("$env:LAOSHIRENAI_GEMINI_API_KEY='fixture-key'")
    expect(windows).toContain("$env:LAOSHIRENAI_PROTOCOL='generate_content'")
    expect(windows).toContain("$env:LAOSHIRENAI_REASONING_EFFORT='low'")
    expect(windows).toContain('Get-FileHash')
  })

  it('builds a Windows Codex command with a one-time ticket instead of an API key', () => {
    const command = buildClientAutoConfigCommand({
      target: 'codex',
      ticket: 'ticket-codex-test',
      isWindows: true
    })
    expect(command).toContain("$env:LAOSHIRENAI_SETUP_TOKEN='ticket-codex-test'")
    expect(command).toContain("$env:LAOSHIRENAI_SKIP_CLIENT_INSTALL='1'")
    expect(command).toContain(`install.ps1?v=${clientAutoConfigVersion}`)
    expect(command).toContain('Get-FileHash')
  })

  it('adds Codex App installation only when explicitly selected', () => {
    const withApp = buildClientAutoConfigCommand({
      target: 'codex',
      ticket: 'ticket-codex-app',
      isWindows: true,
      installCodexApp: true
    })
    const cliOnly = buildClientAutoConfigCommand({
      target: 'codex',
      ticket: 'ticket-codex-cli',
      isWindows: false
    })

    expect(withApp).toContain("$env:LAOSHIRENAI_INSTALL_CODEX_APP='1'")
    expect(cliOnly).not.toContain('LAOSHIRENAI_INSTALL_CODEX_APP')
  })

  it('installs a missing client when the released one-click option requests it', () => {
    const shell = buildClientAutoConfigCommand({
      target: 'codex', ticket: 'ticket-install', isWindows: false, installMissing: true
    })
    const windows = buildClientAutoConfigCommand({
      target: 'codex', ticket: 'ticket-install', isWindows: true, installMissing: true
    })
    expect(shell).not.toContain('LAOSHIRENAI_SKIP_CLIENT_INSTALL')
    expect(windows).not.toContain('LAOSHIRENAI_SKIP_CLIENT_INSTALL')
  })

  it('never adds Codex App installation to Claude Code commands', () => {
    const command = buildClientAutoConfigCommand({
      target: 'claude',
      ticket: 'ticket-claude-app-ignored',
      isWindows: false,
      installCodexApp: true
    })
    expect(command).not.toContain('LAOSHIRENAI_INSTALL_CODEX_APP')
  })

  it('escapes a one-time ticket in a Windows Claude Code command', () => {
    const command = buildClientAutoConfigCommand({
      target: 'claude',
      ticket: "ticket-claude'test",
      isWindows: true
    })

    expect(command).toContain("$env:LAOSHIRENAI_SETUP_TOKEN='ticket-claude''test'")
    expect(command).not.toContain('LAOSHIRENAI_CLAUDE_API_KEY')
  })

  it('builds a one-line macOS Claude Code command with a one-time ticket', () => {
    const command = buildClientAutoConfigCommand({
      target: 'claude',
      ticket: 'ticket-claude-test',
      isWindows: false
    })
    expect(command).toContain("LAOSHIRENAI_SETUP_TOKEN='ticket-claude-test'")
    expect(command).toContain("LAOSHIRENAI_SKIP_CLIENT_INSTALL='1'")
    expect(command).toContain(`install.sh?v=${clientAutoConfigVersion}`)
    expect(command).toContain('shasum -a 256')
  })

  it('builds a one-line Grok Build command with a one-time ticket', () => {
    const command = buildClientAutoConfigCommand({
      target: 'grok',
      ticket: 'ticket-grok-test',
      isWindows: false
    })
    expect(command).toContain("LAOSHIRENAI_SETUP_TOKEN='ticket-grok-test'")
    expect(command).toContain("LAOSHIRENAI_TOOLS='grok'")
    expect(command).toContain('shasum -a 256')
  })

  it('builds Gemini CLI commands with a one-time ticket on both platforms', () => {
    const shellCommand = buildClientAutoConfigCommand({
      target: 'gemini',
      ticket: 'ticket-gemini-test',
      isWindows: false
    })
    const windowsCommand = buildClientAutoConfigCommand({
      target: 'gemini',
      ticket: 'ticket-gemini-test',
      isWindows: true
    })
    expect(shellCommand).toContain("LAOSHIRENAI_TOOLS='gemini'")
    expect(shellCommand).toContain('shasum -a 256')
    expect(windowsCommand).toContain("$env:LAOSHIRENAI_TOOLS='gemini'")
    expect(windowsCommand).toContain('Get-FileHash')
  })

  it.each([false, true])(
    'builds the official CC Switch 3.19.2 Grok compatibility command on Windows=%s',
    (isWindows) => {
      const command = buildClientAutoConfigCommand({
        target: 'grok',
        ticket: 'ticket-grok-cc-switch',
        isWindows,
        grokCcSwitchCompat: true
      })

      expect(command).toContain("LAOSHIRENAI_TOOLS='grok'")
      expect(command).toContain("LAOSHIRENAI_GROK_CC_SWITCH_COMPAT='1'")
      if (!isWindows) {
        expect(command).toContain(`curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=${clientAutoConfigVersion}' -o "$f"`)
        expect(command).toContain('shasum -a 256')
        expect(command).not.toContain(
          `curl -fsSL https://laoshirenai.com/auto-config/install.sh?v=${clientAutoConfigVersion}`
        )
      }
      expect(command).not.toContain('sk-')
      expect(command).not.toContain('api.laoshirenai.com')
    }
  )

  it('never adds Grok compatibility mode to another client', () => {
    const command = buildClientAutoConfigCommand({
      target: 'claude',
      ticket: 'ticket-claude-no-grok',
      isWindows: false,
      grokCcSwitchCompat: true
    })
    expect(command).not.toContain('LAOSHIRENAI_GROK_CC_SWITCH_COMPAT')
  })

  it('never places a raw API key or base URL in the copied command', () => {
    const command = buildClientAutoConfigCommand({
      target: 'claude',
      ticket: '0123456789abcdef',
      isWindows: false
    })

    expect(command).toContain('LAOSHIRENAI_SETUP_TOKEN=')
    expect(command).not.toContain('sk-')
    expect(command).not.toContain('api.laoshirenai.com')
  })
})
