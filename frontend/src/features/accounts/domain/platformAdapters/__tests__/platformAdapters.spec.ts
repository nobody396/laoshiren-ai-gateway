import { describe, expect, it } from 'vitest'
import { hydrateAccountDraft } from '../../AccountDraft'
import { getPlatformAdapter } from '../../accountPayload'

describe('platform adapter validation', () => {
  it('requires both Bedrock SigV4 credential components on create', () => {
    const draft = hydrateAccountDraft({
      platform: 'anthropic',
      type: 'bedrock',
      name: 'bedrock',
      credentials: { auth_mode: 'sigv4', aws_region: 'us-east-1' }
    })
    expect(getPlatformAdapter('anthropic', 'bedrock').validate(draft, 'create')).toEqual([
      { field: 'credentials.aws_access_key_id', code: 'required' },
      { field: 'credentials.aws_secret_access_key', code: 'required' }
    ])
  })

  it('keeps omitted secrets out of a full non-secret credential replacement', () => {
    const adapter = getPlatformAdapter('anthropic', 'bedrock')
    const original = hydrateAccountDraft({
      platform: 'anthropic',
      type: 'bedrock',
      name: 'bedrock',
      credentials: {
        auth_mode: 'sigv4',
        aws_region: 'us-east-1',
        aws_secret_access_key: 'fixture-value'
      }
    })
    const draft = hydrateAccountDraft({
      ...original,
      credentials: {
        ...original.credentials,
        aws_region: 'eu-west-1',
        aws_secret_access_key: ''
      }
    })

    expect(adapter.toUpdatePayload(draft, original).credentials).toEqual({
      auth_mode: 'sigv4',
      aws_region: 'eu-west-1'
    })
  })
})
