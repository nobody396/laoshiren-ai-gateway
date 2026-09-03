import { describe, expect, it } from 'vitest'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  buildClaudeModelListCommand,
  buildClaudeOwnedConfig,
  buildClaudeSettingsCommand,
  buildClaudeVerificationCommand,
  normalizeClaudeBaseUrl,
} from '../claudeCodeManualConfig'

describe('Claude Code manual configuration', () => {
  it('normalizes the Messages gateway root', () => {
    expect(normalizeClaudeBaseUrl('https://api.laoshirenai.com/v1/')).toBe('https://api.laoshirenai.com')
  })

  it('uses the selected model for empty role slots', () => {
    const config = buildClaudeOwnedConfig({
      baseUrl: 'https://api.laoshirenai.com',
      apiKey: 'fixture-key',
      mainModel: 'claude-sonnet-5',
      opusModel: 'claude-opus-5',
    })

    expect(config.model).toBe('claude-sonnet-5')
    expect(config.env.ANTHROPIC_DEFAULT_OPUS_MODEL).toBe('claude-opus-5')
    expect(config.env.ANTHROPIC_DEFAULT_SONNET_MODEL).toBe('claude-sonnet-5')
    expect(config.env.ANTHROPIC_DEFAULT_HAIKU_MODEL).toBe('claude-sonnet-5')
    expect(config.env.ANTHROPIC_DEFAULT_FABLE_MODEL).toBe('claude-sonnet-5')
    expect(config.modelSettingsUpdate.effortLevel).toBeNull()
  })

  it('writes an explicit persistent effort when the user chooses one', () => {
    const config = buildClaudeOwnedConfig({
      baseUrl: 'https://api.laoshirenai.com',
      apiKey: 'fixture-key',
      mainModel: 'claude-sonnet-5',
      effortLevel: 'xhigh',
    })
    expect(config.modelSettingsUpdate).toEqual({ model: 'claude-sonnet-5', effortLevel: 'xhigh' })
  })

  it('keeps Max and Ultracode session-only', () => {
    const maxConfig = buildClaudeOwnedConfig({
      baseUrl: 'https://api.laoshirenai.com', apiKey: 'fixture-key', mainModel: 'claude-sonnet-5', effortLevel: 'max',
    })
    expect(maxConfig.modelSettingsUpdate.effortLevel).toBeNull()
    expect(buildClaudeVerificationCommand('max')).toContain('--effort max')
    expect(buildClaudeVerificationCommand('ultracode')).toContain('--effort ultracode')
  })

  it('uses the normal Claude Code startup path for verification', () => {
    expect(buildClaudeVerificationCommand()).toBe('claude -p "只回复 CLAUDE_CODE_OK"')
  })

  it('generates model-list commands for Terminal and PowerShell', () => {
    const macos = buildClaudeModelListCommand('https://api.laoshirenai.com', 'fixture-key', 'macos')
    expect(macos).toContain('https://api.laoshirenai.com/v1/models')
    expect(macos).toContain('%{http_code}')
    expect(macos).toContain('读取模型失败：HTTP')
    expect(macos).not.toContain('curl -fsS')
    expect(buildClaudeModelListCommand('https://api.laoshirenai.com', 'fixture-key', 'windows'))
      .toContain('Invoke-RestMethod')
  })

  it('generates minimal merge commands with backup and atomic replacement', () => {
    const input = {
      baseUrl: 'https://api.laoshirenai.com',
      apiKey: 'fixture-key',
      mainModel: 'claude-sonnet-5',
    }
    const unix = buildClaudeSettingsCommand(input, 'linux')
    const windows = buildClaudeSettingsCommand(input, 'windows')

    expect(unix).toContain('CLAUDE_CFG_B64=')
    expect(unix).toContain('shutil.copy2')
    expect(unix).toContain('t.replace(p)')
    expect(windows).toContain("Copy-Item -LiteralPath $p -Destination ($p+'.bak')")
    expect(windows).toContain('Move-Item -LiteralPath $tmp -Destination $p -Force')
  })

  it('executes the Terminal command without replacing unrelated settings', () => {
    const home = mkdtempSync(join(tmpdir(), 'claude-manual-config-'))
    const settingsDir = join(home, '.claude')
    const settingsPath = join(settingsDir, 'settings.json')
    mkdirSync(settingsDir)
    writeFileSync(settingsPath, JSON.stringify({ permissions: { allow: ['Read'] }, env: { EXISTING: 'keep' } }))
    try {
      const command = buildClaudeSettingsCommand({
        baseUrl: 'https://api.laoshirenai.com',
        apiKey: 'fixture-key',
        mainModel: 'claude-sonnet-5',
        effortLevel: 'high',
      }, 'macos')
      execFileSync('bash', ['-c', command], { env: { ...process.env, HOME: home } })
      const result = JSON.parse(readFileSync(settingsPath, 'utf8'))

      expect(result.permissions).toEqual({ allow: ['Read'] })
      expect(result.env.EXISTING).toBe('keep')
      expect(result.env.ANTHROPIC_AUTH_TOKEN).toBe('fixture-key')
      expect(result.model).toBe('claude-sonnet-5')
      expect(result.modelSettings['claude-sonnet-5'].effortLevel).toBe('high')
      expect(readFileSync(`${settingsPath}.bak`, 'utf8')).toContain('EXISTING')
    } finally {
      rmSync(home, { recursive: true, force: true })
    }
  })

  it('clears only the selected model effort when auto is selected', () => {
    const home = mkdtempSync(join(tmpdir(), 'claude-manual-effort-auto-'))
    const settingsDir = join(home, '.claude')
    const settingsPath = join(settingsDir, 'settings.json')
    mkdirSync(settingsDir)
    writeFileSync(settingsPath, JSON.stringify({
      modelSettings: {
        'claude-sonnet-5': { effortLevel: 'xhigh' },
        'claude-opus-5': { effortLevel: 'medium' },
      },
    }))
    try {
      const command = buildClaudeSettingsCommand({
        baseUrl: 'https://api.laoshirenai.com',
        apiKey: 'fixture-key',
        mainModel: 'claude-sonnet-5',
        effortLevel: 'auto',
      }, 'linux')
      execFileSync('bash', ['-c', command], { env: { ...process.env, HOME: home } })
      const result = JSON.parse(readFileSync(settingsPath, 'utf8'))

      expect(result.modelSettings['claude-sonnet-5']).toBeUndefined()
      expect(result.modelSettings['claude-opus-5'].effortLevel).toBe('medium')
    } finally {
      rmSync(home, { recursive: true, force: true })
    }
  })
})
