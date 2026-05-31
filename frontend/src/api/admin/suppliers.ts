import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export type SupplierStatus = 'evaluating' | 'active'
export type SupplierProbeStatus = 'unknown' | 'success' | 'failed'
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
  probe_enabled: boolean
  probe_model: string
  probe_interval_minutes: number
  last_probe_status: SupplierProbeStatus
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
  status: 'success' | 'failed'
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

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/suppliers/${id}`)
}

const suppliersAPI = { list, getById, create, update, probe, remove }
export default suppliersAPI
