import { describe, expect, it } from 'vitest'
import { hydrateAccountDraft, stableEqual } from '../AccountDraft'
import { switchAccountDraft } from '../accountPayload'

describe('AccountDraft', () => {
  it('hydrates defaults without sharing nested account state', () => {
    const source = {
      name: 'fixture',
      platform: 'anthropic' as const,
      type: 'apikey' as const,
      credentials: { model_mapping: { old: 'new' } },
      group_ids: [1]
    }
    const draft = hydrateAccountDraft(source)
    ;(draft.credentials.model_mapping as Record<string, string>).old = 'changed'
    draft.group_ids.push(2)

    expect(source.credentials.model_mapping.old).toBe('new')
    expect(source.group_ids).toEqual([1])
    expect(draft.concurrency).toBe(1)
  })

  it('compares records independent of object key order', () => {
    expect(stableEqual({ b: [2], a: 1 }, { a: 1, b: [2] })).toBe(true)
  })

  it('drops incompatible credentials and preserves common routing fields on platform switch', () => {
    const draft = hydrateAccountDraft({
      name: 'fixture',
      platform: 'anthropic',
      type: 'apikey',
      credentials: {
        api_key: 'fixture-value',
        aws_region: 'us-east-1',
        base_url: 'https://example.invalid',
        model_mapping: { a: 'b' }
      }
    })

    const switched = switchAccountDraft(draft, 'gemini', 'apikey')
    expect(switched.credentials).toEqual({
      base_url: 'https://example.invalid',
      model_mapping: { a: 'b' }
    })
  })
})
