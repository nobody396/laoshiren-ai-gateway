import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface CommissionRates {
  consumption_rate: number
  first_recharge_invitee_rate: number
  first_recharge_referral_rate: number
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
  start?: string
  end?: string
}

export async function list(params?: AgentListParams): Promise<PaginatedResponse<AdminAgentSummary>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminAgentSummary>>('/admin/agents', { params })
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

export async function createSettlement(agentId: number, amount: number, note?: string): Promise<AgentSettlement> {
  const { data } = await apiClient.post<AgentSettlement>(`/admin/agents/${agentId}/settlements`, { amount, note: note || '' })
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
  get,
  listUsers,
  bindUser,
  listCommissions,
  listSettlements,
  createSettlement,
  getRates,
  updateRates,
  getAgentRate,
  updateAgentRate,
  getLevelRules,
  updateLevelRules,
  runLevelEvaluations,
  runAgentLevelEvaluation
}

export default agentsAPI
