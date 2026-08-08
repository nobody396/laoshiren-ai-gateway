import type {
  Account,
  AccountPlatform,
  AccountType,
  CreateAccountRequest,
  UpdateAccountRequest
} from '@/types'

export type AccountPayload = CreateAccountRequest | (UpdateAccountRequest & {
  platform?: AccountPlatform
})

export interface AccountDraft {
  id?: number
  platform: AccountPlatform
  type: AccountType
  name: string
  notes: string | null
  credentials: Record<string, unknown>
  extra: Record<string, unknown>
  proxy_id: number | null
  concurrency: number
  load_factor: number | null
  priority: number
  rate_multiplier: number
  status?: Account['status']
  schedulable?: boolean
  group_ids: number[]
  expires_at: number | null
  auto_pause_on_expired: boolean
  confirm_mixed_channel_risk?: boolean
}

export interface DraftValidationError {
  field: string
  code: string
}

export const COMMON_CREDENTIAL_KEYS = new Set([
  'base_url',
  'model_mapping',
  'model_whitelist',
  'pool_mode',
  'pool_mode_retry_count',
  'custom_error_codes_enabled',
  'custom_error_codes',
  'intercept_warmup_requests',
  'temp_unschedulable_enabled',
  'temp_unschedulable_rules'
])

const SENSITIVE_KEY_PATTERN = /(^|_)(access_token|refresh_token|id_token|api_?key|password|secret|session_token|private_key|token)($|_)/i

export function isSensitiveCredentialKey(key: string): boolean {
  return SENSITIVE_KEY_PATTERN.test(key.replace(/-/g, '_'))
}

export function cloneAccountValue<T>(value: T): T {
  if (Array.isArray(value)) {
    return value.map((item) => cloneAccountValue(item)) as T
  }
  if (value instanceof Date) {
    return new Date(value.getTime()) as T
  }
  if (value !== null && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .map(([key, item]) => [key, cloneAccountValue(item)])
    ) as T
  }
  return value
}

export function cloneRecord(value: Record<string, unknown> | null | undefined): Record<string, unknown> {
  return value ? cloneAccountValue(value) : {}
}

export function hydrateAccountDraft(
  source: Partial<Account> & Partial<CreateAccountRequest>,
  fallback: Pick<AccountDraft, 'platform' | 'type'> = { platform: 'anthropic', type: 'oauth' }
): AccountDraft {
  return {
    id: source.id,
    platform: source.platform ?? fallback.platform,
    type: source.type ?? fallback.type,
    name: source.name ?? '',
    notes: source.notes ?? null,
    credentials: cloneRecord(source.credentials),
    extra: cloneRecord(source.extra),
    proxy_id: source.proxy_id ?? null,
    concurrency: source.concurrency ?? 1,
    load_factor: source.load_factor ?? null,
    priority: source.priority ?? 0,
    rate_multiplier: source.rate_multiplier ?? 1,
    status: source.status,
    schedulable: source.schedulable,
    group_ids: [...(source.group_ids ?? [])],
    expires_at: source.expires_at ?? null,
    auto_pause_on_expired: source.auto_pause_on_expired ?? true,
    confirm_mixed_channel_risk: source.confirm_mixed_channel_risk
  }
}

export function switchDraftPlatform(
  draft: AccountDraft,
  platform: AccountPlatform,
  type: AccountType,
  allowedPlatformKeys: ReadonlySet<string>
): AccountDraft {
  const credentials = Object.fromEntries(
    Object.entries(draft.credentials).filter(([key]) =>
      COMMON_CREDENTIAL_KEYS.has(key) ||
      (!isSensitiveCredentialKey(key) && allowedPlatformKeys.has(key))
    )
  )

  return {
    ...draft,
    platform,
    type,
    credentials
  }
}

export function stableEqual(left: unknown, right: unknown): boolean {
  if (Object.is(left, right)) return true
  if (typeof left !== typeof right || left === null || right === null) return false
  if (Array.isArray(left) || Array.isArray(right)) {
    return Array.isArray(left) && Array.isArray(right) &&
      left.length === right.length && left.every((value, index) => stableEqual(value, right[index]))
  }
  if (typeof left !== 'object') return false

  const leftRecord = left as Record<string, unknown>
  const rightRecord = right as Record<string, unknown>
  const leftKeys = Object.keys(leftRecord).sort()
  const rightKeys = Object.keys(rightRecord).sort()
  return stableEqual(leftKeys, rightKeys) &&
    leftKeys.every((key) => stableEqual(leftRecord[key], rightRecord[key]))
}
