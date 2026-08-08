import type { CreateAccountRequest, UpdateAccountRequest } from '@/types'
import {
  cloneRecord,
  cloneAccountValue,
  hydrateAccountDraft,
  isSensitiveCredentialKey,
  stableEqual,
  switchDraftPlatform,
  type AccountDraft,
  type DraftValidationError
} from '../AccountDraft'

export interface PlatformAccountAdapter {
  readonly platformKeys: ReadonlySet<string>
  hydrate(source: Parameters<typeof hydrateAccountDraft>[0]): AccountDraft
  validate(draft: AccountDraft, mode: 'create' | 'edit' | 'bulk'): DraftValidationError[]
  toCreatePayload(draft: AccountDraft): CreateAccountRequest
  toUpdatePayload(draft: AccountDraft, original: AccountDraft): UpdateAccountRequest
  toBulkPatch(patch: UpdateAccountRequest): UpdateAccountRequest
  switchFrom(draft: AccountDraft, platform: AccountDraft['platform'], type: AccountDraft['type']): AccountDraft
}

interface AdapterConfig {
  platformKeys: string[]
  requiredSecrets: Partial<Record<AccountDraft['type'], string[]>>
  validate?: (draft: AccountDraft, mode: 'create' | 'edit' | 'bulk') => DraftValidationError[]
}

const COMMON_FIELDS: Array<keyof UpdateAccountRequest> = [
  'name', 'notes', 'type', 'proxy_id', 'concurrency', 'load_factor', 'priority',
  'rate_multiplier', 'schedulable', 'status', 'group_ids', 'expires_at',
  'auto_pause_on_expired', 'confirm_mixed_channel_risk'
]

function nonBlank(value: unknown): boolean {
  return typeof value === 'string' ? value.trim().length > 0 : value !== undefined && value !== null
}

function sanitizeCredentials(credentials: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(credentials).filter(([key, value]) => {
    if (!isSensitiveCredentialKey(key)) return true
    return nonBlank(value)
  }))
}

function baseValidation(draft: AccountDraft, mode: 'create' | 'edit' | 'bulk'): DraftValidationError[] {
  if (mode === 'bulk') return []
  return draft.name.trim() ? [] : [{ field: 'name', code: 'required' }]
}

export function createPlatformAdapter(config: AdapterConfig): PlatformAccountAdapter {
  const platformKeys = new Set(config.platformKeys)

  return {
    platformKeys,
    hydrate(source) {
      return hydrateAccountDraft(source)
    },
    validate(draft, mode) {
      const errors = baseValidation(draft, mode)
      if (mode === 'create') {
        for (const key of config.requiredSecrets[draft.type] ?? []) {
          if (!nonBlank(draft.credentials[key])) errors.push({ field: `credentials.${key}`, code: 'required' })
        }
      }
      return [...errors, ...(config.validate?.(draft, mode) ?? [])]
    },
    toCreatePayload(draft) {
      return {
        name: draft.name.trim(),
        notes: draft.notes,
        platform: draft.platform,
        type: draft.type,
        credentials: sanitizeCredentials(draft.credentials),
        extra: Object.keys(draft.extra).length ? cloneRecord(draft.extra) : undefined,
        proxy_id: draft.proxy_id,
        concurrency: draft.concurrency,
        load_factor: draft.load_factor,
        priority: draft.priority,
        rate_multiplier: draft.rate_multiplier,
        group_ids: [...draft.group_ids],
        expires_at: draft.expires_at,
        auto_pause_on_expired: draft.auto_pause_on_expired,
        confirm_mixed_channel_risk: draft.confirm_mixed_channel_risk
      }
    },
    toUpdatePayload(draft, original) {
      const payload: UpdateAccountRequest = {}
      const mutablePayload = payload as Record<string, unknown>
      for (const field of COMMON_FIELDS) {
        const current = draft[field as keyof AccountDraft]
        const previous = original[field as keyof AccountDraft]
        if (!stableEqual(current, previous) && current !== undefined) {
          mutablePayload[field] = cloneAccountValue(current)
        }
      }
      if (!stableEqual(draft.credentials, original.credentials)) {
        payload.credentials = sanitizeCredentials(draft.credentials)
      }
      if (!stableEqual(draft.extra, original.extra)) {
        payload.extra = cloneRecord(draft.extra)
      }
      return payload
    },
    toBulkPatch(patch) {
      const payload = cloneAccountValue(patch) as UpdateAccountRequest
      if (payload.credentials) payload.credentials = sanitizeCredentials(payload.credentials)
      return payload
    },
    switchFrom(draft, platform, type) {
      return switchDraftPlatform(draft, platform, type, platformKeys)
    }
  }
}
