/**
 * Agent (Commission) API endpoints
 * Handles invite codes, commission dashboard, invited users, and commission records
 */

import { apiClient } from './client'

export interface AgentDashboard {
  invited_user_count: number
  total_commission: number
  settled_commission: number
  unsettled_commission: number
  period_commission: number
  this_month_commission: number
  consumption_rate: number
  first_recharge_invitee_rate: number
  rate_source: string
  current_level?: string
  current_level_name?: string
  permanent_level?: string
  permanent_level_name?: string
  temporary_level?: string
  temporary_level_name?: string
  last_month_consumption?: number
  this_month_consumption?: number
  total_consumption?: number
  next_level_gap?: number
  last_evaluated_period?: string
  evaluated_at?: string
  assessment_period_start?: string
  assessment_period_end?: string
  next_assessment_at?: string
  next_monthly_progress?: AgentLevelProgress
  next_cumulative_progress?: AgentLevelProgress
  settlement_minimum_amount: number
  settlement_eligible: boolean
  settlement_gap: number
  payment_profile_complete: boolean
}

export interface AgentLevelProgress {
  level_key: string
  level_name: string
  rate: number
  threshold: number
  current_consumption: number
  gap: number
  progress: number
}

export interface InvitedUserStat {
  user_id: number
  email: string
  username: string
  joined_at: string
  total_consumption: number
  total_commission: number
}

export interface CommissionRecord {
  id: number
  beneficiary_id: number
  user_id: number
  amount: number
  source_amount: number
  type: string
  source_id?: number
  note?: string
  created_at: string
}

export interface PaginationResult {
  page: number
  page_size: number
  total: number
  total_pages: number
}

export interface InviteCodeResponse {
  invite_code: string
}

export interface ValidateReferralResponse {
  valid: boolean
}

export interface PaginatedInvitedUsers {
  items: InvitedUserStat[]
  pagination: PaginationResult
}

export interface PaginatedCommissions {
  items: CommissionRecord[]
  pagination: PaginationResult
}

export interface AgentPaymentProfile {
  agent_id: number
  alipay_real_name: string
  alipay_account: string
  contact_phone: string
  payment_note: string
  alipay_qr_original_filename?: string
  alipay_qr_size?: number
  alipay_qr_url?: string
  has_alipay_qr: boolean
  complete: boolean
  created_at?: string
  updated_at?: string
}

function toNumber(value: unknown, fallback = 0): number {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return fallback
}

function toStringValue(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function normalizePagination(value: unknown): PaginationResult {
  const raw = (value && typeof value === 'object' ? value : {}) as Record<string, unknown>
  return {
    page: toNumber(raw.page ?? raw.Page, 1),
    page_size: toNumber(raw.page_size ?? raw.PageSize, 20),
    total: toNumber(raw.total ?? raw.Total, 0),
    total_pages: toNumber(raw.total_pages ?? raw.Pages, 0)
  }
}

function normalizeInvitedUser(value: unknown): InvitedUserStat {
  const raw = (value && typeof value === 'object' ? value : {}) as Record<string, unknown>
  return {
    user_id: toNumber(raw.user_id ?? raw.UserID),
    email: toStringValue(raw.email ?? raw.Email),
    username: toStringValue(raw.username ?? raw.Username),
    joined_at: toStringValue(raw.joined_at ?? raw.joinedAt ?? raw.RegisteredAt),
    total_consumption: toNumber(raw.total_consumption ?? raw.totalConsumption ?? raw.consumed_amount ?? raw.ConsumedAmount),
    total_commission: toNumber(raw.total_commission ?? raw.totalCommission ?? raw.commission_amount ?? raw.CommissionAmount)
  }
}

function normalizeCommissionRecord(value: unknown): CommissionRecord {
  const raw = (value && typeof value === 'object' ? value : {}) as Record<string, unknown>
  const sourceID = raw.source_id ?? raw.sourceId ?? raw.SourceID
  const note = raw.note ?? raw.Note
  return {
    id: toNumber(raw.id ?? raw.ID),
    beneficiary_id: toNumber(raw.beneficiary_id ?? raw.beneficiaryId ?? raw.BeneficiaryID),
    user_id: toNumber(raw.user_id ?? raw.userId ?? raw.UserID),
    amount: toNumber(raw.amount ?? raw.Amount),
    source_amount: toNumber(raw.source_amount ?? raw.sourceAmount ?? raw.SourceAmount),
    type: toStringValue(raw.type ?? raw.Type),
    source_id: sourceID == null ? undefined : toNumber(sourceID),
    note: typeof note === 'string' ? note : undefined,
    created_at: toStringValue(raw.created_at ?? raw.createdAt ?? raw.CreatedAt)
  }
}

/**
 * Get current user's invite code (any authenticated user)
 */
export async function getMyInviteCode(): Promise<InviteCodeResponse> {
  const { data } = await apiClient.get<InviteCodeResponse>('/user/invite-code')
  return data
}

/**
 * Get agent's invite code (agent/admin only)
 */
export async function getAgentInviteCode(): Promise<InviteCodeResponse> {
  const { data } = await apiClient.get<InviteCodeResponse>('/agent/invite-code')
  return data
}

/**
 * Get agent dashboard stats
 */
export async function getAgentDashboard(params?: { start?: string; end?: string }): Promise<AgentDashboard> {
  const { data } = await apiClient.get<AgentDashboard>('/agent/dashboard', { params })
  return data
}

export async function getAgentPaymentProfile(): Promise<AgentPaymentProfile> {
  const { data } = await apiClient.get<AgentPaymentProfile>('/agent/payment-profile')
  return data
}

export async function updateAgentPaymentProfile(payload: Pick<AgentPaymentProfile, 'alipay_real_name' | 'alipay_account' | 'contact_phone' | 'payment_note'>): Promise<AgentPaymentProfile> {
  const { data } = await apiClient.put<AgentPaymentProfile>('/agent/payment-profile', payload)
  return data
}

export async function uploadAgentPaymentQRCode(file: File): Promise<AgentPaymentProfile> {
  const form = new FormData()
  form.append('file', file)
  const { data } = await apiClient.post<AgentPaymentProfile>('/agent/payment-profile/alipay-qr', form, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
  return data
}

export async function getAgentPaymentQRCode(): Promise<Blob> {
  const { data } = await apiClient.get<Blob>('/agent/payment-profile/alipay-qr', { responseType: 'blob' })
  return data
}

/**
 * Get invited users list with consumption stats
 */
export async function getAgentInvitedUsers(params?: {
  page?: number
  page_size?: number
  start?: string
  end?: string
}): Promise<PaginatedInvitedUsers> {
  const { data } = await apiClient.get<PaginatedInvitedUsers>('/agent/users', { params })
  return {
    items: Array.isArray(data?.items) ? data.items.map(normalizeInvitedUser) : [],
    pagination: normalizePagination(data?.pagination)
  }
}

/**
 * Get agent commission records
 */
export async function getAgentCommissions(params?: {
  page?: number
  page_size?: number
  type?: string
  start?: string
  end?: string
}): Promise<PaginatedCommissions> {
  const { data } = await apiClient.get<PaginatedCommissions>('/agent/commissions', { params })
  return {
    items: Array.isArray(data?.items) ? data.items.map(normalizeCommissionRecord) : [],
    pagination: normalizePagination(data?.pagination)
  }
}

/**
 * Validate a referral code (public endpoint)
 */
export async function validateReferralCode(code: string): Promise<ValidateReferralResponse> {
  const { data } = await apiClient.get<ValidateReferralResponse>('/validate-referral-code', {
    params: { code }
  })
  return data
}

export const agentAPI = {
  getMyInviteCode,
  getAgentInviteCode,
  getAgentDashboard,
  getAgentPaymentProfile,
  updateAgentPaymentProfile,
  uploadAgentPaymentQRCode,
  getAgentPaymentQRCode,
  getAgentInvitedUsers,
  getAgentCommissions,
  validateReferralCode
}

export default agentAPI
