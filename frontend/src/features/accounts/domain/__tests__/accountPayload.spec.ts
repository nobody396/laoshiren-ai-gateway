import { describe, expect, it } from 'vitest'
import type { Account, AccountPlatform, AccountType, CreateAccountRequest } from '@/types'
import {
  getPlatformAdapter,
  toBulkAccountPatch,
  toCreateAccountPayload,
  toUpdateAccountPayload,
  validateAccountDraft
} from '../accountPayload'
import { hydrateAccountDraft } from '../AccountDraft'

const cases: Array<[AccountPlatform, AccountType]> = [
  ['anthropic', 'oauth'],
  ['anthropic', 'setup-token'],
  ['anthropic', 'apikey'],
  ['anthropic', 'bedrock'],
  ['openai', 'oauth'],
  ['openai', 'apikey'],
  ['gemini', 'oauth'],
  ['gemini', 'apikey'],
  ['antigravity', 'oauth'],
  ['antigravity', 'apikey'],
  ['antigravity', 'upstream'],
  ['gpt-image', 'upstream']
]

function createInput(platform: AccountPlatform, type: AccountType): CreateAccountRequest {
  const credentials: Record<string, unknown> = {
    base_url: 'https://example.invalid',
    api_key: 'fixture-value'
  }
  if (type === 'bedrock') {
    credentials.auth_mode = 'api_key'
    credentials.aws_region = 'us-east-1'
  }
  if (type === 'setup-token') credentials.setup_token = 'fixture-value'
  return {
    name: `${platform}-${type}`,
    platform,
    type,
    credentials,
    group_ids: [7]
  }
}

function accountFrom(input: CreateAccountRequest): Account {
  return {
    ...input,
    id: 10,
    notes: input.notes ?? null,
    proxy_id: input.proxy_id ?? null,
    concurrency: input.concurrency ?? 1,
    priority: input.priority ?? 0,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-07-11T00:00:00Z',
    updated_at: '2026-07-11T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null
  }
}

describe.each(cases)('%s/%s adapter', (platform, type) => {
  it('normalizes create, edit and bulk through the selected adapter', () => {
    const input = createInput(platform, type)
    const create = toCreateAccountPayload(input)
    expect(create.platform).toBe(platform)
    expect(create.type).toBe(type)

    const account = accountFrom(create)
    expect(toUpdateAccountPayload(account, {})).toEqual({})
    expect(toUpdateAccountPayload(account, { priority: 9 })).toEqual({ priority: 9 })
    expect(getPlatformAdapter(platform, type).toBulkPatch({ priority: 8 })).toEqual({ priority: 8 })
  })
})

describe('account payload invariants', () => {
  it('does not overwrite a stored secret with a blank edit value', () => {
    const original = accountFrom(createInput('openai', 'apikey'))
    const update = toUpdateAccountPayload(original, {
      credentials: { ...original.credentials, api_key: '', base_url: 'https://changed.invalid' }
    })

    expect(update.credentials).toEqual({ base_url: 'https://changed.invalid' })
  })

  it('returns no fields for an unchanged draft', () => {
    const original = accountFrom(createInput('gemini', 'apikey'))
    expect(toUpdateAccountPayload(original, {
      credentials: structuredClone(original.credentials),
      extra: {}
    })).toEqual({})
  })

  it('removes blank secret fields from bulk patches', () => {
    expect(toBulkAccountPatch(['openai', 'anthropic'], ['apikey'], {
      credentials: { api_key: ' ', base_url: 'https://example.invalid' }
    })).toEqual({ credentials: { base_url: 'https://example.invalid' } })
  })

  it('uses consistent name validation in create and edit flows', () => {
    const draft = hydrateAccountDraft({ platform: 'openai', type: 'oauth', name: ' ' })
    expect(validateAccountDraft(draft, 'create')).toContainEqual({ field: 'name', code: 'required' })
    expect(validateAccountDraft(draft, 'edit')).toEqual([{ field: 'name', code: 'required' }])
    expect(validateAccountDraft(draft, 'bulk')).toEqual([])
  })
})
