import { describe, expect, it } from 'vitest'

import {
  buildCcsImportDeeplink,
  buildCodexModelCatalog,
  CODEX_AUTO_COMPACT_TOKEN_LIMIT,
  CODEX_CONTEXT_WINDOW_TOKENS,
  CODEX_EFFECTIVE_CONTEXT_WINDOW_PERCENT,
  CODEX_LONG_AUTO_COMPACT_TOKEN_LIMIT,
  CODEX_LONG_CONTEXT_WINDOW_TOKENS,
  getCompatibleCcsTargets,
  OPENAI_CODEX_MODELS,
  type CcsImportTarget,
  type CodexContextProfile
} from '@/utils/ccSwitchImport'

const decodeBase64Utf8 = (value: string): string => {
  const binary = atob(value)
  const bytes = Uint8Array.from(binary, char => char.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}

const parseDeepLink = (
  target: CcsImportTarget,
  allowMessagesDispatch = false,
  availableModels?: readonly string[],
  codexContextProfile: CodexContextProfile = 'standard'
) => {
  const deepLink = buildCcsImportDeeplink({
    apiBaseUrl: 'https://api.laoshirenai.com/',
    siteName: '老实人 AI',
    target,
    availableModels,
    codexContextProfile,
    key: {
      key: 'sk-test-not-a-secret',
      name: '测试密钥',
      group: {
        platform: 'openai',
        name: 'OpenAI 月卡',
        allow_messages_dispatch: allowMessagesDispatch
      }
    }
  })
  return new URL(deepLink)
}

describe('CC Switch import compatibility', () => {
  it('offers all OpenAI-compatible clients for an OpenAI group', () => {
    expect(getCompatibleCcsTargets('openai')).toEqual([
      'codex',
      'opencode',
      'openclaw',
      'hermes'
    ])
  })

  it('adds Claude clients only when OpenAI Messages dispatch is enabled', () => {
    expect(getCompatibleCcsTargets('openai', true)).toEqual([
      'codex',
      'opencode',
      'openclaw',
      'hermes',
      'claude'
    ])
  })

  it('keeps native Anthropic and Antigravity targets protocol-safe', () => {
    expect(getCompatibleCcsTargets('anthropic')).toEqual(['claude'])
    expect(getCompatibleCcsTargets('antigravity')).toEqual(['claude', 'gemini'])
  })

  it('does not claim a Claude Desktop deeplink that CC Switch does not expose', () => {
    expect(getCompatibleCcsTargets('anthropic')).not.toContain('claude-desktop')
  })

  it('does not offer coding-agent imports for image-only groups', () => {
    expect(getCompatibleCcsTargets('gpt-image')).toEqual([])
  })

  it('offers Grok groups only to the native Grok Build client', () => {
    expect(getCompatibleCcsTargets('grok')).toEqual(['grokbuild'])
  })
})

describe('CC Switch provider deeplinks', () => {
  it('compacts Codex tasks before the upstream context ceiling', () => {
    const catalog = JSON.parse(buildCodexModelCatalog())

    expect(CODEX_CONTEXT_WINDOW_TOKENS).toBe(272000)
    expect(CODEX_EFFECTIVE_CONTEXT_WINDOW_PERCENT).toBe(95)
    expect(CODEX_AUTO_COMPACT_TOKEN_LIMIT).toBe(258000)
    for (const model of catalog.models) {
      expect(model.context_window).toBe(CODEX_CONTEXT_WINDOW_TOKENS)
      expect(model.max_context_window).toBe(CODEX_CONTEXT_WINDOW_TOKENS)
      expect(model.effective_context_window_percent).toBe(CODEX_EFFECTIVE_CONTEXT_WINDOW_PERCENT)
      expect(model.auto_compact_token_limit).toBe(CODEX_AUTO_COMPACT_TOKEN_LIMIT)
    }
  })

  it.each(['codex', 'opencode', 'openclaw', 'hermes'] as CcsImportTarget[])(
    'builds an OpenAI-compatible %s provider with the /v1 endpoint',
    (target) => {
      const url = parseDeepLink(target)
      expect(url.searchParams.get('app')).toBe(target)
      expect(url.searchParams.get('endpoint')).toBe('https://api.laoshirenai.com/v1')
      expect(url.searchParams.get('model')).toBe('gpt-5.6-sol')
      expect(url.searchParams.get('usageBaseUrl')).toBe('https://api.laoshirenai.com')
      if (target === 'codex') expect(url.searchParams.get('name')).toContain('272K')
    }
  )

  it('embeds the complete production Codex model catalog', () => {
    const url = parseDeepLink('codex')
    const encodedConfig = url.searchParams.get('config')
    expect(encodedConfig).not.toBeNull()
    expect(url.searchParams.get('configFormat')).toBe('json')

    const config = JSON.parse(decodeBase64Utf8(encodedConfig!))
    expect(config.modelCatalog.models).toEqual(OPENAI_CODEX_MODELS)
    const importedModels = config.modelCatalog.models.map((model: { model: string }) => model.model)
    expect(importedModels).toEqual([
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.6',
      'gpt-5.5',
      'gpt-5.4'
    ])
    expect(importedModels).not.toContain('gpt-5.6-luna')
    expect(importedModels).not.toContain('gpt-5.4-mini')
    expect(importedModels).not.toContain('gpt-5.3-codex-spark')
    expect(config.config).toContain('model = "gpt-5.6-sol"')
    expect(config.config).toContain('model_context_window = 272000')
    expect(config.config).toContain('model_auto_compact_token_limit = 258000')
    expect(config.config).toContain('base_url = "https://api.laoshirenai.com/v1"')
    expect(config.config).not.toContain('sk-test-not-a-secret')
  })

  it('imports only the active models exposed by the GPT CYBER group', () => {
    const cyberModels = [
      'codex-auto-review',
      'gpt-5.4',
      'gpt-5.4-openai-compact',
      'gpt-5.5',
      'gpt-5.5-openai-compact',
      'gpt-5.6-sol',
      'gpt-5.6-sol-openai-compact',
      'gpt-5.6-terra',
      'gpt-5.6-terra-openai-compact',
      'gpt-daybreak-blue-latest'
    ]
    const url = parseDeepLink('codex', false, cyberModels)
    const config = JSON.parse(decodeBase64Utf8(url.searchParams.get('config')!))
    const importedModels = config.modelCatalog.models.map((model: { model: string }) => model.model)

    expect(importedModels).toEqual(cyberModels)
    expect(importedModels).not.toContain('gpt-5.6')
    expect(importedModels).not.toContain('gpt-5.6-luna')
    expect(importedModels).not.toContain('gpt-5.4-mini')
    expect(importedModels).not.toContain('gpt-5.3-codex-spark')
    expect(url.searchParams.get('model')).toBe('gpt-5.6-sol')
    expect(config.config).toContain('model = "gpt-5.6-sol"')
  })

  it('builds an explicit 1M / 900K high-context Codex profile', () => {
    const url = parseDeepLink('codex', false, ['gpt-5.6-sol'], 'long')
    const config = JSON.parse(decodeBase64Utf8(url.searchParams.get('config')!))

    expect(CODEX_LONG_CONTEXT_WINDOW_TOKENS).toBe(1000000)
    expect(CODEX_LONG_AUTO_COMPACT_TOKEN_LIMIT).toBe(900000)
    expect(config.config).toContain('model_context_window = 1000000')
    expect(config.config).toContain('model_auto_compact_token_limit = 900000')
    expect(url.searchParams.get('name')).toContain('1M')
    expect(config.modelCatalog.models).toEqual([{
      model: 'gpt-5.6-sol',
      displayName: 'GPT-5.6-Sol',
      contextWindow: 1000000
    }])

    const catalog = JSON.parse(buildCodexModelCatalog(OPENAI_CODEX_MODELS, 'long'))
    for (const model of catalog.models) {
      expect(model.context_window).toBe(1000000)
      expect(model.max_context_window).toBe(1000000)
      expect(model.effective_context_window_percent).toBe(100)
      expect(model.auto_compact_token_limit).toBe(900000)
    }
  })

  it('fails closed when an OpenAI group has no active models', () => {
    expect(() => parseDeepLink('codex', false, [])).toThrow(/at least one active model/)
  })

  it('keeps additive clients on their native standard-field contract', () => {
    expect(parseDeepLink('opencode').searchParams.get('config')).toBeNull()
  })

  it('builds Claude import only when Messages dispatch is enabled', () => {
    expect(() => parseDeepLink('claude')).toThrow(/not compatible/)

    const url = parseDeepLink('claude', true)
    expect(url.searchParams.get('app')).toBe('claude')
    expect(url.searchParams.get('endpoint')).toBe('https://api.laoshirenai.com')
    expect(url.searchParams.get('model')).toBe('claude-opus-5')
    expect(url.searchParams.get('haikuModel')).toBe('claude-haiku-4-5')
    expect(url.searchParams.get('sonnetModel')).toBe('claude-sonnet-5')
    expect(url.searchParams.get('opusModel')).toBe('claude-opus-5')

    const encodedConfig = url.searchParams.get('config')
    expect(encodedConfig).not.toBeNull()
    expect(url.searchParams.get('configFormat')).toBe('json')
    expect(JSON.parse(decodeBase64Utf8(encodedConfig!))).toEqual({
      env: {
        CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY: '1',
        CLAUDE_CODE_EFFORT_LEVEL: 'high'
      }
    })
  })

  it('adds Fable 5 only for Claude public groups whose complete pool supports it', () => {
    const url = new URL(buildCcsImportDeeplink({
      apiBaseUrl: 'https://api.laoshirenai.com',
      target: 'claude',
      key: {
        key: 'sk-test-not-a-secret',
        group: { id: 5, platform: 'anthropic' }
      }
    }))

    const config = JSON.parse(decodeBase64Utf8(url.searchParams.get('config')!))
    expect(config.env.ANTHROPIC_DEFAULT_FABLE_MODEL).toBe('claude-fable-5')
    expect(config.env.CLAUDE_CODE_EFFORT_LEVEL).toBe('high')
  })

  it('avoids duplicating /v1 when the public API base already includes it', () => {
    const url = new URL(buildCcsImportDeeplink({
      apiBaseUrl: 'https://api.laoshirenai.com/v1',
      target: 'hermes',
      key: {
        key: 'sk-test-not-a-secret',
        group: { platform: 'openai' }
      }
    }))

    expect(url.searchParams.get('endpoint')).toBe('https://api.laoshirenai.com/v1')
    expect(url.searchParams.get('usageBaseUrl')).toBe('https://api.laoshirenai.com')
  })

  it('rejects the lossy Grok Build deeplink contract in CC Switch 3.19.2', () => {
    expect(() => buildCcsImportDeeplink({
      apiBaseUrl: 'https://api.laoshirenai.com/v1',
      siteName: '老实人 AI',
      target: 'grokbuild',
      key: {
        key: 'test-grok-key-placeholder',
        group: { platform: 'grok', name: 'Grok 月卡' }
      }
    })).toThrow(/one-click compatibility setup/)
  })

  it('rejects Anthropic and generic OpenAI bridge targets for a Grok group', () => {
    for (const target of ['claude', 'codex', 'opencode', 'openclaw', 'hermes'] as CcsImportTarget[]) {
      expect(() => buildCcsImportDeeplink({
        apiBaseUrl: 'https://api.laoshirenai.com',
        target,
        key: {
          key: 'test-grok-key-placeholder',
          group: { platform: 'grok' }
        }
      })).toThrow(/not compatible/)
    }
  })

  it('never falls back to a single-model Grok Build deeplink', () => {
    expect(() => buildCcsImportDeeplink({
      apiBaseUrl: 'https://api.laoshirenai.com/v1/',
      target: 'grokbuild',
      key: {
        key: 'test-grok-key-placeholder',
        group: { platform: 'grok' }
      }
    })).toThrow(/one-click compatibility setup/)
  })
})
