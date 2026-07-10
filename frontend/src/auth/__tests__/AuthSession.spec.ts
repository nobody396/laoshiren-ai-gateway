import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AuthResponse, User } from '@/types'
import { AuthSession, AuthSessionChangedError, type RefreshResult } from '../AuthSession'

const user = (id: number): User => ({
  id,
  username: `user-${id}`,
  email: `user-${id}@example.com`,
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: null,
  created_at: '2026-01-01',
  updated_at: '2026-01-01',
})

const response = (id: number, access = `access-${id}`, refresh = `refresh-${id}`): AuthResponse => ({
  access_token: access,
  refresh_token: refresh,
  expires_in: 3600,
  token_type: 'Bearer',
  user: user(id),
})

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

describe('AuthSession', () => {
  beforeEach(() => localStorage.clear())

  it('hydrates the legacy storage keys without logging out existing users', () => {
    localStorage.setItem('auth_token', 'saved-access')
    localStorage.setItem('refresh_token', 'saved-refresh')
    localStorage.setItem('token_expires_at', '12345')
    localStorage.setItem('auth_user', JSON.stringify(user(1)))
    const session = new AuthSession(() => 1000)

    const snapshot = session.hydrate()

    expect(snapshot.accessToken).toBe('saved-access')
    expect(snapshot.refreshToken).toBe('saved-refresh')
    expect(snapshot.expiresAt).toBe(12345)
    expect(snapshot.user).toEqual(user(1))
  })

  it('coalesces concurrent refreshes and atomically rotates storage and snapshot', async () => {
    const session = new AuthSession(() => 1000)
    session.setAuthenticated(response(1))
    const gate = deferred<RefreshResult>()
    const transport = vi.fn(() => gate.promise)
    const snapshots: Array<string | null> = []
    session.configureRefresh(transport)
    session.bindSnapshot((snapshot) => snapshots.push(snapshot.accessToken))

    const calls = Array.from({ length: 10 }, () => session.refreshOnce())
    expect(transport).toHaveBeenCalledTimes(1)
    gate.resolve({ access_token: 'rotated-access', refresh_token: 'rotated-refresh', expires_in: 600 })
    const results = await Promise.all(calls)

    expect(results.every((item) => item.access_token === 'rotated-access')).toBe(true)
    expect(session.snapshot.accessToken).toBe('rotated-access')
    expect(localStorage.getItem('auth_token')).toBe('rotated-access')
    expect(localStorage.getItem('refresh_token')).toBe('rotated-refresh')
    expect(localStorage.getItem('token_expires_at')).toBe(String(601000))
    expect(snapshots.at(-1)).toBe('rotated-access')
  })

  it('shares one refresh between the proactive timer and a reactive caller', async () => {
    const scheduled: Array<() => void> = []
    const schedule = ((callback: () => void) => {
      scheduled.push(callback)
      return 1 as unknown as ReturnType<typeof setTimeout>
    }) as typeof setTimeout
    const session = new AuthSession(() => 1000, schedule, (() => undefined) as typeof clearTimeout)
    const gate = deferred<RefreshResult>()
    const transport = vi.fn(() => gate.promise)
    session.configureRefresh(transport)
    session.setAuthenticated({ ...response(1), expires_in: 121 })
    expect(scheduled).toHaveLength(1)

    scheduled[0]()
    const reactive = session.refreshOnce()
    expect(transport).toHaveBeenCalledTimes(1)
    gate.resolve({ access_token: 'shared', refresh_token: 'shared-refresh', expires_in: 600 })
    await reactive
    expect(session.accessToken).toBe('shared')
  })

  it('clears and emits expiry once when refresh fails', async () => {
    const session = new AuthSession()
    session.setAuthenticated(response(1))
    const gate = deferred<RefreshResult>()
    session.configureRefresh(() => gate.promise)
    const events = vi.fn()
    session.bindEvents(events)
    const first = session.refreshOnce()
    const second = session.refreshOnce()
    gate.reject(new Error('refresh failed'))
    const results = await Promise.allSettled([first, second])

    expect(results.every((item) => item.status === 'rejected')).toBe(true)
    expect(events).toHaveBeenCalledTimes(1)
    expect(session.accessToken).toBeNull()
    expect(localStorage.getItem('auth_token')).toBeNull()
  })

  it('keeps a committed rotation when the profile projection refresh fails', async () => {
    const session = new AuthSession()
    session.setAuthenticated(response(1))
    session.configureRefresh(async () => ({
      access_token: 'rotated', refresh_token: 'rotated-refresh', expires_in: 600,
    }))
    session.bindRefreshed(async () => { throw new Error('profile unavailable') })

    await session.refreshOnce()

    expect(session.accessToken).toBe('rotated')
    expect(localStorage.getItem('refresh_token')).toBe('rotated-refresh')
  })

  it('cannot resurrect a logged-out session from an in-flight refresh', async () => {
    const session = new AuthSession()
    session.setAuthenticated(response(1))
    const gate = deferred<RefreshResult>()
    session.configureRefresh(() => gate.promise)
    const pending = session.refreshOnce()
    session.clear('logout')
    gate.resolve({ access_token: 'stale', refresh_token: 'stale-refresh', expires_in: 600 })
    const [result] = await Promise.allSettled([pending])

    expect(result.status).toBe('rejected')
    if (result.status === 'rejected') expect(result.reason).toBeInstanceOf(AuthSessionChangedError)
    expect(session.accessToken).toBeNull()
  })

  it('cannot overwrite a newer user with user A refresh results', async () => {
    const session = new AuthSession()
    session.setAuthenticated(response(1))
    const gate = deferred<RefreshResult>()
    session.configureRefresh(() => gate.promise)
    const pending = session.refreshOnce()
    session.setAuthenticated(response(2))
    gate.resolve({ access_token: 'stale-a', refresh_token: 'stale-a-refresh', expires_in: 600 })
    await Promise.allSettled([pending])

    expect(session.snapshot.user?.id).toBe(2)
    expect(session.accessToken).toBe('access-2')
    expect(localStorage.getItem('auth_token')).toBe('access-2')
  })
})
