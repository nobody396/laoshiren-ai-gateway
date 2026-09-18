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
    for (const [index, model] of catalog.models.entries()) {
      const nativeContext = OPENAI_CODEX_MODELS[index].contextWindow
      expect(model.context_window).toBe(nativeContext)
      expect(model.max_context_window).toBe(nativeContext)
      expect(model.effective_context_window_percent).toBe(CODEX_EFFECTIVE_CONTEXT_WINDOW_PERCENT)
      expect(model.auto_compact_token_limit).toBe(Math.floor(nativeContext * CODEX_EFFECTIVE_CONTEXT_WINDOW_PERCENT / 100))
    }
  })

  it('keeps each generated Codex model reasoning list instead of applying one global whitelist', () => {
    const catalog = JSON.parse(buildCodexModelCatalog())
    const levels = (model: string) => catalog.models
      .find((row: { slug: string }) => row.slug === model)
      .supported_reasoning_levels
      .map((row: { effort: string }) => row.effort)

    expect(levels('gpt-5.6-sol')).toEqual(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max', 'ultra'])
    expect(levels('gpt-5.3-codex-spark')).toEqual(['none'])
    expect(levels('qwen3.6-flash')).toEqual([])
    expect(levels('qwen3.8-max')).toEqual(['none', 'low', 'medium', 'xhigh'])
    // ultra is a Codex-only tier the gateway rewrites to max, so it is offered
    // on the three models Codex shows it on and nowhere else.
    expect(levels('gpt-6-astra')).toContain('ultra')
    expect(levels('gpt-5.6-terra')).toContain('ultra')
    expect(levels('gpt-5.6-luna')).not.toContain('ultra')
    expect(levels('qwen3.8-max')).not.toContain('ultra')
  })

  it.each(['codex', 'opencode', 'openclaw', 'hermes'] as CcsImportTarget[])(
    'builds an OpenAI-compatible %s provider with the /v1 endpoint',
    (target) => {
      const url = parseDeepLink(target)
      expect(url.searchParams.get('app')).toBe(target)
      expect(url.searchParams.get('endpoint')).toBe('https://api.laoshirenai.com/v1')
      expect(url.searchParams.get('model')).toBe('gpt-5.6-sol')
      expect(url.searchParams.get('usageBaseUrl')).toBe('https://api.laoshirenai.com')
      if (target === 'codex') expect(url.searchParams.get('name')).toContain('原生上下文')
    }
  )

  it('embeds the complete production Codex model catalog', () => {
    const url = parseDeepLink('codex')
    const encodedConfig = url.searchParams.get('config')
    expect(encodedConfig).not.toBeNull()
    expect(url.searchParams.get('configFormat')).toBe('json')

    const config = JSON.parse(decodeBase64Utf8(encodedConfig!))
    expect(config.modelCatalog.models).toHaveLength(OPENAI_CODEX_MODELS.length)
    OPENAI_CODEX_MODELS.forEach((generated, index) => {
      expect(config.modelCatalog.models[index]).toEqual({ ...generated, visibility: 'list' })
    })
    const importedModels = config.modelCatalog.models.map((model: { model: string }) => model.model)
    expect(importedModels).toEqual(OPENAI_CODEX_MODELS.map(model => model.model))
    expect(importedModels).toContain('gpt-5.6-luna')
    expect(importedModels).toContain('gpt-5.4-mini')
    expect(importedModels).toContain('gpt-5.3-codex-spark')
    expect(config.config).toContain('model = "gpt-5.6-sol"')
    expect(config.config).toContain('model_context_window = 1050000')
    expect(config.config).toContain('model_auto_compact_token_limit = 997500')
    expect(config.config).toContain('base_url = "https://api.laoshirenai.com/v1"')
    expect(config.config).not.toContain('sk-test-not-a-secret')
  })

  it('imports only the active models exposed by the GPT CYBER group', () => {
    const cyberModels = [
      'gpt-5.6-sol',
      'gpt-daybreak-blue-latest'
    ]
    const url = parseDeepLink('codex', false, cyberModels)
    const config = JSON.parse(decodeBase64Utf8(url.searchParams.get('config')!))
    const importedModels = config.modelCatalog.models.map((model: { model: string }) => model.model)

    expect(importedModels).toEqual(cyberModels)
    expect(importedModels).not.toContain('gpt-5.6')
    expect(importedModels).not.toContain('gpt-5.6-terra')
    expect(importedModels).not.toContain('gpt-5.3-codex-spark')
    expect(url.searchParams.get('model')).toBe('gpt-5.6-sol')
    expect(config.config).toContain('model = "gpt-5.6-sol"')
  })

  it('imports every GPT standard model including GPT-6 Astra', () => {
    const standardModels = [
      'gpt-6-astra',
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.5',
      'gpt-5.4',
      'gpt-5.3-codex-spark'
    ]
    const url = parseDeepLink('codex', false, standardModels)
    const config = JSON.parse(decodeBase64Utf8(url.searchParams.get('config')!))
    const importedModels = config.modelCatalog.models.map((model: { model: string }) => model.model)

    expect(importedModels).toEqual(standardModels)
    const astra = config.modelCatalog.models.find((model: { model: string }) => model.model === 'gpt-6-astra')
    expect(astra.displayName).toBe('GPT-6 Astra')
    expect(astra.contextWindow).toBe(1050000)
    expect(astra.visibility).toBe('list')
  })

  it('imports the full 8-model catalog for the CodeX enterprise group', () => {
    const enterpriseModels = [
      'gpt-5.4',
      'gpt-5.4-mini',
      'gpt-5.5',
      'gpt-5.6-luna',
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.3-codex-spark',
      'codex-auto-review'
    ]
    const url = parseDeepLink('codex', false, enterpriseModels)
    const config = JSON.parse(decodeBase64Utf8(url.searchParams.get('config')!))
    const importedModels = config.modelCatalog.models.map((model: { model: string }) => model.model)

    expect(importedModels).toEqual(enterpriseModels)
    // Every entry must be visible in the client selector, including
    // fallback-generated ones unknown to the generated catalog.
    for (const model of config.modelCatalog.models) {
      expect(model.visibility).toBe('list')
    }
    // Generated catalog entries keep their provider-owned display names.
    const sol = config.modelCatalog.models.find((model: { model: string }) => model.model === 'gpt-5.6-sol')
    expect(sol.displayName).toBe(OPENAI_CODEX_MODELS.find(model => model.model === 'gpt-5.6-sol')?.displayName)
    // Fallback entries derive a readable display name.
    const autoReview = config.modelCatalog.models.find((model: { model: string }) => model.model === 'codex-auto-review')
    expect(autoReview.displayName).toBe('Codex Auto Review')
    expect(url.searchParams.get('model')).toBe('gpt-5.6-sol')
    expect(config.config).toContain('model = "gpt-5.6-sol"')
  })

  it('keeps the legacy Pro 20X group catalog at exactly its five models', () => {
    const proModels = ['gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6', 'gpt-5.5', 'gpt-5.4']
    const url = parseDeepLink('codex', false, proModels)
    const config = JSON.parse(decodeBase64Utf8(url.searchParams.get('config')!))
    const importedModels = config.modelCatalog.models.map((model: { model: string }) => model.model)

    expect(importedModels).toEqual(proModels)
    expect(importedModels).not.toContain('gpt-5.6-luna')
    expect(importedModels).not.toContain('gpt-5.4-mini')
    expect(importedModels).not.toContain('gpt-5.3-codex-spark')
    expect(importedModels).not.toContain('codex-auto-review')
  })

  it('prefers the group default model when the catalog default is unavailable', () => {
    const deepLink = buildCcsImportDeeplink({
      apiBaseUrl: 'https://api.laoshirenai.com/',
      target: 'codex',
      availableModels: ['gpt-5.4', 'gpt-5.4-mini'],
      key: {
        key: 'sk-test-not-a-secret',
        name: '测试密钥',
        group: {
          platform: 'openai',
          name: 'CodeX 企业级分组',
          default_mapped_model: 'gpt-5.4-mini'
        }
      }
    })
    const url = new URL(deepLink)
    const config = JSON.parse(decodeBase64Utf8(url.searchParams.get('config')!))

    expect(url.searchParams.get('model')).toBe('gpt-5.4-mini')
    expect(config.config).toContain('model = "gpt-5.4-mini"')
  })

  it('builds an explicit 1M / 900K high-context Codex profile', () => {
    const url = parseDeepLink('codex', false, ['gpt-5.6-sol'], 'long')
    const config = JSON.parse(decodeBase64Utf8(url.searchParams.get('config')!))

    expect(CODEX_LONG_CONTEXT_WINDOW_TOKENS).toBe(1000000)
    expect(CODEX_LONG_AUTO_COMPACT_TOKEN_LIMIT).toBe(900000)
    expect(config.config).toContain('model_context_window = 1000000')
    expect(config.config).toContain('model_auto_compact_token_limit = 900000')
    expect(url.searchParams.get('name')).toContain('1M')
    const sol = OPENAI_CODEX_MODELS.find(model => model.model === 'gpt-5.6-sol')!
    expect(config.modelCatalog.models).toEqual([{
      ...sol,
      contextWindow: 1000000,
      visibility: 'list'
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

  it('uses the native Anthropic group default for the Claude main model and every role slot', () => {
    const url = new URL(buildCcsImportDeeplink({
      apiBaseUrl: 'https://api.laoshirenai.com',
      target: 'claude',
      key: {
        key: 'sk-test-not-a-secret',
        name: 'GLM 5.3',
        group: {
          platform: 'anthropic',
          name: 'GLM 分组',
          default_mapped_model: 'glm-5.3'
        }
      }
    }))

    expect(url.searchParams.get('model')).toBe('glm-5.3')
    expect(url.searchParams.get('haikuModel')).toBe('glm-5.3')
    expect(url.searchParams.get('sonnetModel')).toBe('glm-5.3')
    expect(url.searchParams.get('opusModel')).toBe('glm-5.3')
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
