import {apiClient} from './client'
import type {
    CreateInvoiceRequestResponse,
    InvoiceProfile,
    InvoiceProfileInput,
    InvoiceRequestListResponse,
    TopupOrderInvoiceStatus,
    TopupOrderListResponse,
    TopupOrderPaymentStatus,
} from '@/types'

export async function listMyTopupOrders(params?: {
  page?: number
  page_size?: number
  status?: TopupOrderPaymentStatus | ''
  invoice_status?: TopupOrderInvoiceStatus | ''
  start_time?: string
  end_time?: string
}): Promise<TopupOrderListResponse> {
  const { data } = await apiClient.get<TopupOrderListResponse>('/topup/orders', {
    params,
  })
  return data
}

export async function listInvoiceProfiles(): Promise<InvoiceProfile[]> {
  const { data } = await apiClient.get<InvoiceProfile[]>('/invoice/profiles')
  return data
}

export async function createInvoiceProfile(payload: InvoiceProfileInput): Promise<InvoiceProfile> {
  const { data } = await apiClient.post<InvoiceProfile>('/invoice/profiles', payload)
  return data
}

export async function updateInvoiceProfile(id: number, payload: InvoiceProfileInput): Promise<InvoiceProfile> {
  const { data } = await apiClient.put<InvoiceProfile>(`/invoice/profiles/${id}`, payload)
  return data
}

export async function deleteInvoiceProfile(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/invoice/profiles/${id}`)
  return data
}

export async function setDefaultInvoiceProfile(id: number): Promise<InvoiceProfile> {
  const { data } = await apiClient.put<InvoiceProfile>(`/invoice/profiles/${id}/default`)
  return data
}

export async function createInvoiceRequest(payload: {
  profile_id: number
  order_ids: number[]
  remark?: string | null
}): Promise<CreateInvoiceRequestResponse> {
  const { data } = await apiClient.post<CreateInvoiceRequestResponse>('/invoice/requests', payload)
  return data
}

export async function listInvoiceRequests(params?: {
  page?: number
  page_size?: number
  status?: string
  search?: string
  start_time?: string
  end_time?: string
}): Promise<InvoiceRequestListResponse> {
  const { data } = await apiClient.get<InvoiceRequestListResponse>('/invoice/requests', {
    params,
  })
  return data
}

export const invoiceAPI = {
  listMyTopupOrders,
  listInvoiceProfiles,
  createInvoiceProfile,
  updateInvoiceProfile,
  deleteInvoiceProfile,
  setDefaultInvoiceProfile,
  createInvoiceRequest,
  listInvoiceRequests,
}

export default invoiceAPI
