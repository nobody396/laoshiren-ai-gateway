/**
 * Axios HTTP Client Configuration
 * Base client with interceptors for authentication, token refresh, and error handling
 */

import axios, {AxiosError, AxiosInstance, AxiosResponse, InternalAxiosRequestConfig} from 'axios'
import type {ApiResponse} from '@/types'
import i18n, {getLocale} from '@/i18n'
import {useAppStore} from '@/stores/app'

// ==================== Axios Instance Configuration ====================

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

export const apiClient: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// ==================== Token Refresh State ====================

// Track if a token refresh is in progress to prevent multiple simultaneous refresh requests
let isRefreshing = false
// Queue of requests waiting for token refresh
let refreshSubscribers: Array<(token: string) => void> = []

const NON_FATAL_401_PATTERNS = [
  /\/admin\/accounts(?:$|\?)/,
  /\/admin\/accounts\/today-stats\/batch(?:$|\?)/,
  /\/admin\/accounts\/\d+\/usage(?:$|\?)/
]

/**
 * Subscribe to token refresh completion
 */
function subscribeTokenRefresh(callback: (token: string) => void): void {
  refreshSubscribers.push(callback)
}

/**
 * Notify all subscribers that token has been refreshed
 */
function onTokenRefreshed(token: string): void {
  refreshSubscribers.forEach((callback) => callback(token))
  refreshSubscribers = []
}

function isNonFatal401Endpoint(url: string): boolean {
  return NON_FATAL_401_PATTERNS.some((pattern) => pattern.test(url))
}

function isBlobLike(data: unknown): data is Blob {
  return !!data
    && typeof data === 'object'
    && typeof (data as Blob).text === 'function'
    && typeof (data as Blob).arrayBuffer === 'function'
}

let lastForbiddenToastAt = 0
function notifyForbiddenOnce(message: string): void {
  const now = Date.now()
  if (now - lastForbiddenToastAt < 3000) return
  lastForbiddenToastAt = now
  try {
    useAppStore().showWarning(message)
  } catch {
    // Pinia may be unavailable in isolated tests.
  }
}

async function normalizeErrorPayload(data: unknown): Promise<Record<string, any>> {
  if (typeof data === 'object' && data !== null) {
    if (isBlobLike(data)) {
      try {
        const text = await data.text()
        if (!text) {
          return {}
        }
        return normalizeErrorPayload(text)
      } catch {
        return {}
      }
    }
    return data as Record<string, any>
  }

  if (typeof data === 'string') {
    const trimmed = data.trim()
    if (!trimmed) {
      return {}
    }

    try {
      const parsed = JSON.parse(trimmed)
      if (typeof parsed === 'object' && parsed !== null) {
        return parsed as Record<string, any>
      }
    } catch {
      // Fall through to surface plain-text messages.
    }

    return { message: trimmed }
  }

  return {}
}

// ==================== Request Interceptor ====================

// Get user's timezone
const getUserTimezone = (): string => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    // Attach token from localStorage
    const token = localStorage.getItem('auth_token')
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`
    }

    // Attach locale for backend translations
    if (config.headers) {
      config.headers['Accept-Language'] = getLocale()
    }

    // Attach timezone for all GET requests (backend may use it for default date ranges)
    if (config.method === 'get') {
      if (!config.params) {
        config.params = {}
      }
      config.params.timezone = getUserTimezone()
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// ==================== Response Interceptor ====================

apiClient.interceptors.response.use(
  (response: AxiosResponse) => {
    // Unwrap standard API response format { code, message, data }
    const apiResponse = response.data as ApiResponse<unknown>
    if (apiResponse && typeof apiResponse === 'object' && 'code' in apiResponse) {
      if (apiResponse.code === 0) {
        // Success - return the data portion
        response.data = apiResponse.data
      } else {
        // API error
        return Promise.reject({
          status: response.status,
          code: apiResponse.code,
          message: apiResponse.message || 'Unknown error'
        })
      }
    }
    return response
  },
  async (error: AxiosError<ApiResponse<unknown>>) => {
    // Request cancellation: keep the original axios cancellation error so callers can ignore it.
    // Otherwise we'd misclassify it as a generic "network error".
    if (error.code === 'ERR_CANCELED' || axios.isCancel(error)) {
      return Promise.reject(error)
    }

    const originalRequest = error.config as InternalAxiosRequestConfig & {
      _retry?: boolean
      // When true, a 401 on this request will NOT clear the global auth state or
      // redirect to /login.  Use for "per-resource" endpoints (e.g. account usage)
      // where a 401 reflects a per-row problem, not a session-level failure.
      _skipLogout?: boolean
    }

    // Handle common errors
    if (error.response) {
      const { status, data } = error.response
      const url = String(error.config?.url || '')

      // Normalize JSON/blob/plain-text errors so download endpoints can still surface API messages.
      const apiData = await normalizeErrorPayload(data)

      // Ops monitoring disabled: treat as feature-flagged 404, and proactively redirect away
      // from ops pages to avoid broken UI states.
      if (status === 404 && apiData.message === 'Ops monitoring is disabled') {
        try {
          localStorage.setItem('ops_monitoring_enabled_cached', 'false')
        } catch {
          // ignore localStorage failures
        }
        try {
          window.dispatchEvent(new CustomEvent('ops-monitoring-disabled'))
        } catch {
          // ignore event failures
        }

        if (window.location.pathname.startsWith('/admin/ops')) {
          window.location.href = '/admin/settings'
        }

        return Promise.reject({
          status,
          code: 'OPS_DISABLED',
          reason: apiData.reason,
          message: apiData.message || error.message,
          url
        })
      }

      if (status === 401 && isNonFatal401Endpoint(url)) {
        return Promise.reject({
          status,
          code: apiData.code,
          reason: apiData.reason,
          error: apiData.error,
          message: apiData.message || apiData.detail || error.message,
          url
        })
      }

      if (status === 403 && apiData.code === 'FORBIDDEN') {
        const friendly = i18n.global.t('common.permissionDenied') as string
        notifyForbiddenOnce(friendly)
        return Promise.reject({
          status,
          code: 'FORBIDDEN',
          reason: apiData.reason,
          error: apiData.error,
          message: friendly,
          url
        })
      }

      // 401: Try to refresh the token if we have a refresh token
      // This handles TOKEN_EXPIRED, INVALID_TOKEN, TOKEN_REVOKED, etc.
      if (status === 401 && !originalRequest._retry) {
        const refreshToken = localStorage.getItem('refresh_token')
        const isAuthEndpoint =
          url.includes('/auth/login') || url.includes('/auth/register') || url.includes('/auth/refresh')

        // If we have a refresh token and this is not an auth endpoint, try to refresh
        if (refreshToken && !isAuthEndpoint) {
          if (isRefreshing) {
            // Wait for the ongoing refresh to complete
            return new Promise((resolve, reject) => {
              subscribeTokenRefresh((newToken: string) => {
                if (newToken) {
                  // Mark as retried to prevent infinite loop if retry also returns 401
                  originalRequest._retry = true
                  if (originalRequest.headers) {
                    originalRequest.headers.Authorization = `Bearer ${newToken}`
                  }
                  resolve(apiClient(originalRequest))
                } else {
                  // Refresh failed, reject with original error
                  reject({
                    status,
                    code: apiData.code,
                    reason: apiData.reason,
                    message: apiData.message || apiData.detail || error.message
                  })
                }
              })
            })
          }

          originalRequest._retry = true
          isRefreshing = true

          try {
            // Call refresh endpoint directly to avoid circular dependency
            const refreshResponse = await axios.post(
              `${API_BASE_URL}/auth/refresh`,
              { refresh_token: refreshToken },
              { headers: { 'Content-Type': 'application/json' } }
            )

            const refreshData = refreshResponse.data as ApiResponse<{
              access_token: string
              refresh_token: string
              expires_in: number
            }>

            if (refreshData.code === 0 && refreshData.data) {
              const { access_token, refresh_token: newRefreshToken, expires_in } = refreshData.data

              // Update tokens in localStorage (convert expires_in to timestamp)
              localStorage.setItem('auth_token', access_token)
              localStorage.setItem('refresh_token', newRefreshToken)
              localStorage.setItem('token_expires_at', String(Date.now() + expires_in * 1000))

              // Notify subscribers with new token
              onTokenRefreshed(access_token)

              // Retry the original request with new token
              if (originalRequest.headers) {
                originalRequest.headers.Authorization = `Bearer ${access_token}`
              }

              isRefreshing = false
              return apiClient(originalRequest)
            }

            // Refresh response was not successful, fall through to clear auth
            throw new Error('Token refresh failed')
          } catch (refreshError) {
            // Refresh failed - notify subscribers with empty token
            onTokenRefreshed('')
            isRefreshing = false

            // Clear tokens and redirect to login
            localStorage.removeItem('auth_token')
            localStorage.removeItem('refresh_token')
            localStorage.removeItem('auth_user')
            localStorage.removeItem('token_expires_at')
            sessionStorage.setItem('auth_expired', '1')

            if (!window.location.pathname.includes('/login')) {
              window.location.href = '/login'
            }

            return Promise.reject({
              status: 401,
              code: 'TOKEN_REFRESH_FAILED',
              message: 'Session expired. Please log in again.'
            })
          }
        }

        // No refresh token or is auth endpoint - clear auth and redirect
        // Skip the global logout side-effects for requests that are explicitly
        // marked as per-resource (e.g. fetching per-account usage data).  A 401
        // on those endpoints does not mean the admin session has expired.
        if (!originalRequest._skipLogout) {
          const hasToken = !!localStorage.getItem('auth_token')
          const headers = error.config?.headers as Record<string, unknown> | undefined
          const authHeader = headers?.Authorization ?? headers?.authorization
          const sentAuth =
            typeof authHeader === 'string'
              ? authHeader.trim() !== ''
              : Array.isArray(authHeader)
                ? authHeader.length > 0
                : !!authHeader

          localStorage.removeItem('auth_token')
          localStorage.removeItem('refresh_token')
          localStorage.removeItem('auth_user')
          localStorage.removeItem('token_expires_at')
          if ((hasToken || sentAuth) && !isAuthEndpoint) {
            sessionStorage.setItem('auth_expired', '1')
          }
          // Only redirect if not already on login page
          if (!window.location.pathname.includes('/login')) {
            window.location.href = '/login'
          }
        }
      }

      // Return structured error
      return Promise.reject({
        status,
        code: apiData.code,
        reason: apiData.reason,
        error: apiData.error,
        message: apiData.message || apiData.detail || error.message
      })
    }

    // Network error
    return Promise.reject({
      status: 0,
      message: 'Network error. Please check your connection.'
    })
  }
)

export default apiClient
