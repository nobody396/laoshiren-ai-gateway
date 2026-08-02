import { describe, expect, it } from 'vitest'

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

  it('does not offer an incompatible client setup', () => {
    expect(getClientAutoConfigTarget('gemini')).toBeNull()
    expect(getClientAutoConfigTarget('gpt-image')).toBeNull()
    expect(getClientAutoConfigTarget()).toBeNull()
  })

  it('returns reader-facing client names', () => {
    expect(getClientAutoConfigName('claude')).toBe('Claude Code')
    expect(getClientAutoConfigName('codex')).toBe('Codex')
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
      'irm https://laoshirenai.com/auto-config/install.ps1 | iex'
    )
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
      "curl -fsSL https://laoshirenai.com/auto-config/install.sh | " +
      "LAOSHIRENAI_SETUP_TOKEN='ticket-claude-test' LAOSHIRENAI_TOOLS='claude' bash"
    )
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
