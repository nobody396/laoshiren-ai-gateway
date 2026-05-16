import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'

// Mock authAPI
const mockLogin = vi.fn()
const mockLogin2FA = vi.fn()
const mockLogout = vi.fn()
const mockGetCurrentUser = vi.fn()
const mockRegister = vi.fn()
const mockRefreshToken = vi.fn()
const mockGetMyPermissions = vi.fn()
const mockGetMyMenuTree = vi.fn()

vi.mock('@/api', () => ({
  authAPI: {
    login: (...args: any[]) => mockLogin(...args),
    login2FA: (...args: any[]) => mockLogin2FA(...args),
    logout: (...args: any[]) => mockLogout(...args),
    getCurrentUser: (...args: any[]) => mockGetCurrentUser(...args),
    register: (...args: any[]) => mockRegister(...args),
    refreshToken: (...args: any[]) => mockRefreshToken(...args),
  },
  isTotp2FARequired: (response: any) => response?.requires_2fa === true,
}))

vi.mock('@/api/admin/rbac', () => ({
  default: {
    getMyPermissions: (...args: any[]) => mockGetMyPermissions(...args),
    getMyMenuTree: (...args: any[]) => mockGetMyMenuTree(...args),
  },
}))

const fakeUser = {
  id: 1,
  username: 'testuser',
  email: 'test@example.com',
  role: 'user' as const,
  balance: 100,
  concurrency: 5,
  status: 'active' as const,
  allowed_groups: null,
  created_at: '2024-01-01',
  updated_at: '2024-01-01',
}

const fakeAdminUser = {
  ...fakeUser,
  id: 2,
  username: 'admin',
  email: 'admin@example.com',
  role: 'admin' as const,
}

const fakeAuthResponse = {
  access_token: 'test-token-123',
  refresh_token: 'refresh-token-456',
  expires_in: 3600,
  token_type: 'Bearer',
  user: { ...fakeUser },
}

describe('useAuthStore', () => {
  const ensureStorage = () => {
    const g = globalThis as typeof globalThis & { localStorage?: Storage }
    const current = g.localStorage as (Storage & Record<string, any>) | undefined
    if (
      current &&
      typeof current.getItem === 'function' &&
      typeof current.setItem === 'function' &&
      typeof current.removeItem === 'function' &&
      typeof current.clear === 'function'
    ) {
      return current
    }

    const store = new Map<string, string>()
    const mockStorage: Storage = {
      get length() {
        return store.size
      },
      clear() {
        store.clear()
      },
      getItem(key: string) {
        return store.has(key) ? store.get(key)! : null
      },
      key(index: number) {
        return Array.from(store.keys())[index] ?? null
      },
      removeItem(key: string) {
        store.delete(key)
      },
      setItem(key: string, value: string) {
        store.set(key, String(value))
      }
    }

    Object.defineProperty(globalThis, 'localStorage', {
      value: mockStorage,
      configurable: true,
      writable: true
    })

    return mockStorage
  }

  beforeEach(() => {
    setActivePinia(createPinia())
    ensureStorage().clear()
    vi.useFakeTimers()
    vi.clearAllMocks()
    mockGetMyPermissions.mockResolvedValue([])
    mockGetMyMenuTree.mockResolvedValue([])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // --- login ---

  describe('login', () => {
    it('成功登录后设置 token 和 user', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.token).toBe('test-token-123')
      expect(store.user).toEqual(fakeUser)
      expect(store.isAuthenticated).toBe(true)
      expect(localStorage.getItem('auth_token')).toBe('test-token-123')
      expect(localStorage.getItem('auth_user')).toBe(JSON.stringify(fakeUser))
    })

    it('登录失败时清除状态并抛出错误', async () => {
      mockLogin.mockRejectedValue(new Error('Invalid credentials'))
      const store = useAuthStore()

      await expect(store.login({ email: 'test@example.com', password: 'wrong' })).rejects.toThrow(
        'Invalid credentials'
      )

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('需要 2FA 时返回响应但不设置认证状态', async () => {
      const twoFAResponse = { requires_2fa: true, temp_token: 'temp-123' }
      mockLogin.mockResolvedValue(twoFAResponse)
      const store = useAuthStore()

      const result = await store.login({ email: 'test@example.com', password: '123456' })

      expect(result).toEqual(twoFAResponse)
      expect(store.token).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('登录新用户前清除旧管理员 refresh token', async () => {
      localStorage.setItem('auth_token', 'old-admin-token')
      localStorage.setItem('auth_user', JSON.stringify(fakeAdminUser))
      localStorage.setItem('refresh_token', 'old-admin-refresh')
      localStorage.setItem('token_expires_at', String(Date.now() + 3600_000))

      const responseWithoutRefresh = {
        access_token: 'new-user-token',
        token_type: 'Bearer',
        user: { ...fakeUser },
      }
      mockLogin.mockResolvedValue(responseWithoutRefresh)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.user).toEqual(fakeUser)
      expect(store.isAdmin).toBe(false)
      expect(localStorage.getItem('auth_token')).toBe('new-user-token')
      expect(localStorage.getItem('auth_user')).toBe(JSON.stringify(fakeUser))
      expect(localStorage.getItem('refresh_token')).toBeNull()
      expect(localStorage.getItem('token_expires_at')).toBeNull()
    })

    it('需要 2FA 时清除旧认证状态', async () => {
      const adminResponse = { ...fakeAuthResponse, user: { ...fakeAdminUser } }
      mockLogin.mockResolvedValueOnce(adminResponse)
      const store = useAuthStore()
      await store.login({ email: 'admin@example.com', password: '123456' })
      expect(store.isAdmin).toBe(true)

      const twoFAResponse = { requires_2fa: true, temp_token: 'temp-123' }
      mockLogin.mockResolvedValueOnce(twoFAResponse)

      const result = await store.login({ email: 'test@example.com', password: '123456' })

      expect(result).toEqual(twoFAResponse)
      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
      expect(localStorage.getItem('auth_token')).toBeNull()
      expect(localStorage.getItem('auth_user')).toBeNull()
      expect(localStorage.getItem('refresh_token')).toBeNull()
    })
  })

  // --- login2FA ---

  describe('login2FA', () => {
    it('2FA 验证成功后设置认证状态', async () => {
      mockLogin2FA.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()

      const user = await store.login2FA('temp-123', '654321')

      expect(store.token).toBe('test-token-123')
      expect(store.user).toEqual(fakeUser)
      expect(user).toEqual(fakeUser)
      expect(mockLogin2FA).toHaveBeenCalledWith({
        temp_token: 'temp-123',
        totp_code: '654321',
      })
    })

    it('2FA 验证失败时清除状态并抛出错误', async () => {
      mockLogin2FA.mockRejectedValue(new Error('Invalid TOTP'))
      const store = useAuthStore()

      await expect(store.login2FA('temp-123', '000000')).rejects.toThrow('Invalid TOTP')
      expect(store.token).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })
  })

  // --- logout ---

  describe('logout', () => {
    it('注销后清除所有状态和 localStorage', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      mockLogout.mockResolvedValue(undefined)
      const store = useAuthStore()

      // 先登录
      await store.login({ email: 'test@example.com', password: '123456' })
      expect(store.isAuthenticated).toBe(true)

      // 注销
      await store.logout()

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
      expect(localStorage.getItem('auth_token')).toBeNull()
      expect(localStorage.getItem('auth_user')).toBeNull()
      expect(localStorage.getItem('refresh_token')).toBeNull()
      expect(localStorage.getItem('token_expires_at')).toBeNull()
    })
  })

  // --- checkAuth ---

  describe('checkAuth', () => {
    it('从 localStorage 恢复持久化状态', () => {
      localStorage.setItem('auth_token', 'saved-token')
      localStorage.setItem('auth_user', JSON.stringify(fakeUser))

      // Mock refreshUser (getCurrentUser) 防止后台刷新报错
      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })

      const store = useAuthStore()
      store.checkAuth()

      expect(store.token).toBe('saved-token')
      expect(store.user).toEqual(fakeUser)
      expect(store.isAuthenticated).toBe(true)
    })

    it('localStorage 无数据时保持未认证状态', () => {
      const store = useAuthStore()
      store.checkAuth()

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(store.isAuthenticated).toBe(false)
    })

    it('localStorage 中用户数据损坏时清除状态', () => {
      localStorage.setItem('auth_token', 'saved-token')
      localStorage.setItem('auth_user', 'invalid-json{{{')

      const store = useAuthStore()
      store.checkAuth()

      expect(store.token).toBeNull()
      expect(store.user).toBeNull()
      expect(localStorage.getItem('auth_token')).toBeNull()
    })

    it('恢复 refresh token 和过期时间', () => {
      const futureTs = String(Date.now() + 3600_000)
      localStorage.setItem('auth_token', 'saved-token')
      localStorage.setItem('auth_user', JSON.stringify(fakeUser))
      localStorage.setItem('refresh_token', 'saved-refresh')
      localStorage.setItem('token_expires_at', futureTs)

      mockGetCurrentUser.mockResolvedValue({ data: fakeUser })

      const store = useAuthStore()
      store.checkAuth()

      expect(store.isAuthenticated).toBe(true)
    })
  })

  // --- isAdmin ---

  describe('isAdmin', () => {
    it('管理员用户返回 true', async () => {
      const adminResponse = { ...fakeAuthResponse, user: { ...fakeAdminUser } }
      mockLogin.mockResolvedValue(adminResponse)
      const store = useAuthStore()

      await store.login({ email: 'admin@example.com', password: '123456' })

      expect(store.isAdmin).toBe(true)
    })

    it('普通用户返回 false', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.isAdmin).toBe(false)
    })

    it('未登录时返回 false', () => {
      const store = useAuthStore()
      expect(store.isAdmin).toBe(false)
    })
  })

  // --- refreshUser ---

  describe('refreshUser', () => {
    it('刷新用户数据并更新 localStorage', async () => {
      mockLogin.mockResolvedValue(fakeAuthResponse)
      const store = useAuthStore()
      await store.login({ email: 'test@example.com', password: '123456' })

      const updatedUser = { ...fakeUser, username: 'updated-name' }
      mockGetCurrentUser.mockResolvedValue({ data: updatedUser })

      const result = await store.refreshUser()

      expect(result).toEqual(updatedUser)
      expect(store.user).toEqual(updatedUser)
      expect(JSON.parse(localStorage.getItem('auth_user')!)).toEqual(updatedUser)
    })

    it('未认证时抛出错误', async () => {
      const store = useAuthStore()
      await expect(store.refreshUser()).rejects.toThrow('Not authenticated')
    })

    it('刷新 token 后同步当前用户信息', async () => {
      mockLogin.mockResolvedValue({ ...fakeAuthResponse, expires_in: 121 })
      mockRefreshToken.mockResolvedValue({
        access_token: 'new-token',
        refresh_token: 'new-refresh',
        expires_in: 3600,
        token_type: 'Bearer',
      })
      const syncedUser = { ...fakeUser, username: 'synced-name' }
      mockGetCurrentUser.mockResolvedValue({ data: syncedUser })
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })
      mockGetCurrentUser.mockClear()

      await vi.advanceTimersByTimeAsync(1000)

      expect(mockRefreshToken).toHaveBeenCalled()
      expect(mockGetCurrentUser).toHaveBeenCalled()
      expect(store.token).toBe('new-token')
      expect(store.user).toEqual(syncedUser)
      expect(localStorage.getItem('refresh_token')).toBe('new-refresh')
      expect(JSON.parse(localStorage.getItem('auth_user')!)).toEqual(syncedUser)
    })
  })

  // --- isSimpleMode ---

  describe('isSimpleMode', () => {
    it('run_mode 为 simple 时返回 true', async () => {
      const simpleResponse = {
        ...fakeAuthResponse,
        user: { ...fakeUser, run_mode: 'simple' as const },
      }
      mockLogin.mockResolvedValue(simpleResponse)
      const store = useAuthStore()

      await store.login({ email: 'test@example.com', password: '123456' })

      expect(store.isSimpleMode).toBe(true)
    })

    it('默认为 standard 模式', () => {
      const store = useAuthStore()
      expect(store.isSimpleMode).toBe(false)
    })
  })
})
