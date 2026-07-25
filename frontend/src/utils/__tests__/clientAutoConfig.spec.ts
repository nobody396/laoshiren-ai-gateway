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
  it('builds the existing Windows Codex setup command', () => {
    expect(buildClientAutoConfigCommand({
      target: 'codex',
      platform: 'openai',
      apiKey: 'sk-codex-test',
      baseUrl: 'https://api.laoshirenai.com/',
      isWindows: true
    })).toBe(
      "$env:LAOSHIRENAI_CODEX_API_KEY='sk-codex-test'; " +
      "$env:LAOSHIRENAI_TOOLS='codex'; " +
      "$env:LAOSHIRENAI_BASE_URL='https://api.laoshirenai.com'; " +
      'irm https://laoshirenai.com/auto-config/install.ps1 | iex'
    )
  })

  it('builds a one-line Windows Claude Code setup command', () => {
    expect(buildClientAutoConfigCommand({
      target: 'claude',
      platform: 'anthropic',
      apiKey: "sk-claude'test",
      baseUrl: 'https://api.laoshirenai.com',
      isWindows: true
    })).toContain(
      "$env:LAOSHIRENAI_CLAUDE_API_KEY='sk-claude''test'; $env:LAOSHIRENAI_TOOLS='claude'"
    )
  })

  it('builds a one-line macOS Claude Code setup command with the Antigravity endpoint', () => {
    expect(buildClientAutoConfigCommand({
      target: 'claude',
      platform: 'antigravity',
      apiKey: 'sk-antigravity-test',
      baseUrl: 'https://api.laoshirenai.com/',
      isWindows: false
    })).toBe(
      "curl -fsSL https://laoshirenai.com/auto-config/install.sh | bash -s -- " +
      "--api-key 'sk-antigravity-test' --tools claude " +
      "--base-url 'https://api.laoshirenai.com/antigravity'"
    )
  })

  it('does not duplicate an existing Antigravity path', () => {
    const command = buildClientAutoConfigCommand({
      target: 'claude',
      platform: 'antigravity',
      apiKey: 'sk-antigravity-test',
      baseUrl: 'https://api.laoshirenai.com/antigravity/',
      isWindows: false
    })

    expect(command).toContain("--base-url 'https://api.laoshirenai.com/antigravity'")
    expect(command).not.toContain('/antigravity/antigravity')
  })
})
