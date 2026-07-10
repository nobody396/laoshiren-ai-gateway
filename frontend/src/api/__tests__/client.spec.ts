import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import type {AxiosInstance} from 'axios'
import type {AuthSession} from '@/auth'
import axios from 'axios'
import clientSource from '../client.ts?raw'

// 需要在导入 client 之前设置 mock
vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))

describe('API Client', () => {
  let apiClient: AxiosInstance
  let authSession: AuthSession

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

  const resetStorage = () => {
    ensureStorage().clear()
  }

  beforeEach(async () => {
    resetStorage()
    // 每次测试重新导入以获取干净的模块状态
    vi.resetModules()
    const mod = await import('@/api/client')
    apiClient = mod.apiClient
    authSession = (await import('@/auth')).authSession
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  // --- 请求拦截器 ---

  describe('请求拦截器', () => {
    it('自动附加 Authorization 头', async () => {
      localStorage.setItem('auth_token', 'my-jwt-token')

      // 拦截实际请求
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('Authorization')).toBe('Bearer my-jwt-token')
      expect(config.headers.get('Cache-Control')).toContain('no-store')
      expect(config.headers.get('Pragma')).toBe('no-cache')
    })

    it('登录态 GET 请求附加防缓存参数', async () => {
      localStorage.setItem('auth_token', 'my-jwt-token')

      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/auth/me')

      const config = adapter.mock.calls[0][0]
      expect(config.params).toHaveProperty('_nc')
      expect(config.params._nc).toEqual(expect.any(String))
    })

    it('无 token 时不附加 Authorization 头', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('Authorization')).toBeFalsy()
      expect(config.params?._nc).toBeUndefined()
    })

    it('GET 请求自动附加 timezone 参数', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.params).toHaveProperty('timezone')
    })

    it('POST 请求不附加 timezone 参数', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.post('/test', { foo: 'bar' })

      const config = adapter.mock.calls[0][0]
      expect(config.params?.timezone).toBeUndefined()
    })
  })

  // --- 响应拦截器 ---

  describe('响应拦截器', () => {
    it('code=0 时解包 data 字段', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: { name: 'test' }, message: 'ok' },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      const response = await apiClient.get('/test')
      expect(response.data).toEqual({ name: 'test' })
    })

    it('code!=0 时拒绝并返回结构化错误', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 1001, message: '参数错误', data: null },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          code: 1001,
          message: '参数错误',
        })
      )
    })

    it('下载接口返回 Blob 错误时解析后端消息', async () => {
      const blobLike = {
        text: vi.fn().mockResolvedValue(JSON.stringify({
          code: 400,
          reason: 'INVOICE_EXPORT_EMPTY',
          message: 'no invoice requests available for export',
        })),
        arrayBuffer: vi.fn(),
      }

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 400,
          data: blobLike,
        },
        config: {
          url: '/admin/invoice/requests/export',
        },
        code: 'ERR_BAD_REQUEST',
        message: 'Request failed with status code 400',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.post('/admin/invoice/requests/export', {}, { responseType: 'blob' })).rejects.toEqual(
        expect.objectContaining({
          status: 400,
          code: 400,
          reason: 'INVOICE_EXPORT_EMPTY',
          message: 'no invoice requests available for export',
        })
      )
    })
  })

  // --- 401 Token 刷新 ---

  describe('401 Token 刷新', () => {
    it('十个并发 401 只旋转一次 refresh token 并全部重放', async () => {
      const fakeUser = {
        id: 1, username: 'user', email: 'user@example.com', role: 'user' as const,
        balance: 0, concurrency: 1, status: 'active' as const, allowed_groups: null,
        created_at: '2026-01-01', updated_at: '2026-01-01',
      }
      authSession.setAuthenticated({
        access_token: 'old-access', refresh_token: 'old-refresh', expires_in: 3600,
        token_type: 'Bearer', user: fakeUser,
      })
      let resolveRefresh!: (value: any) => void
      const refreshGate = new Promise((resolve) => { resolveRefresh = resolve })
      let releaseRefreshStarted!: () => void
      const refreshStarted = new Promise<void>((resolve) => { releaseRefreshStarted = resolve })
      const refresh = vi.fn(() => {
        releaseRefreshStarted()
        return refreshGate
      })
      authSession.configureRefresh(refresh)

      let initialAttempts = 0
      let releaseInitial!: () => void
      const allInitial = new Promise<void>((resolve) => { releaseInitial = resolve })
      const adapter = vi.fn(async (config: any) => {
        const authorization = config.headers.get('Authorization')
        if (authorization === 'Bearer rotated-access') {
          return { status: 200, data: { code: 0, data: { ok: true } }, headers: {}, config, statusText: 'OK' }
        }
        initialAttempts += 1
        if (initialAttempts === 10) releaseInitial()
        throw {
          response: { status: 401, data: { code: 'TOKEN_EXPIRED', message: 'expired' } },
          config,
          code: 'ERR_BAD_REQUEST',
          message: 'expired',
        }
      })
      apiClient.defaults.adapter = adapter

      const requests = Array.from({ length: 10 }, () => apiClient.get('/protected'))
      await allInitial
      await refreshStarted
      expect(refresh).toHaveBeenCalledTimes(1)
      resolveRefresh({ access_token: 'rotated-access', refresh_token: 'rotated-refresh', expires_in: 600 })
      const responses = await Promise.all(requests)

      expect(responses.every((response) => response.data.ok)).toBe(true)
      expect(refresh).toHaveBeenCalledTimes(1)
      expect(localStorage.getItem('refresh_token')).toBe('rotated-refresh')
    })
    it('无 refresh_token 时 401 清除 localStorage', async () => {
      localStorage.setItem('auth_token', 'expired-token')
      // 不设置 refresh_token

      // Mock window.location
      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/dashboard', href: '/dashboard' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'TOKEN_EXPIRED', message: 'Token expired' },
        },
        config: {
          url: '/test',
          headers: { Authorization: 'Bearer expired-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toBeDefined()

      expect(localStorage.getItem('auth_token')).toBeNull()

      // 恢复 location
      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })

    it('账号 usage 接口 401 不清除全局登录态', async () => {
      localStorage.setItem('auth_token', 'still-valid-token')

      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/admin/accounts', href: '/admin/accounts' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'UNAUTHORIZED', message: 'Authorization required' },
        },
        config: {
          url: '/admin/accounts/18/usage',
          headers: { Authorization: 'Bearer still-valid-token' },
        },
        code: 'ERR_BAD_REQUEST',
        message: 'Request failed with status code 401',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/admin/accounts/18/usage')).rejects.toEqual(
        expect.objectContaining({
          status: 401,
          code: 'UNAUTHORIZED',
          message: 'Authorization required',
        })
      )

      expect(localStorage.getItem('auth_token')).toBe('still-valid-token')

      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })

    it('账号列表自动刷新接口 401 不清除全局登录态', async () => {
      localStorage.setItem('auth_token', 'still-valid-token')

      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/admin/accounts', href: '/admin/accounts' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'UNAUTHORIZED', message: 'Authorization required' },
        },
        config: {
          url: '/admin/accounts?page=1&page_size=20',
          headers: { Authorization: 'Bearer still-valid-token' },
        },
        code: 'ERR_BAD_REQUEST',
        message: 'Request failed with status code 401',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/admin/accounts', { params: { page: 1, page_size: 20 } })).rejects.toEqual(
        expect.objectContaining({
          status: 401,
          code: 'UNAUTHORIZED',
          message: 'Authorization required',
        })
      )

      expect(localStorage.getItem('auth_token')).toBe('still-valid-token')

      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })

    it('账号今日统计批量接口 401 不清除全局登录态', async () => {
      localStorage.setItem('auth_token', 'still-valid-token')

      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/admin/accounts', href: '/admin/accounts' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'UNAUTHORIZED', message: 'Authorization required' },
        },
        config: {
          url: '/admin/accounts/today-stats/batch',
          headers: { Authorization: 'Bearer still-valid-token' },
        },
        code: 'ERR_BAD_REQUEST',
        message: 'Request failed with status code 401',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.post('/admin/accounts/today-stats/batch', { account_ids: [1, 2, 3] })).rejects.toEqual(
        expect.objectContaining({
          status: 401,
          code: 'UNAUTHORIZED',
          message: 'Authorization required',
        })
      )

      expect(localStorage.getItem('auth_token')).toBe('still-valid-token')

      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })
  })

  it('transport source does not import stores, router or UI modules', () => {
    expect(clientSource).not.toMatch(/from ['"]@\/stores/)
    expect(clientSource).not.toMatch(/from ['"]@\/router/)
    expect(clientSource).not.toMatch(/useAppStore|useAuthStore|router\./)
  })

  // --- 网络错误 ---

  describe('网络错误', () => {
    it('网络错误返回 status 0 的错误', async () => {
      const adapter = vi.fn().mockRejectedValue({
        code: 'ERR_NETWORK',
        message: 'Network Error',
        config: { url: '/test' },
        // 没有 response
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          status: 0,
          message: 'Network error. Please check your connection.',
        })
      )
    })
  })

  // --- 请求取消 ---

  describe('请求取消', () => {
    it('取消的请求保持原始取消错误', async () => {
      const source = axios.CancelToken.source()

      const adapter = vi.fn().mockRejectedValue(
        new axios.Cancel('Operation canceled')
      )
      apiClient.defaults.adapter = adapter

      await expect(
        apiClient.get('/test', { cancelToken: source.token })
      ).rejects.toBeDefined()
    })
  })
})
