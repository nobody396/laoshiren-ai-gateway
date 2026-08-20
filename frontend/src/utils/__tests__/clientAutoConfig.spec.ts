import { describe, expect, it } from 'vitest'
import { clientAutoConfigVersion } from '@/generated/modelCatalog'

import {
  buildClientAutoConfigCommand,
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
  })
})

describe('client auto-config commands', () => {
  it('builds a Windows Codex command with a one-time ticket instead of an API key', () => {
    expect(buildClientAutoConfigCommand({
      target: 'codex',
      ticket: 'ticket-codex-test',
      isWindows: true
    })).toBe(
      "$env:LAOSHIRENAI_SETUP_TOKEN='ticket-codex-test'; " +
      "$env:LAOSHIRENAI_TOOLS='codex'; " +
      `irm https://laoshirenai.com/auto-config/install.ps1?v=${clientAutoConfigVersion} | iex`
    )
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
    expect(buildClientAutoConfigCommand({
      target: 'claude',
      ticket: 'ticket-claude-test',
      isWindows: false
    })).toBe(
      `curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=${clientAutoConfigVersion}' | ` +
      "LAOSHIRENAI_SETUP_TOKEN='ticket-claude-test' LAOSHIRENAI_TOOLS='claude' bash"
    )
  })

  it('builds a one-line Grok Build command with a one-time ticket', () => {
    expect(buildClientAutoConfigCommand({
      target: 'grok',
      ticket: 'ticket-grok-test',
      isWindows: false
    })).toBe(
      `curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=${clientAutoConfigVersion}' | ` +
      "LAOSHIRENAI_SETUP_TOKEN='ticket-grok-test' LAOSHIRENAI_TOOLS='grok' bash"
    )
  })

  it('builds Gemini CLI commands with a one-time ticket on both platforms', () => {
    expect(buildClientAutoConfigCommand({
      target: 'gemini',
      ticket: 'ticket-gemini-test',
      isWindows: false
    })).toBe(
      `curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=${clientAutoConfigVersion}' | ` +
      "LAOSHIRENAI_SETUP_TOKEN='ticket-gemini-test' LAOSHIRENAI_TOOLS='gemini' bash"
    )
    expect(buildClientAutoConfigCommand({
      target: 'gemini',
      ticket: 'ticket-gemini-test',
      isWindows: true
    })).toBe(
      "$env:LAOSHIRENAI_SETUP_TOKEN='ticket-gemini-test'; " +
      "$env:LAOSHIRENAI_TOOLS='gemini'; " +
      `irm https://laoshirenai.com/auto-config/install.ps1?v=${clientAutoConfigVersion} | iex`
    )
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
        expect(command).toContain(
          `curl -fsSL 'https://laoshirenai.com/auto-config/install.sh?v=${clientAutoConfigVersion}' |`
        )
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
