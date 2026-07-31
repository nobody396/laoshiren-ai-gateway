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
  email_restriction_enabled?: boolean
  email_suffix_whitelist?: string[]
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
  verification_status: 'incomplete' | 'pending_review' | 'verified' | 'rejected'
  verification_note?: string
  verified_at?: string
  verified_by?: number
  verified: boolean
  created_at?: string
  updated_at?: string
}

export interface AffiliateProgramSettings {
  id: number
  program_version: 'v3'
  mode: 'off' | 'shadow' | 'live'
  started_at?: string
  ordinary_referral_rate_bps: number
  ordinary_invitee_rate_bps: number
  first_paid_bonus_threshold_micros: number
  first_paid_bonus_micros: number
  agent_pool_rate_bps: number
  qualification_direct_user_count: number
  qualification_min_user_consumption_micros: number
  qualification_direct_team_consumption_micros: number
  qualification_self_consumption_micros: number
  max_campaign_links: number
  commission_conversion_multiplier_millis: number
  withdrawal_min_micros: number
  withdrawal_sla_hours: number
  margin_floor_bps: number
  operational_reserve_bps: number
  stress_cost_per_raw_credit_micros: number
  revision: number
  updated_at: string
}

export interface AffiliateAgentApplication {
  id: number
  user_id: number
  email: string
  username: string
  status: 'pending_review' | 'approved' | 'rejected' | 'cancelled'
  qualifying_route: 'direct_team' | 'self_consumption' | 'direct_volume'
  valid_direct_user_count: number
  self_consumption_micros: number
  direct_team_consumption_micros: number
  combined_consumption_micros: number
  application_note: string
  decision_note: string
  submitted_at: string
  reviewed_at?: string
  reviewed_by?: number
}

export interface AffiliateQualifiedCandidate {
  user_id: number
  email: string
  username: string
  qualification_route: 'direct_team' | 'self_consumption'
  valid_direct_user_count: number
  self_consumption_micros: number
  direct_team_consumption_micros: number
  combined_consumption_micros: number
}

export interface AffiliateOperationsSummary {
  qualified_followup: number
  pending_applications: number
  pending_payment_profiles: number
  processing_withdrawals: number
  overdue_withdrawals: number
  abnormal_partners: number
  actionable_total: number
}

export interface AffiliatePartnerPerformance {
  agent_id: number
  email: string
  username: string
  activated_at: string
  direct_user_count: number
  paid_direct_user_count: number
  self_recharge_micros: number
  direct_team_recharge_micros: number
  self_consumption_micros: number
  direct_team_consumption_micros: number
  recent_30d_consumption_micros: number
  lifetime_earned_micros: number
  available_commission_micros: number
  processing_withdrawal_micros: number
  paid_commission_micros: number
}

export interface AffiliatePartnerUserPerformance {
  user_id: number
  email: string
  username: string
  joined_at: string
  recharge_micros: number
  consumption_micros: number
  generated_commission_micros: number
}

export interface AffiliatePartnerCommissionEntry {
  id: number
  consumer_user_id: number
  entry_type: string
  posting_status: string
  amount_micros: number
  occurred_at: string
}

export interface AffiliatePartnerPerformanceDetail {
  summary: AffiliatePartnerPerformance
  period_start: string
  period_end: string
  direct_users: AffiliatePartnerUserPerformance[]
  commission_ledger: AffiliatePartnerCommissionEntry[]
  withdrawals: Array<{
    id: number
    amount_micros: number
    status: string
    requested_at: string
    paid_at?: string
    payment_reference?: string
    failure_reason?: string
  }>
}

export interface AffiliateCommunitySettings {
  enabled: boolean
  title: string
  message: string
  qr_original_filename?: string
  qr_size: number
  has_qr_code: boolean
  qr_code_url?: string
  revision: number
  updated_at?: string
}

export interface AffiliateCommercialPackage {
  id: string
  name: string
  kind: 'payg' | 'monthly'
  shop_price_cny: number
  direct_price_cny: number
  platform_credits: number
  daily_platform_credits?: number
  stress_cost_cny: number
  shop_stress_margin_percent: number
  direct_stress_margin_percent: number
  passes_configured_margin_gate: boolean
}

export interface AffiliateCommercialPolicy {
  cash_asset_symbol: string
  credit_asset_symbol: string
  shop_fee_bps: number
  max_reward_pool_bps: number
  max_reward_burden_bps: number
  operational_reserve_bps: number
  margin_floor_bps: number
  stress_cost_per_credit: number
  pricing_table_version: string
  gpt_cost_mix: {
    cheap_account_multiplier: number
    expensive_account_multiplier: number
    cheap_traffic_percent: number
    expensive_traffic_percent: number
    blended_account_multiplier: number
  }
  group_targets: Array<{ id: string; name: string; rate_multiplier: number }>
  packages: AffiliateCommercialPackage[]
  minimum_stress_margin_percent: number
  passes_configured_margin_gate: boolean
}

export interface AdminAffiliateWithdrawal {
  id: number
  agent_id: number
  amount_micros: number
  status: 'processing' | 'paid' | 'failed'
  agent_risk_status: AffiliateRiskStatus
  payment_alipay_real_name: string
  payment_alipay_account: string
  payment_contact_phone: string
  payment_note?: string
  requested_at: string
  due_at: string
  paid_at?: string
  failed_at?: string
  handled_by?: number
  payment_reference?: string
  failure_reason?: string
}

export type AdminAffiliateWithdrawalListStatus = AdminAffiliateWithdrawal['status'] | 'all'

export type AffiliateRiskStatus = 'clear' | 'review' | 'blocked'

export interface AffiliateRiskPrincipal {
  agent_id: number
  email: string
  username: string
  agent_status: string
  risk_status: AffiliateRiskStatus
  risk_note: string
  held_reward_count: number
  held_reward_micros: number
  held_cash_count: number
  held_cash_micros: number
  updated_at: string
  has_upstream: boolean
  self_commission_enabled: boolean
  self_commission_rate_bps: number
  self_commission_effective_at?: string
  self_commission_revision: number
  self_commission_reason?: string
  self_commission_updated_by?: number
  self_commission_updated_by_email?: string
  self_commission_updated_by_username?: string
  self_commission_updated_at?: string
  self_commission_eligible: boolean
  self_commission_block_reason?: string
}

export interface AffiliateSelfCommissionPolicy {
  agent_id: number
  enabled: boolean
  rate_bps: number
  effective_at?: string
  revision: number
  updated_by?: number
  reason?: string
  created_at?: string
  updated_at?: string
  has_upstream: boolean
  eligible: boolean
  block_reason_code?: string
}

export interface AffiliateRiskActionResult {
  id: number
  agent_id: number
  previous_risk_status: AffiliateRiskStatus
  next_risk_status: AffiliateRiskStatus
  reason: string
  released_reward_count: number
  released_reward_micros: number
  released_cash_count: number
  released_cash_micros: number
  created_at: string
}

export interface AffiliatePerformanceReversal {
  id: number
  original_event_id: number
  reversal_event_id: number
  consumer_user_id: number
  direct_agent_id?: number
  amount_micros: number
  reversed_reward_micros: number
  reversed_cash_micros: number
  reason: string
  operator_id: number
  created_at: string
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

export async function listPendingPaymentProfiles(limit = 100): Promise<AgentPaymentProfile[]> {
  const { data } = await apiClient.get<{ items: AgentPaymentProfile[] }>('/admin/agents/payment-profiles/pending', {
    params: { limit }
  })
  return data.items ?? []
}

export async function reviewPaymentProfile(
  agentId: number,
  payload: { status: 'verified' | 'rejected'; note?: string }
): Promise<AgentPaymentProfile> {
  const { data } = await apiClient.put<AgentPaymentProfile>(
    `/admin/agents/${agentId}/payment-profile/verification`,
    payload
  )
  return data
}

export async function getAffiliateProgram(): Promise<AffiliateProgramSettings> {
  const { data } = await apiClient.get<AffiliateProgramSettings>('/admin/agents/affiliate-program')
  return data
}

export async function updateAffiliateProgram(payload: AffiliateProgramSettings): Promise<AffiliateProgramSettings> {
  const { data } = await apiClient.put<AffiliateProgramSettings>('/admin/agents/affiliate-program', payload)
  return data
}

export async function listAffiliateApplications(
  status: AffiliateAgentApplication['status'] | 'all' = 'pending_review',
  limit = 100
): Promise<AffiliateAgentApplication[]> {
  const { data } = await apiClient.get<{ items: AffiliateAgentApplication[] }>('/admin/agents/affiliate-applications', {
    params: { status, limit }
  })
  return data.items ?? []
}

export async function listAffiliateQualifiedCandidates(limit = 100): Promise<AffiliateQualifiedCandidate[]> {
  const { data } = await apiClient.get<{ items: AffiliateQualifiedCandidate[] }>(
    '/admin/agents/affiliate-qualified-candidates',
    { params: { limit } }
  )
  return data.items ?? []
}

export async function getAffiliateOperationsSummary(): Promise<AffiliateOperationsSummary> {
  const { data } = await apiClient.get<AffiliateOperationsSummary>('/admin/agents/affiliate-operations-summary')
  return data
}

export async function listAffiliatePartnerPerformance(limit = 100): Promise<AffiliatePartnerPerformance[]> {
  const { data } = await apiClient.get<{ items: AffiliatePartnerPerformance[] }>('/admin/agents/affiliate-performance', {
    params: { limit }
  })
  return data.items ?? []
}

export async function getAffiliatePartnerPerformance(
  agentId: number,
  params?: { start?: string; end?: string }
): Promise<AffiliatePartnerPerformanceDetail> {
  const { data } = await apiClient.get<AffiliatePartnerPerformanceDetail>(
    `/admin/agents/${agentId}/affiliate-performance`,
    { params }
  )
  return data
}

export async function reviewAffiliateApplication(
  applicationId: number,
  payload: { approve: boolean; note?: string }
): Promise<{ application: AffiliateAgentApplication }> {
  const { data } = await apiClient.post<{ application: AffiliateAgentApplication }>(
    `/admin/agents/affiliate-applications/${applicationId}/review`,
    payload
  )
  return data
}

export async function getAffiliateCommercialPolicy(): Promise<AffiliateCommercialPolicy> {
  const { data } = await apiClient.get<AffiliateCommercialPolicy>('/admin/agents/affiliate-commercial-policy')
  return data
}

export async function getAffiliateCommunity(): Promise<AffiliateCommunitySettings> {
  const { data } = await apiClient.get<AffiliateCommunitySettings>('/admin/agents/affiliate-community')
  return data
}

export async function updateAffiliateCommunity(payload: Pick<AffiliateCommunitySettings, 'enabled' | 'title' | 'message' | 'revision'>): Promise<AffiliateCommunitySettings> {
  const { data } = await apiClient.put<AffiliateCommunitySettings>('/admin/agents/affiliate-community', payload)
  return data
}

export async function uploadAffiliateCommunityQRCode(file: File): Promise<AffiliateCommunitySettings> {
  const form = new FormData()
  form.append('file', file)
  const { data } = await apiClient.post<AffiliateCommunitySettings>('/admin/agents/affiliate-community/qr', form, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
  return data
}

export async function getAffiliateCommunityQRCode(): Promise<Blob> {
  const { data } = await apiClient.get<Blob>('/admin/agents/affiliate-community/qr', { responseType: 'blob' })
  return data
}

export async function listAffiliateWithdrawals(
  status: AdminAffiliateWithdrawalListStatus = 'processing',
  limit = 100
): Promise<AdminAffiliateWithdrawal[]> {
  const { data } = await apiClient.get<{ items: AdminAffiliateWithdrawal[] }>('/admin/agents/affiliate-withdrawals', {
    params: { status, limit }
  })
  return data.items ?? []
}

export async function completeAffiliateWithdrawal(id: number, paymentReference = ''): Promise<void> {
  await apiClient.post(`/admin/agents/affiliate-withdrawals/${id}/complete`, {
    payment_reference: paymentReference
  })
}

export async function failAffiliateWithdrawal(id: number, reason: string): Promise<void> {
  await apiClient.post(`/admin/agents/affiliate-withdrawals/${id}/fail`, { reason })
}

export async function getAffiliateWithdrawalQRCode(id: number): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/admin/agents/affiliate-withdrawals/${id}/payment-qr`, {
    responseType: 'blob'
  })
  return data
}

export async function listAffiliateRiskPrincipals(limit = 100): Promise<AffiliateRiskPrincipal[]> {
  const { data } = await apiClient.get<{ items: AffiliateRiskPrincipal[] }>('/admin/agents/affiliate-risk', {
    params: { limit }
  })
  return data.items ?? []
}

export async function updateAffiliateRisk(
  agentId: number,
  payload: { status: AffiliateRiskStatus; reason: string }
): Promise<AffiliateRiskActionResult> {
  const { data } = await apiClient.put<AffiliateRiskActionResult>(
    `/admin/agents/${agentId}/affiliate-risk`,
    payload
  )
  return data
}

export async function updateAffiliateSelfCommissionPolicy(
  agentId: number,
  payload: { enabled: boolean; expected_revision: number; reason: string }
): Promise<AffiliateSelfCommissionPolicy> {
  const { data } = await apiClient.put<AffiliateSelfCommissionPolicy>(
    `/admin/agents/${agentId}/self-commission-policy`,
    payload
  )
  return data
}

export async function reverseAffiliatePerformance(
  eventId: number,
  reason: string
): Promise<AffiliatePerformanceReversal> {
  const { data } = await apiClient.post<AffiliatePerformanceReversal>('/admin/agents/affiliate-reversals', {
    event_id: eventId,
    reason
  })
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
    registration_bonus_amount: 5,
    email_restriction_enabled: false,
    email_suffix_whitelist: []
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
  listPendingPaymentProfiles,
  reviewPaymentProfile,
  getAffiliateProgram,
  updateAffiliateProgram,
  listAffiliateQualifiedCandidates,
  getAffiliateOperationsSummary,
  listAffiliatePartnerPerformance,
  getAffiliatePartnerPerformance,
  listAffiliateApplications,
  reviewAffiliateApplication,
  getAffiliateCommercialPolicy,
  getAffiliateCommunity,
  updateAffiliateCommunity,
  uploadAffiliateCommunityQRCode,
  getAffiliateCommunityQRCode,
  listAffiliateWithdrawals,
  completeAffiliateWithdrawal,
  failAffiliateWithdrawal,
  getAffiliateWithdrawalQRCode,
  listAffiliateRiskPrincipals,
  updateAffiliateRisk,
  updateAffiliateSelfCommissionPolicy,
  reverseAffiliatePerformance,
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
