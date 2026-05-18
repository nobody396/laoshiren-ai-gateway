/**
 * User API endpoints
 * Handles user profile management and password changes
 */

import { apiClient } from './client'
import type { User, ChangePasswordRequest } from '@/types'

export interface UserReferralDashboard {
  invited_user_count: number
  total_commission: number
  this_month_commission: number
  first_recharge_invitee_rate: number
  first_recharge_referral_rate: number
}

export interface UserIdentityBinding {
  id: number
  user_id: number
  provider: string
  provider_user_id: string
  email: string
  email_verified: boolean
  display_name: string
  avatar_url: string
  raw_profile: Record<string, unknown>
  last_login_at?: string | null
  bound_at: string
  created_at: string
  updated_at: string
}

export interface StartIdentityBindResponse {
  auth_url: string
}

/**
 * Get current user profile
 * @returns User profile data
 */
export async function getProfile(): Promise<User> {
  const { data } = await apiClient.get<User>('/user/profile')
  return data
}

/**
 * Update current user profile
 * @param profile - Profile data to update
 * @returns Updated user profile data
 */
export async function updateProfile(profile: {
  username?: string
}): Promise<User> {
  const { data } = await apiClient.put<User>('/user', profile)
  return data
}

/**
 * Change current user password
 * @param passwords - Old and new password
 * @returns Success message
 */
export async function changePassword(
  oldPassword: string,
  newPassword: string
): Promise<{ message: string }> {
  const payload: ChangePasswordRequest = {
    old_password: oldPassword,
    new_password: newPassword
  }

  const { data } = await apiClient.put<{ message: string }>('/user/password', payload)
  return data
}

/**
 * Get referral dashboard stats for current user
 */
export async function getUserReferralDashboard(): Promise<UserReferralDashboard> {
  const { data } = await apiClient.get<UserReferralDashboard>('/user/referral/dashboard')
  return data
}

export async function listIdentityBindings(): Promise<UserIdentityBinding[]> {
  const { data } = await apiClient.get<UserIdentityBinding[]>('/user/identities')
  return data
}

export async function startLinuxDoBind(redirect = '/profile'): Promise<StartIdentityBindResponse> {
  return startIdentityBind('linuxdo', redirect)
}

export async function startIdentityBind(
  provider: 'linuxdo' | 'google' | 'github',
  redirect = '/profile'
): Promise<StartIdentityBindResponse> {
  const { data } = await apiClient.post<StartIdentityBindResponse>(`/user/identities/${provider}/start-bind`, null, {
    params: { redirect }
  })
  return data
}

export async function unbindIdentity(provider: string): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/user/identities/${provider}`)
  return data
}

/**
 * Send verification code for a notification email
 */
export const userAPI = {
  getProfile,
  updateProfile,
  changePassword,
  getUserReferralDashboard,
  listIdentityBindings,
  startLinuxDoBind,
  startIdentityBind,
  unbindIdentity
}

export default userAPI
