import type { User } from '@/types'

export const AUTH_TOKEN_KEY = 'auth_token'
export const REFRESH_TOKEN_KEY = 'refresh_token'
export const TOKEN_EXPIRES_AT_KEY = 'token_expires_at'
export const AUTH_USER_KEY = 'auth_user'

export type SessionUser = User & { run_mode?: 'standard' | 'simple' }

export interface StoredAuthSession {
  accessToken: string | null
  refreshToken: string | null
  expiresAt: number | null
  user: SessionUser | null
}

export const EMPTY_STORED_SESSION: StoredAuthSession = {
  accessToken: null,
  refreshToken: null,
  expiresAt: null,
  user: null,
}

function availableStorage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}

export function readStoredSession(): StoredAuthSession {
  const storage = availableStorage()
  if (!storage) return { ...EMPTY_STORED_SESSION }
  const accessToken = storage.getItem(AUTH_TOKEN_KEY)
  const refreshToken = storage.getItem(REFRESH_TOKEN_KEY)
  const rawExpiresAt = storage.getItem(TOKEN_EXPIRES_AT_KEY)
  const rawUser = storage.getItem(AUTH_USER_KEY)
  try {
    const expiresAt = rawExpiresAt ? Number(rawExpiresAt) : null
    const user = rawUser ? JSON.parse(rawUser) as SessionUser : null
    if (rawExpiresAt && !Number.isFinite(expiresAt)) {
      throw new Error('incomplete stored session')
    }
    return { accessToken, refreshToken, expiresAt, user }
  } catch {
    clearStoredSession()
    return { ...EMPTY_STORED_SESSION }
  }
}

export function writeStoredSession(next: StoredAuthSession): void {
  const storage = availableStorage()
  if (!storage) return
  const keys = [AUTH_TOKEN_KEY, REFRESH_TOKEN_KEY, TOKEN_EXPIRES_AT_KEY, AUTH_USER_KEY]
  const before = new Map(keys.map((key) => [key, storage.getItem(key)]))
  try {
    setOrRemove(storage, REFRESH_TOKEN_KEY, next.refreshToken)
    setOrRemove(storage, TOKEN_EXPIRES_AT_KEY, next.expiresAt == null ? null : String(next.expiresAt))
    setOrRemove(storage, AUTH_USER_KEY, next.user ? JSON.stringify(next.user) : null)
    // Publish the access token last so readers never observe it with stale
    // rotating-token or user metadata.
    setOrRemove(storage, AUTH_TOKEN_KEY, next.accessToken)
  } catch (error) {
    for (const key of keys) setOrRemove(storage, key, before.get(key) ?? null)
    throw error
  }
}

export function clearStoredSession(): void {
  const storage = availableStorage()
  if (!storage) return
  storage.removeItem(AUTH_TOKEN_KEY)
  storage.removeItem(REFRESH_TOKEN_KEY)
  storage.removeItem(TOKEN_EXPIRES_AT_KEY)
  storage.removeItem(AUTH_USER_KEY)
}

function setOrRemove(storage: Storage, key: string, value: string | null): void {
  if (value == null || value === '') storage.removeItem(key)
  else storage.setItem(key, value)
}
