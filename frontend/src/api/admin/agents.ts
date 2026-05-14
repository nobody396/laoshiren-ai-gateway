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
  updateAgentRate
}

export default agentsAPI
