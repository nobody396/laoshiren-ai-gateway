import { describe, expect, it } from 'vitest'
import { execFileSync } from 'node:child_process'
import { mkdtempSync, mkdirSync, existsSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import {
  buildCodexOwnedConfig,
  buildCodexSettingsCommand,
  buildCodexVerificationCommand,
  normalizeCodexBaseUrl,
} from '../codexManualConfig'

describe('Codex manual configuration', () => {
  it('normalizes the Responses provider base URL and defaults review to the main model', () => {
    expect(normalizeCodexBaseUrl('https://api.laoshirenai.com/')).toBe('https://api.laoshirenai.com/v1')
    const config = buildCodexOwnedConfig({
      baseUrl: 'https://api.laoshirenai.com', apiKey: 'fixture-key', mainModel: 'gpt-5.6-sol', reasoningEffort: 'high',
    })
    expect(config.reviewModel).toBe('gpt-5.6-sol')
    expect(config.effort).toBe('high')
    expect(config.catalog.models.some(model => model.slug === 'gpt-5.6-sol')).toBe(true)
  })

  it('keeps the selected model exact reasoning levels, including Max', () => {
    const config = buildCodexOwnedConfig({
      baseUrl: 'https://api.laoshirenai.com', apiKey: 'fixture-key', mainModel: 'gpt-5.6-sol', reasoningEffort: 'max',
    })
    expect(config.effort).toBe('max')
    expect(config.catalog.models[0].supported_reasoning_levels.map((row: { effort: string }) => row.effort))
      .toEqual(['none', 'low', 'medium', 'high', 'xhigh', 'max', 'ultra'])
  })

  it('does not write a level that the selected model does not support', () => {
    const config = buildCodexOwnedConfig({
      baseUrl: 'https://api.laoshirenai.com', apiKey: 'fixture-key', mainModel: 'gpt-5.5', reasoningEffort: 'max',
    })
    expect(config.effort).toBe('')
  })

  it('merges config, auth, and catalog without replacing unrelated Codex settings', () => {
    const home = mkdtempSync(join(tmpdir(), 'codex-manual-config-'))
    const dir = join(home, '.codex')
    mkdirSync(dir)
    writeFileSync(join(dir, 'config.toml'), 'notify = ["keep"]\n\n[mcp_servers.keep]\ncommand = "keep"\n')
    writeFileSync(join(dir, 'auth.json'), JSON.stringify({ KEEP: 'yes' }))
    try {
      const command = buildCodexSettingsCommand({
        baseUrl: 'https://api.laoshirenai.com',
        apiKey: 'fixture-key',
        mainModel: 'gpt-5.6-sol',
        reviewModel: 'gpt-5.4',
        reasoningEffort: 'high',
        availableModels: ['gpt-5.6-sol', 'gpt-5.4'],
      }, 'macos')
      execFileSync('bash', ['-c', command], { env: { ...process.env, HOME: home } })
      const first = readFileSync(join(dir, 'config.toml'), 'utf8')
      execFileSync('bash', ['-c', command], { env: { ...process.env, HOME: home } })
      expect(readFileSync(join(dir, 'config.toml'), 'utf8')).toBe(first)
      expect(first).toContain('model = "gpt-5.6-sol"')
      expect(first).toContain('review_model = "gpt-5.4"')
      expect(first).toContain('[mcp_servers.keep]')
      expect(first).toContain('command = "keep"')
      const auth = JSON.parse(readFileSync(join(dir, 'auth.json'), 'utf8'))
      expect(auth).toEqual({ KEEP: 'yes', OPENAI_API_KEY: 'fixture-key' })
      const catalog = JSON.parse(readFileSync(join(dir, 'laoshirenai-model-catalog.json'), 'utf8'))
      expect(catalog.models.map((model: { slug: string }) => model.slug)).toEqual(['gpt-5.6-sol', 'gpt-5.4'])
      expect(readFileSync(join(dir, 'config.toml.bak'), 'utf8')).toContain('gpt-5.6-sol')
    } finally {
      rmSync(home, { recursive: true, force: true })
    }
  })

  it('fails without touching auth.json or the catalog when config.toml is malformed', () => {
    const home = mkdtempSync(join(tmpdir(), 'codex-manual-config-'))
    const dir = join(home, '.codex')
    mkdirSync(dir)
    writeFileSync(join(dir, 'config.toml'), '[broken\nkey = 1\n')
    writeFileSync(join(dir, 'auth.json'), JSON.stringify({ KEEP: 'yes' }))
    try {
      const command = buildCodexSettingsCommand({
        baseUrl: 'https://api.laoshirenai.com',
        apiKey: 'fixture-key',
        mainModel: 'gpt-5.6-sol',
      }, 'macos')
      expect(() => execFileSync('bash', ['-c', command], { env: { ...process.env, HOME: home }, stdio: 'pipe' })).toThrow()
      expect(JSON.parse(readFileSync(join(dir, 'auth.json'), 'utf8'))).toEqual({ KEEP: 'yes' })
      expect(readFileSync(join(dir, 'config.toml'), 'utf8')).toBe('[broken\nkey = 1\n')
      expect(existsSync(join(dir, 'laoshirenai-model-catalog.json'))).toBe(false)
      expect(existsSync(join(dir, 'config.toml.bak'))).toBe(false)
      expect(existsSync(join(dir, 'auth.json.bak'))).toBe(false)
    } finally {
      rmSync(home, { recursive: true, force: true })
    }
  })

  it('generates Windows and real Codex verification commands', () => {
    const windows = buildCodexSettingsCommand({
      baseUrl: 'https://api.laoshirenai.com', apiKey: 'fixture-key', mainModel: 'gpt-5.6-sol',
    }, 'windows')
    expect(windows).toContain('model_providers.laoshirenai_responses')
    expect(windows).toContain("Copy-Item -LiteralPath $p -Destination ($p+'.bak')")
    const validateAt = windows.indexOf('refusing malformed TOML header')
    const backupAt = windows.indexOf('Copy-Item')
    const authWriteAt = windows.indexOf("$at=$ap+'.tmp'")
    const catalogWriteAt = windows.indexOf("$mt=$mp+'.tmp'")
    const configWriteAt = windows.indexOf("$ct=$cp+'.tmp'")
    expect(validateAt).toBeGreaterThan(-1)
    expect(validateAt).toBeLessThan(backupAt)
    expect(backupAt).toBeLessThan(authWriteAt)
    expect(authWriteAt).toBeLessThan(catalogWriteAt)
    expect(catalogWriteAt).toBeLessThan(configWriteAt)
    expect(buildCodexVerificationCommand('macos')).toContain('codex exec --skip-git-repo-check --ephemeral --json')
  })
})
