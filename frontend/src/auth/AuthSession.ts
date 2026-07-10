import type { AuthResponse, User } from '@/types'
import {
  clearStoredSession,
  EMPTY_STORED_SESSION,
  readStoredSession,
  writeStoredSession,
  type StoredAuthSession,
} from './sessionStorage'

const TOKEN_REFRESH_BUFFER_MS = 120_000

export interface RefreshResult {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type?: string
}

export type RefreshTransport = (refreshToken: string, signal: AbortSignal) => Promise<RefreshResult>
export type AuthSessionEvent = { type: 'session-expired' }

export class AuthSessionChangedError extends Error {
  constructor() {
    super('Auth session changed while refresh was in flight')
    this.name = 'AuthSessionChangedError'
  }
}

type SnapshotListener = (snapshot: Readonly<StoredAuthSession>) => void
type EventListener = (event: AuthSessionEvent) => void
type RefreshedListener = () => void | Promise<void>

export class AuthSession {
  private current: StoredAuthSession = { ...EMPTY_STORED_SESSION }
  private epoch = 0
  private refreshPromise: Promise<RefreshResult> | null = null
  private refreshController: AbortController | null = null
  private refreshTransport: RefreshTransport | null = null
  private refreshTimer: ReturnType<typeof setTimeout> | null = null
  private snapshotListener: SnapshotListener | null = null
  private eventListener: EventListener | null = null
  private refreshedListener: RefreshedListener | null = null

  constructor(
    private readonly now: () => number = () => Date.now(),
    private readonly schedule?: typeof setTimeout,
    private readonly cancelSchedule?: typeof clearTimeout,
  ) {}

  get snapshot(): Readonly<StoredAuthSession> {
    return this.current
  }

  get accessToken(): string | null {
    return this.current.accessToken
  }

  get refreshToken(): string | null {
    return this.current.refreshToken
  }

  configureRefresh(transport: RefreshTransport): void {
    this.refreshTransport = transport
    this.scheduleRefresh()
  }

  bindSnapshot(listener: SnapshotListener): void {
    this.snapshotListener = listener
    listener(this.current)
  }

  bindEvents(listener: EventListener): void {
    this.eventListener = listener
  }

  bindRefreshed(listener: RefreshedListener): void {
    this.refreshedListener = listener
  }

  hydrate(): Readonly<StoredAuthSession> {
    const stored = readStoredSession()
    if (!sameSession(this.current, stored)) {
      this.replace(stored, { persist: false, invalidate: true })
    } else {
      this.scheduleRefresh()
    }
    return this.current
  }

  setAuthenticated(response: AuthResponse, userOverride?: User): void {
    const user = userOverride ?? response.user
    this.replace({
      accessToken: response.access_token,
      refreshToken: response.refresh_token ?? null,
      expiresAt: response.expires_in ? this.now() + response.expires_in * 1000 : null,
      user,
    }, { persist: true, invalidate: true })
  }

  adoptTokens(accessToken: string, refreshToken: string | null, expiresIn?: number): void {
    this.replace({
      accessToken,
      refreshToken,
      expiresAt: expiresIn ? this.now() + expiresIn * 1000 : null,
      user: null,
    }, { persist: true, invalidate: true })
  }

  updateUser(user: User): void {
    this.replace({ ...this.current, user }, { persist: true, invalidate: false })
  }

  clear(reason: 'logout' | 'expired' | 'replace' = 'replace'): boolean {
    const hadSession = !!(this.current.accessToken || this.current.refreshToken || this.current.user)
    this.invalidateInFlight()
    this.cancelTimer()
    clearStoredSession()
    this.current = { ...EMPTY_STORED_SESSION }
    this.snapshotListener?.(this.current)
    if (reason === 'expired' && hadSession) this.eventListener?.({ type: 'session-expired' })
    return hadSession
  }

  refreshOnce(): Promise<RefreshResult> {
    if (this.refreshPromise) return this.refreshPromise
    const refreshToken = this.current.refreshToken
    const transport = this.refreshTransport
    if (!refreshToken || !transport) {
      return Promise.reject(new Error('No refresh token available'))
    }

    const epoch = this.epoch
    const controller = new AbortController()
    this.refreshController = controller
    const operation = (async () => {
      try {
        const result = await transport(refreshToken, controller.signal)
        if (controller.signal.aborted || epoch !== this.epoch || refreshToken !== this.current.refreshToken) {
          throw new AuthSessionChangedError()
        }
        this.replace({
          ...this.current,
          accessToken: result.access_token,
          refreshToken: result.refresh_token,
          expiresAt: this.now() + result.expires_in * 1000,
        }, { persist: true, invalidate: false })
        try {
          await this.refreshedListener?.()
        } catch {
          // Token rotation already committed. A profile projection failure must
          // not invalidate the otherwise valid new rotating-token family.
        }
        return result
      } catch (error) {
        if (controller.signal.aborted || epoch !== this.epoch) {
          throw new AuthSessionChangedError()
        }
        this.clear('expired')
        throw error
      }
    })()
    this.refreshPromise = operation
    const cleanup = () => {
      if (this.refreshPromise === operation) this.refreshPromise = null
      if (this.refreshController === controller) this.refreshController = null
    }
    void operation.then(cleanup, cleanup)
    return operation
  }

  private replace(next: StoredAuthSession, options: { persist: boolean; invalidate: boolean }): void {
    if (options.invalidate) this.invalidateInFlight()
    if (options.persist) writeStoredSession(next)
    this.current = { ...next }
    this.snapshotListener?.(this.current)
    this.scheduleRefresh()
  }

  private invalidateInFlight(): void {
    this.epoch += 1
    this.refreshController?.abort()
    this.refreshController = null
    this.refreshPromise = null
  }

  private scheduleRefresh(): void {
    this.cancelTimer()
    if (!this.refreshTransport || !this.current.refreshToken || this.current.expiresAt == null) return
    const delay = Math.max(0, this.current.expiresAt - this.now() - TOKEN_REFRESH_BUFFER_MS)
    this.refreshTimer = (this.schedule ?? globalThis.setTimeout)(() => {
      this.refreshTimer = null
      void this.refreshOnce().catch(() => undefined)
    }, delay)
  }

  private cancelTimer(): void {
    if (this.refreshTimer == null) return
    ;(this.cancelSchedule ?? globalThis.clearTimeout)(this.refreshTimer)
    this.refreshTimer = null
  }
}

function sameSession(a: StoredAuthSession, b: StoredAuthSession): boolean {
  return a.accessToken === b.accessToken && a.refreshToken === b.refreshToken &&
    a.expiresAt === b.expiresAt && JSON.stringify(a.user) === JSON.stringify(b.user)
}
