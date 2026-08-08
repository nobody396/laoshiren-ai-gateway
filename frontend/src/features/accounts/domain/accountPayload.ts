import type { Account, AccountPlatform, AccountType, CreateAccountRequest, UpdateAccountRequest } from '@/types'
import { cloneAccountValue, hydrateAccountDraft, type AccountDraft, type DraftValidationError } from './AccountDraft'
import { antigravityAccountAdapter } from './platformAdapters/antigravity'
import { anthropicAccountAdapter } from './platformAdapters/anthropic'
import { bedrockAccountAdapter } from './platformAdapters/bedrock'
import { geminiAccountAdapter } from './platformAdapters/gemini'
import { openAIAccountAdapter } from './platformAdapters/openai'
import type { PlatformAccountAdapter } from './platformAdapters/shared'

const adapters: Record<AccountPlatform, PlatformAccountAdapter> = {
  anthropic: anthropicAccountAdapter,
  openai: openAIAccountAdapter,
  gemini: geminiAccountAdapter,
  antigravity: antigravityAccountAdapter,
  'gpt-image': openAIAccountAdapter,
  grok: openAIAccountAdapter
}

export function getPlatformAdapter(platform: AccountPlatform, type: AccountType): PlatformAccountAdapter {
  if (type === 'bedrock') return bedrockAccountAdapter
  return adapters[platform]
}

export function validateAccountDraft(
  draft: AccountDraft,
  mode: 'create' | 'edit' | 'bulk'
): DraftValidationError[] {
  return getPlatformAdapter(draft.platform, draft.type).validate(draft, mode)
}

export function switchAccountDraft(
  draft: AccountDraft,
  platform: AccountPlatform,
  type: AccountType
): AccountDraft {
  return getPlatformAdapter(platform, type).switchFrom(draft, platform, type)
}

export function toCreateAccountPayload(payload: CreateAccountRequest): CreateAccountRequest {
  const draft = hydrateAccountDraft(payload)
  return getPlatformAdapter(draft.platform, draft.type).toCreatePayload(draft)
}

export function toUpdateAccountPayload(account: Account, changes: UpdateAccountRequest): UpdateAccountRequest {
  const original = hydrateAccountDraft(account)
  const draft = hydrateAccountDraft({
    ...account,
    ...changes,
    credentials: changes.credentials ?? account.credentials,
    extra: changes.extra ?? account.extra,
    group_ids: changes.group_ids ?? account.group_ids
  })
  return getPlatformAdapter(draft.platform, draft.type).toUpdatePayload(draft, original)
}

export function toBulkAccountPatch(
  platforms: AccountPlatform[],
  types: AccountType[],
  patch: UpdateAccountRequest
): UpdateAccountRequest {
  if (platforms.length === 0) return cloneAccountValue(patch)
  const normalized = platforms.map((platform) =>
    getPlatformAdapter(platform, types.length === 1 ? types[0] : 'oauth').toBulkPatch(patch)
  )
  const first = normalized[0]
  if (normalized.every((candidate) => JSON.stringify(candidate) === JSON.stringify(first))) return first

  const common = cloneAccountValue(first) as Record<string, unknown>
  for (const key of Object.keys(common)) {
    if (!normalized.every((candidate) => JSON.stringify((candidate as Record<string, unknown>)[key]) === JSON.stringify(common[key]))) {
      delete common[key]
    }
  }
  return common as UpdateAccountRequest
}
