/**
 * Authentication API endpoints
 * Handles user login, registration, and logout operations
 */

import axios from 'axios'
import { apiClient } from './client'
import { authSession } from '@/auth'
import type {
  LoginRequest,
  RegisterRequest,
  AuthResponse,
  CurrentUserResponse,
  SendVerifyCodeRequest,
  SendVerifyCodeResponse,
  PublicSettings,
  TotpLoginResponse,
  TotpLogin2FARequest,
  ApiResponse,
} from '@/types'

/**
 * Login response type - can be either full auth or 2FA required
 */
export type LoginResponse = AuthResponse | TotpLoginResponse

/**
 * Type guard to check if login response requires 2FA
 */
export function isTotp2FARequired(response: LoginResponse): response is TotpLoginResponse {
  return 'requires_2fa' in response && response.requires_2fa === true
}

/**
 * User login
 * @param credentials - Email and password
 * @returns Authentication response with token and user data, or 2FA required response
 */
export async function login(credentials: LoginRequest): Promise<LoginResponse> {
  const { data } = await apiClient.post<LoginResponse>('/auth/login', credentials)
  return data
}

/**
 * Complete login with 2FA code
 * @param request - Temp token and TOTP code
 * @returns Authentication response with token and user data
 */
export async function login2FA(request: TotpLogin2FARequest): Promise<AuthResponse> {
  const { data } = await apiClient.post<AuthResponse>('/auth/login/2fa', request)
  return data
}

/**
 * User registration
 * @param userData - Registration data (username, email, password)
 * @returns Authentication response with token and user data
 */
export async function register(userData: RegisterRequest): Promise<AuthResponse> {
  const { data } = await apiClient.post<AuthResponse>('/auth/register', userData)
  return data
}

/**
 * Get current authenticated user
 * @returns User profile data
 */
export async function getCurrentUser() {
  return apiClient.get<CurrentUserResponse>('/auth/me')
}

/**
 * User logout
 * Clears authentication token and user data from localStorage
 * Optionally revokes the refresh token on the server
 */
export async function logout(refreshToken?: string | null): Promise<void> {
  // Try to revoke the refresh token on the server
  if (refreshToken) {
    try {
      await apiClient.post('/auth/logout', { refresh_token: refreshToken })
    } catch {
      // Ignore errors - we still want to clear local state
    }
  }

}

/**
 * Refresh token response
 */
export interface RefreshTokenResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: string
}

/**
 * Refresh the access token using the refresh token
 * @returns New token pair
 */
export async function refreshToken(currentRefreshToken?: string | null, signal?: AbortSignal): Promise<RefreshTokenResponse> {
  if (!currentRefreshToken) {
    throw new Error('No refresh token available')
  }
  const baseURL = import.meta.env.VITE_API_BASE_URL || '/api/v1'
  const response = await axios.post<ApiResponse<RefreshTokenResponse>>(
    `${baseURL}/auth/refresh`,
    { refresh_token: currentRefreshToken },
    { headers: { 'Content-Type': 'application/json' }, signal },
  )
  if (response.data.code !== 0 || !response.data.data) {
    throw new Error(response.data.message || 'Token refresh failed')
  }
  return response.data.data
}

/**
 * Revoke all sessions for the current user
 * @returns Response with message
 */
export async function revokeAllSessions(): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/auth/revoke-all-sessions')
  return data
}

export interface SSOTicketResponse {
  ticket: string
  expires_in: number
}

export async function issueSSOTicket(apiKeyId?: number): Promise<SSOTicketResponse> {
  const { data } = await apiClient.post<SSOTicketResponse>('/auth/sso/ticket', {
    ...(apiKeyId ? { api_key_id: apiKeyId } : {})
  })
  return data
}

export type EmbedTargetKind = 'purchase_subscription' | 'custom_menu'
export type EmbedDelivery = 'iframe' | 'new_tab'

export interface EmbedTicketResponse {
  ticket: string
  expires_in: number
  target_url: string
  audience: string
}

export async function issueEmbedTicket(
  targetKind: EmbedTargetKind,
  targetId: string,
  delivery: EmbedDelivery,
): Promise<EmbedTicketResponse> {
  const { data } = await apiClient.post<EmbedTicketResponse>('/auth/embed/ticket', {
    target_kind: targetKind,
    target_id: targetId,
    delivery,
  })
  return data
}

/**
 * Check if user is authenticated
 * @returns True if user has valid token
 */
export function isAuthenticated(): boolean {
  return authSession.accessToken !== null
}

/**
 * Get public settings (no auth required)
 * @returns Public settings including registration and Turnstile config
 */
export async function getPublicSettings(): Promise<PublicSettings> {
  const { data } = await apiClient.get<PublicSettings>('/settings/public')
  return data
}

/**
 * Send verification code to email
 * @param request - Email and optional Turnstile token
 * @returns Response with countdown seconds
 */
export async function sendVerifyCode(
  request: SendVerifyCodeRequest
): Promise<SendVerifyCodeResponse> {
  const { data } = await apiClient.post<SendVerifyCodeResponse>('/auth/send-verify-code', request)
  return data
}

/**
 * Validate promo code response
 */
export interface ValidatePromoCodeResponse {
  valid: boolean
  bonus_amount?: number
  error_code?: string
  message?: string
}

/**
 * Validate promo code (public endpoint, no auth required)
 * @param code - Promo code to validate
 * @returns Validation result with bonus amount if valid
 */
export async function validatePromoCode(code: string): Promise<ValidatePromoCodeResponse> {
  const { data } = await apiClient.post<ValidatePromoCodeResponse>('/auth/validate-promo-code', { code })
  return data
}

/**
 * Validate invitation code response
 */
export interface ValidateInvitationCodeResponse {
  valid: boolean
  error_code?: string
}

/**
 * Validate invitation code (public endpoint, no auth required)
 * @param code - Invitation code to validate
 * @returns Validation result
 */
export async function validateInvitationCode(code: string): Promise<ValidateInvitationCodeResponse> {
  const { data } = await apiClient.post<ValidateInvitationCodeResponse>('/auth/validate-invitation-code', { code })
  return data
}

/**
 * Forgot password request
 */
export interface ForgotPasswordRequest {
  email: string
  turnstile_token?: string
}

/**
 * Forgot password response
 */
export interface ForgotPasswordResponse {
  message: string
}

/**
 * Request password reset link
 * @param request - Email and optional Turnstile token
 * @returns Response with message
 */
export async function forgotPassword(request: ForgotPasswordRequest): Promise<ForgotPasswordResponse> {
  const { data } = await apiClient.post<ForgotPasswordResponse>('/auth/forgot-password', request)
  return data
}

/**
 * Reset password request
 */
export interface ResetPasswordRequest {
  email: string
  token: string
  new_password: string
}

/**
 * Reset password response
 */
export interface ResetPasswordResponse {
  message: string
}

/**
 * Reset password with token
 * @param request - Email, token, and new password
 * @returns Response with message
 */
export async function resetPassword(request: ResetPasswordRequest): Promise<ResetPasswordResponse> {
  const { data } = await apiClient.post<ResetPasswordResponse>('/auth/reset-password', request)
  return data
}

/**
 * Complete LinuxDo OAuth registration by supplying an invitation code
 * @param pendingOAuthToken - Short-lived JWT from the OAuth callback
 * @param invitationCode - Invitation code entered by the user
 * @returns Token pair on success
 */
export async function completeLinuxDoOAuthRegistration(
  pendingOAuthToken: string,
  invitationCode: string,
  referralCode = ''
): Promise<{ access_token: string; refresh_token: string; expires_in: number; token_type: string }> {
  return completeOAuthRegistration('linuxdo', pendingOAuthToken, invitationCode, referralCode)
}

export async function completeOAuthRegistration(
  provider: 'linuxdo' | 'google' | 'github',
  pendingOAuthToken: string,
  invitationCode = '',
  referralCode = ''
): Promise<{ access_token: string; refresh_token: string; expires_in: number; token_type: string }> {
  const payload: {
    pending_oauth_token: string
    invitation_code?: string
    referral_code?: string
  } = {
    pending_oauth_token: pendingOAuthToken
  }
  const trimmedInvitationCode = invitationCode.trim()
  const trimmedReferralCode = referralCode.trim()
  if (trimmedInvitationCode) {
    payload.invitation_code = trimmedInvitationCode
  }
  if (trimmedReferralCode) {
    payload.referral_code = trimmedReferralCode
  }
  const { data } = await apiClient.post<{
    access_token: string
    refresh_token: string
    expires_in: number
    token_type: string
  }>(`/auth/oauth/${provider}/complete-registration`, payload)
  return data
}

export const authAPI = {
  login,
  login2FA,
  isTotp2FARequired,
  register,
  getCurrentUser,
  logout,
  isAuthenticated,
  getPublicSettings,
  sendVerifyCode,
  validatePromoCode,
  validateInvitationCode,
  forgotPassword,
  resetPassword,
  refreshToken,
  revokeAllSessions,
  issueSSOTicket,
  issueEmbedTicket,
  completeLinuxDoOAuthRegistration,
  completeOAuthRegistration
}

export default authAPI
