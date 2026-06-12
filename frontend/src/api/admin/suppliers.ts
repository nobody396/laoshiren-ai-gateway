import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type SupplierStatus = 'evaluating' | 'active'
export type SupplierProbeStatus = 'unknown' | 'success' | 'degraded' | 'failed'
export type SupplierContactPlatform = '' | 'wechat' | 'telegram' | 'qq' | 'email' | 'phone' | 'other'

export interface Supplier {
  id: number
  name: string
  website_url: string
  base_url: string
  api_key: string
  upstream_group: string
  contact_platform: SupplierContactPlatform
  contact_value: string
  status: SupplierStatus
  cost_rmb_per_usd: number | null
  notes: string
  source_account_id: number | null
  source_platform: string
  probe_enabled: boolean
  probe_model: string
  probe_interval_minutes: number
  last_probe_status: SupplierProbeStatus
  last_probe_sub_status: string
  last_probe_http_code: number | null
  last_probe_latency_ms: number | null
  last_probe_error: string
  last_probe_at: string | null
  next_probe_at: string | null
  probe_success_rate: number
  probe_success_count: number
  probe_total_count: number
  created_at: string
  updated_at: string
  target_group_ids: number[]
  target_group_count: number
}

export interface SupplierProbeResult {
  id: number
  supplier_id: number
  status: 'success' | 'degraded' | 'failed'
  sub_status: string
  http_code: number
  model: string
  latency_ms: number
  accuracy_ok: boolean
  response_text: string
  error_message: string
  checked_at: string
  created_at: string
}

export interface SupplierProbeResponse {
  supplier: Supplier
  result: SupplierProbeResult
}

export interface SupplierProbeSnapshotItem {
  id: number
  name: string
  source_account_id: number | null
  source_platform: string
  probe_enabled: boolean
  probe_model: string
  last_probe_status: SupplierProbeStatus
  last_probe_sub_status: string
  last_probe_latency_ms: number | null
  last_probe_error: string
  last_probe_at: string | null
  next_probe_at: string | null
  probe_success_rate: number
  probe_success_count: number
  probe_total_count: number
  window_total: number
  window_success: number
  window_degraded: number
  window_failed: number
  window_success_rate: number
  window_average_latency_ms: number
}

export interface SupplierHourlyStability {
  hour: number
  label: string
  status: SupplierProbeStatus
  total: number
  success: number
  degraded: number
  failed: number
  success_rate: number
  average_latency_ms: number
}

export interface SupplierProbeSnapshot {
  generated_at: string
  window_minutes: number
  days: number
  total_suppliers: number
  enabled_suppliers: number
  healthy_suppliers: number
  degraded_suppliers: number
  failed_suppliers: number
  unknown_suppliers: number
  window_total: number
  window_success: number
  window_degraded: number
  window_failed: number
  window_success_rate: number
  average_latency_ms: number
  suppliers: SupplierProbeSnapshotItem[]
  hourly: SupplierHourlyStability[]
}

export interface SupplierAccountSyncResult {
  created: number
  updated: number
  skipped: number
  skipped_accounts?: Array<{
    account_id: number
    account_name: string
    reason: string
  }>
}

export interface SupplierProbeBatchResult {
  total: number
  success: number
  degraded: number
  failed: number
  skipped: number
  results: Array<{
    supplier_id: number
    supplier_name: string
    result?: SupplierProbeResult
    error?: string
  }>
}

export interface CreateSupplierRequest {
  name: string
  website_url?: string
  base_url?: string
  api_key?: string
  upstream_group?: string
  contact_platform?: SupplierContactPlatform
  contact_value?: string
  status?: SupplierStatus
  cost_rmb_per_usd?: number | null
  notes?: string
  probe_enabled?: boolean
  probe_model?: string
  probe_interval_minutes?: number
  target_group_ids?: number[]
}

export type UpdateSupplierRequest = Partial<CreateSupplierRequest>

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    status?: string
    probe_status?: string
    search?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<Supplier>> {
  const { data } = await apiClient.get<PaginatedResponse<Supplier>>('/admin/suppliers', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    signal: options?.signal
  })
  return data
}

export async function getById(id: number): Promise<Supplier> {
  const { data } = await apiClient.get<Supplier>(`/admin/suppliers/${id}`)
  return data
}

export async function create(req: CreateSupplierRequest): Promise<Supplier> {
  const { data } = await apiClient.post<Supplier>('/admin/suppliers', req)
  return data
}

export async function update(id: number, req: UpdateSupplierRequest): Promise<Supplier> {
  const { data } = await apiClient.put<Supplier>(`/admin/suppliers/${id}`, req)
  return data
}

export async function probe(id: number): Promise<SupplierProbeResponse> {
  const { data } = await apiClient.post<SupplierProbeResponse>(`/admin/suppliers/${id}/probe`)
  return data
}

export async function getProbeSnapshot(
  params: { window_minutes?: number; days?: number } = {}
): Promise<SupplierProbeSnapshot> {
  const { data } = await apiClient.get<SupplierProbeSnapshot>('/admin/suppliers/probe-snapshot', {
    params
  })
  return data
}

export async function bulkSetProbeEnabled(ids: number[], enabled: boolean): Promise<{ affected: number }> {
  const { data } = await apiClient.put<{ affected: number }>('/admin/suppliers/probe-enabled', {
    ids,
    enabled
  })
  return data
}

export async function syncAccounts(): Promise<SupplierAccountSyncResult> {
  const { data } = await apiClient.post<SupplierAccountSyncResult>('/admin/suppliers/sync-accounts')
  return data
}

export async function probeAll(ids: number[] = []): Promise<SupplierProbeBatchResult> {
  const { data } = await apiClient.post<SupplierProbeBatchResult>('/admin/suppliers/probe-all', { ids })
  return data
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/suppliers/${id}`)
}

const suppliersAPI = {
  list,
  getById,
  create,
  update,
  probe,
  getProbeSnapshot,
  bulkSetProbeEnabled,
  syncAccounts,
  probeAll,
  remove
}
export default suppliersAPI
