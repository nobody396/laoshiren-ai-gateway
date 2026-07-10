/**
 * Transport-only Axios client. Authentication ownership lives in AuthSession;
 * UI reactions are injected by bootstrap through api/runtime.ts.
 */

import axios, { AxiosError, AxiosInstance, AxiosResponse, InternalAxiosRequestConfig } from 'axios'
import type { ApiResponse } from '@/types'
import { getLocale } from '@/i18n'
import { authSession, AuthSessionChangedError } from '@/auth'
import { ApiError } from './errors'
import { emitApiEvent } from './runtime'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'
const AUTHENTICATED_GET_CACHE_BUSTER = '_nc'

export const apiClient: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
})

authSession.bindEvents((event) => emitApiEvent(event))

const NON_FATAL_401_PATTERNS = [
  /\/admin\/accounts(?:$|\?)/,
  /\/admin\/accounts\/today-stats\/batch(?:$|\?)/,
  /\/admin\/accounts\/\d+\/usage(?:$|\?)/,
]

const NO_SESSION_REFRESH_PATHS = [
  '/auth/login',
  '/auth/login/2fa',
  '/auth/register',
  '/auth/refresh',
  '/auth/sso/exchange',
  '/auth/embed/exchange',
]

type RetryConfig = InternalAxiosRequestConfig & { _retry?: boolean; _skipLogout?: boolean }

function isNonFatal401Endpoint(url: string): boolean {
  return NON_FATAL_401_PATTERNS.some((pattern) => pattern.test(url))
}

function isNoSessionRefreshEndpoint(url: string): boolean {
  return NO_SESSION_REFRESH_PATHS.some((path) => url.includes(path))
}

function isBlobLike(data: unknown): data is Blob {
  return !!data && typeof data === 'object' &&
    typeof (data as Blob).text === 'function' && typeof (data as Blob).arrayBuffer === 'function'
}

async function normalizeErrorPayload(data: unknown): Promise<Record<string, any>> {
  if (typeof data === 'object' && data !== null) {
    if (isBlobLike(data)) {
      try { return normalizeErrorPayload(await data.text()) } catch { return {} }
    }
    return data as Record<string, any>
  }
  if (typeof data !== 'string' || !data.trim()) return {}
  try {
    const parsed = JSON.parse(data)
    return typeof parsed === 'object' && parsed !== null ? parsed : { message: data.trim() }
  } catch {
    return { message: data.trim() }
  }
}

function getUserTimezone(): string {
  try { return Intl.DateTimeFormat().resolvedOptions().timeZone } catch { return 'UTC' }
}

function sentAuthorization(config?: InternalAxiosRequestConfig): boolean {
  const headers = config?.headers as Record<string, unknown> | undefined
  const value = headers?.Authorization ?? headers?.authorization
  if (typeof value === 'string') return value.trim() !== ''
  if (Array.isArray(value)) return value.length > 0
  return !!value
}

apiClient.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  // Import legacy storage exactly once into the owner before reading it. This
  // preserves existing sessions while keeping transport away from localStorage.
  authSession.hydrate()
  const token = authSession.accessToken
  if (token && config.headers) {
    config.headers.Authorization = `Bearer ${token}`
    config.headers['Cache-Control'] = 'no-cache, no-store, must-revalidate'
    config.headers.Pragma = 'no-cache'
  }
  if (config.headers) config.headers['Accept-Language'] = getLocale()
  if (config.method === 'get') {
    config.params ??= {}
    config.params.timezone = getUserTimezone()
    if (token && config.params[AUTHENTICATED_GET_CACHE_BUSTER] == null) {
      config.params[AUTHENTICATED_GET_CACHE_BUSTER] = `${Date.now()}-${Math.random().toString(36).slice(2)}`
    }
  }
  return config
})

let lastForbiddenEventAt = 0

apiClient.interceptors.response.use(
  (response: AxiosResponse) => {
    const wrapped = response.data as ApiResponse<unknown>
    if (wrapped && typeof wrapped === 'object' && 'code' in wrapped) {
      if (wrapped.code !== 0) {
        throw new ApiError(response.status, wrapped.code, wrapped.message || 'Unknown error')
      }
      response.data = wrapped.data
    }
    return response
  },
  async (error: AxiosError<ApiResponse<unknown>>) => {
    if (error.code === 'ERR_CANCELED' || axios.isCancel(error)) throw error
    if (!error.response) {
      throw new ApiError(0, undefined, 'Network error. Please check your connection.')
    }

    const { status, data } = error.response
    const config = error.config as RetryConfig
    const url = String(config?.url || '')
    const apiData = await normalizeErrorPayload(data)
    const apiError = () => new ApiError(
      status,
      apiData.code,
      apiData.message || apiData.detail || error.message,
      { reason: apiData.reason, error: apiData.error, url },
    )

    if (status === 404 && apiData.message === 'Ops monitoring is disabled') {
      emitApiEvent({ type: 'ops-monitoring-disabled' })
      throw new ApiError(status, 'OPS_DISABLED', apiData.message, { reason: apiData.reason, url })
    }
    if (status === 401 && isNonFatal401Endpoint(url)) throw apiError()
    if (status === 403 && apiData.code === 'FORBIDDEN') {
      const now = Date.now()
      if (now - lastForbiddenEventAt >= 3000) {
        lastForbiddenEventAt = now
        emitApiEvent({ type: 'forbidden', messageKey: 'common.permissionDenied' })
      }
      throw new ApiError(status, 'FORBIDDEN', apiData.message || 'Permission denied', {
        reason: apiData.reason, error: apiData.error, url,
      })
    }

    if (status === 401 && !config?._skipLogout && !isNoSessionRefreshEndpoint(url)) {
      if (!config._retry && authSession.refreshToken) {
        config._retry = true
        try {
          const refreshed = await authSession.refreshOnce()
          if (config.headers) config.headers.Authorization = `Bearer ${refreshed.access_token}`
          return apiClient(config)
        } catch (refreshError) {
          if (refreshError instanceof AuthSessionChangedError) throw apiError()
          throw new ApiError(401, 'TOKEN_REFRESH_FAILED', 'Session expired. Please log in again.')
        }
      }
      if (authSession.accessToken || authSession.refreshToken || sentAuthorization(config)) {
        authSession.clear('expired')
      }
    }

    throw apiError()
  },
)

export default apiClient
