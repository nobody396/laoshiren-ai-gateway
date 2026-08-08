import { describe, expect, it } from 'vitest'

import {
  buildCcsImportDeeplink,
  getCompatibleCcsTargets,
  type CcsImportTarget
} from '@/utils/ccSwitchImport'

const parseDeepLink = (target: CcsImportTarget, allowMessagesDispatch = false) => {
  const deepLink = buildCcsImportDeeplink({
    apiBaseUrl: 'https://api.laoshirenai.com/',
    siteName: '老实人 AI',
    target,
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
  it.each(['codex', 'opencode', 'openclaw', 'hermes'] as CcsImportTarget[])(
    'builds an OpenAI-compatible %s provider with the /v1 endpoint',
    (target) => {
      const url = parseDeepLink(target)
      expect(url.searchParams.get('app')).toBe(target)
      expect(url.searchParams.get('endpoint')).toBe('https://api.laoshirenai.com/v1')
      expect(url.searchParams.get('model')).toBe('gpt-5.6-sol')
      expect(url.searchParams.get('usageBaseUrl')).toBe('https://api.laoshirenai.com')
    }
  )

  it('lets CC Switch build native app configuration from standard fields', () => {
    expect(parseDeepLink('codex').searchParams.get('config')).toBeNull()
    expect(parseDeepLink('opencode').searchParams.get('config')).toBeNull()
  })

  it('builds Claude import only when Messages dispatch is enabled', () => {
    expect(() => parseDeepLink('claude')).toThrow(/not compatible/)

    const url = parseDeepLink('claude', true)
    expect(url.searchParams.get('app')).toBe('claude')
    expect(url.searchParams.get('endpoint')).toBe('https://api.laoshirenai.com')
    expect(url.searchParams.get('model')).toBe('claude-opus-5')
    expect(url.searchParams.get('sonnetModel')).toBe('claude-sonnet-4-6[1M]')
    expect(url.searchParams.get('opusModel')).toBe('claude-opus-5[1M]')
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

  it('builds the native Grok Build provider contract supported by CC Switch', () => {
    const url = new URL(buildCcsImportDeeplink({
      apiBaseUrl: 'https://api.laoshirenai.com/v1',
      siteName: '老实人 AI',
      target: 'grokbuild',
      key: {
        key: 'test-grok-key-placeholder',
        group: { platform: 'grok', name: 'Grok 月卡' }
      }
    }))

    expect(url.searchParams.get('resource')).toBe('provider')
    expect(url.searchParams.get('app')).toBe('grokbuild')
    expect(url.searchParams.get('endpoint')).toBe('https://api.laoshirenai.com/v1')
    expect(url.searchParams.get('model')).toBe('grok-4.5')
    expect(url.searchParams.get('name')).toContain('Grok Build')
    expect(url.searchParams.get('haikuModel')).toBeNull()
    expect(url.searchParams.get('sonnetModel')).toBeNull()
    expect(url.searchParams.get('opusModel')).toBeNull()
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

  it('avoids duplicating /v1 for a native Grok Build provider', () => {
    const url = new URL(buildCcsImportDeeplink({
      apiBaseUrl: 'https://api.laoshirenai.com/v1/',
      target: 'grokbuild',
      key: {
        key: 'test-grok-key-placeholder',
        group: { platform: 'grok' }
      }
    }))

    expect(url.searchParams.get('endpoint')).toBe('https://api.laoshirenai.com/v1')
    expect(url.searchParams.get('usageBaseUrl')).toBe('https://api.laoshirenai.com')
  })
})
