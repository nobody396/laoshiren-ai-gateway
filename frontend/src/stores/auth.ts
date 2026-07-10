/**
 * Pinia projection of the single AuthSession owner.
 */

import { computed, readonly, ref } from 'vue'
import { defineStore } from 'pinia'
import { authAPI, isTotp2FARequired, type LoginResponse } from '@/api'
import { authSession, type StoredAuthSession } from '@/auth'
import { usePermissionStore } from '@/stores/permission'
import type { AuthResponse, LoginRequest, RegisterRequest, User } from '@/types'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const token = ref<string | null>(null)
  const runMode = ref<'standard' | 'simple'>('standard')

  const isAuthenticated = computed(() => !!token.value && !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const isSimpleMode = computed(() => runMode.value === 'simple')

  authSession.configureRefresh((refreshToken, signal) => authAPI.refreshToken(refreshToken, signal))
  authSession.hydrate()
  authSession.bindSnapshot(applySnapshot)
  authSession.bindRefreshed(async () => {
    if (token.value && user.value) await refreshUser()
  })

  function applySnapshot(snapshot: Readonly<StoredAuthSession>): void {
    token.value = snapshot.accessToken
    if (snapshot.user?.run_mode) runMode.value = snapshot.user.run_mode
    else if (!snapshot.user) runMode.value = 'standard'
    if (snapshot.user) {
      const { run_mode: _runMode, ...userData } = snapshot.user
      user.value = userData
    } else {
      user.value = null
    }
  }

  function checkAuth(): void {
    const snapshot = authSession.hydrate()
    applySnapshot(snapshot)
    if (!snapshot.accessToken || !snapshot.user) return
    void refreshUser().catch((error) => console.error('Failed to refresh user on init:', error))
    if (snapshot.user.role === 'admin') {
      void usePermissionStore().fetchPermissions().catch((error) => {
        console.error('Failed to fetch RBAC permissions on init:', error)
      })
    }
  }

  async function login(credentials: LoginRequest): Promise<LoginResponse> {
    authSession.clear('replace')
    try {
      const response = await authAPI.login(credentials)
      if (!isTotp2FARequired(response)) setAuthFromResponse(response)
      return response
    } catch (error) {
      authSession.clear('replace')
      throw error
    }
  }

  async function login2FA(tempToken: string, totpCode: string): Promise<User> {
    authSession.clear('replace')
    try {
      const response = await authAPI.login2FA({ temp_token: tempToken, totp_code: totpCode })
      setAuthFromResponse(response)
      return user.value!
    } catch (error) {
      authSession.clear('replace')
      throw error
    }
  }

  async function register(userData: RegisterRequest): Promise<User> {
    authSession.clear('replace')
    try {
      const response = await authAPI.register(userData)
      setAuthFromResponse(response)
      return user.value!
    } catch (error) {
      authSession.clear('replace')
      throw error
    }
  }

  function setAuthFromResponse(response: AuthResponse): void {
    if (response.user.run_mode) runMode.value = response.user.run_mode
    const { run_mode: _runMode, ...userData } = response.user
    authSession.setAuthenticated({ ...response, user: userData }, userData)
    syncPermissions(userData)
  }

  async function setToken(
    newToken: string,
    newRefreshToken: string | null = authSession.refreshToken,
    expiresIn?: number,
  ): Promise<User> {
    authSession.adoptTokens(newToken, newRefreshToken, expiresIn)
    try {
      const refreshedUser = await refreshUser()
      syncPermissions(refreshedUser)
      return refreshedUser
    } catch (error) {
      authSession.clear('replace')
      throw error
    }
  }

  async function logout(): Promise<void> {
    const refreshToken = authSession.refreshToken
    try {
      await authAPI.logout(refreshToken)
    } finally {
      authSession.clear('logout')
      usePermissionStore().reset()
    }
  }

  async function refreshUser(): Promise<User> {
    if (!token.value) throw new Error('Not authenticated')
    try {
      const response = await authAPI.getCurrentUser()
      if (response.data.run_mode) runMode.value = response.data.run_mode
      const { run_mode: _runMode, ...userData } = response.data
      authSession.updateUser(userData)
      syncPermissions(userData)
      return userData
    } catch (error) {
      if ((error as { status?: number }).status === 401) authSession.clear('expired')
      throw error
    }
  }

  function syncPermissions(currentUser: User): void {
    const permissionStore = usePermissionStore()
    if (currentUser.role === 'admin') void permissionStore.fetchPermissions()
    else permissionStore.reset()
  }

  return {
    user,
    token,
    runMode: readonly(runMode),
    isAuthenticated,
    isAdmin,
    isSimpleMode,
    login,
    login2FA,
    register,
    setToken,
    logout,
    checkAuth,
    refreshUser,
  }
})
