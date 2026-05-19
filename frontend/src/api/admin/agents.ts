import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface CommissionRates {
  consumption_rate: number
  first_recharge_invitee_rate: number
  first_recharge_referral_rate: number
  invite_activity?: InviteActivityConfig
  updated_at?: string
}

export interface InviteActivityConfig {
  enabled: boolean
  name: string
  start_at?: string | null
  end_at?: string | null
  registration_bonus_amount: number
  updated_at?: string
}

export interface AgentRateConfig {
  agent_id: number
  consumption_rate: number
  enabled: boolean
  effective_consumption_rate: number
  rate_source: string
  override_enabled?: boolean
}

export interface AdminAgentSummary {
  agent_id: number
  email: string
  username: string
  status: string
  invite_code?: string
  created_at: string
  last_active_at?: string | null
  invited_user_count: number
  total_consumption: number
  period_consumption: number
  total_commission: number
  period_commission: number
  this_month_commission: number
  settled_commission: number
  unsettled_commission: number
  consumption_rate: number
  rate_source: string
  override_enabled: boolean
  override_consumption_rate?: number
  current_level?: string
  permanent_level?: string
  temporary_level?: string
  base_rate?: number
  last_evaluated_period?: string
  last_month_consumption?: number
  next_level_key?: string
  next_level_gap?: number
  settlement_minimum_amount: number
  settlement_eligible: boolean
  settlement_gap: number
  payment_profile_complete: boolean
  payment_profile_updated_at?: string
}

export interface AdminAgentUserStat {
  user_id: number
  email: string
  username: string
  joined_at: string
  first_invited_topup_at?: string
  total_recharged: number
  total_consumption: number
  period_consumption: number
  total_commission: number
  period_commission: number
  commission_record_count: number
  last_commission_at?: string
}

export interface BindAgentUserPayload {
  email: string
  overwrite_agent?: boolean
}

export interface BindAgentUserResult {
  user_id: number
  email: string
  username: string
  agent_id: number
  previous_agent_id?: number
  inviter_id?: number
  inviter_id_changed: boolean
  already_bound: boolean
  overwritten: boolean
}

export interface AdminAgentCommissionRecord {
  id: number
  beneficiary_id: number
  user_id: number
  user_email: string
  user_username: string
  amount: number
  source_amount: number
  type: string
  rate?: number
  rate_source?: string
  source_id?: number
  note?: string
  created_at: string
}

export interface AgentSettlement {
  id: number
  agent_id: number
  amount: number
  operator_id: number
  note: string
  status: string
  payment_alipay_real_name?: string
  payment_alipay_account?: string
  payment_contact_phone?: string
  payment_note?: string
  payment_qr_object_key?: string
  payment_reference?: string
  created_at: string
}

export interface AgentSettlementSettings {
  minimum_amount: number
  updated_at?: string
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

export interface AgentLevelRule {
  level_key: string
  level_name: string
  rate: number
  monthly_consumption_threshold?: number | null
  cumulative_consumption_threshold?: number | null
  sort_order: number
  enabled: boolean
  updated_at?: string
}

export interface AgentLevelState {
  agent_id: number
  base_level_key: string
  base_rate: number
  permanent_level_key: string
  temporary_level_key?: string
  current_level_key: string
  current_rate: number
  rate_source: string
  last_evaluated_period: string
  last_month_consumption: number
  total_consumption: number
  next_level_key?: string
  next_level_gap: number
  evaluated_at?: string
}

export interface AgentLevelEvaluationResult {
  agent_id: number
  state: AgentLevelState
}

export interface AgentLevelEvaluationRunResult {
  period: string
  evaluated_count: number
  failed_count: number
  items?: AgentLevelEvaluationResult[]
}

export interface AgentListParams {
  page?: number
  page_size?: number
  search?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
  settlement_status?: 'eligible' | 'below_threshold' | 'missing_profile' | ''
  start?: string
  end?: string
}

export async function list(params?: AgentListParams): Promise<PaginatedResponse<AdminAgentSummary>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminAgentSummary>>('/admin/agents', { params })
  return data
}

export async function listSettlementCandidates(params?: AgentListParams): Promise<PaginatedResponse<AdminAgentSummary>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminAgentSummary>>('/admin/agents/settlement-candidates', { params })
  return data
}

export async function get(agentId: number, params?: { start?: string; end?: string }): Promise<AdminAgentSummary> {
  const { data } = await apiClient.get<AdminAgentSummary>(`/admin/agents/${agentId}`, { params })
  return data
}

export async function listUsers(agentId: number, params?: AgentListParams): Promise<PaginatedResponse<AdminAgentUserStat>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminAgentUserStat>>(`/admin/agents/${agentId}/users`, { params })
  return data
}

export async function bindUser(agentId: number, payload: BindAgentUserPayload): Promise<BindAgentUserResult> {
  const { data } = await apiClient.post<BindAgentUserResult>(`/admin/agents/${agentId}/users/bind`, payload)
  return data
}

export async function listCommissions(agentId: number, params?: AgentListParams & { type?: string }): Promise<PaginatedResponse<AdminAgentCommissionRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminAgentCommissionRecord>>(`/admin/agents/${agentId}/commissions`, { params })
  return data
}

export async function listSettlements(agentId: number, params?: { page?: number; page_size?: number }): Promise<PaginatedResponse<AgentSettlement>> {
  const { data } = await apiClient.get<PaginatedResponse<AgentSettlement>>(`/admin/agents/${agentId}/settlements`, { params })
  return data
}

export async function createSettlement(agentId: number, amount: number, note?: string, paymentReference?: string): Promise<AgentSettlement> {
  const { data } = await apiClient.post<AgentSettlement>(`/admin/agents/${agentId}/settlements`, {
    amount,
    note: note || '',
    payment_reference: paymentReference || ''
  })
  return data
}

export async function getSettlementSettings(): Promise<AgentSettlementSettings> {
  const { data } = await apiClient.get<AgentSettlementSettings>('/admin/agents/settlement-settings')
  return data
}

export async function updateSettlementSettings(payload: AgentSettlementSettings): Promise<AgentSettlementSettings> {
  const { data } = await apiClient.put<AgentSettlementSettings>('/admin/agents/settlement-settings', payload)
  return data
}

export async function getPaymentProfile(agentId: number): Promise<AgentPaymentProfile> {
  const { data } = await apiClient.get<AgentPaymentProfile>(`/admin/agents/${agentId}/payment-profile`)
  return data
}

export async function getPaymentQRCode(agentId: number): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/admin/agents/${agentId}/payment-profile/alipay-qr`, { responseType: 'blob' })
  return data
}

export async function getRates(): Promise<CommissionRates> {
  const { data } = await apiClient.get<CommissionRates>('/admin/agents/rates')
  return data
}

export async function updateRates(rates: CommissionRates): Promise<CommissionRates> {
  const { data } = await apiClient.put<CommissionRates>('/admin/agents/rates', rates)
  return data
}

export async function getInviteActivity(): Promise<InviteActivityConfig> {
  const rates = await getRates()
  return rates.invite_activity || defaultInviteActivityConfig()
}

export async function updateInviteActivity(activity: InviteActivityConfig): Promise<InviteActivityConfig> {
  const currentRates = await getRates()
  const { data } = await apiClient.put<CommissionRates>('/admin/agents/rates', {
    consumption_rate: currentRates.consumption_rate,
    first_recharge_invitee_rate: currentRates.first_recharge_invitee_rate,
    first_recharge_referral_rate: currentRates.first_recharge_referral_rate,
    invite_activity: activity
  })
  return data.invite_activity || activity
}

function defaultInviteActivityConfig(): InviteActivityConfig {
  return {
    enabled: false,
    name: '公测邀请活动',
    start_at: null,
    end_at: null,
    registration_bonus_amount: 5
  }
}

export async function getAgentRate(agentId: number): Promise<AgentRateConfig> {
  const { data } = await apiClient.get<AgentRateConfig>(`/admin/agents/${agentId}/rate`)
  return data
}

export async function updateAgentRate(agentId: number, payload: { consumption_rate: number; enabled: boolean }): Promise<AgentRateConfig> {
  const { data } = await apiClient.put<AgentRateConfig>(`/admin/agents/${agentId}/rate`, payload)
  return data
}

export async function getLevelRules(): Promise<AgentLevelRule[]> {
  const { data } = await apiClient.get<AgentLevelRule[]>('/admin/agents/level-rules')
  return data
}

export async function updateLevelRules(rules: AgentLevelRule[]): Promise<AgentLevelRule[]> {
  const { data } = await apiClient.put<AgentLevelRule[]>('/admin/agents/level-rules', { rules })
  return data
}

export async function runLevelEvaluations(): Promise<AgentLevelEvaluationRunResult> {
  const { data } = await apiClient.post<AgentLevelEvaluationRunResult>('/admin/agents/level-evaluations/run')
  return data
}

export async function runAgentLevelEvaluation(agentId: number): Promise<AgentLevelEvaluationResult> {
  const { data } = await apiClient.post<AgentLevelEvaluationResult>(`/admin/agents/${agentId}/level-evaluations/run`)
  return data
}

export const agentsAPI = {
  list,
  listSettlementCandidates,
  get,
  listUsers,
  bindUser,
  listCommissions,
  listSettlements,
  createSettlement,
  getSettlementSettings,
  updateSettlementSettings,
  getPaymentProfile,
  getPaymentQRCode,
  getRates,
  updateRates,
  getInviteActivity,
  updateInviteActivity,
  getAgentRate,
  updateAgentRate,
  getLevelRules,
  updateLevelRules,
  runLevelEvaluations,
  runAgentLevelEvaluation
}

export default agentsAPI
