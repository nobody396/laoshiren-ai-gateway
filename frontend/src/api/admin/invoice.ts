import {apiClient} from '../client'
import type {
    InvoiceRequestListResponse,
    TopupOrderInvoiceStatus,
    TopupOrderListResponse,
    TopupOrderPaymentStatus,
} from '@/types'

export async function listTopupOrders(params?: {
  page?: number
  page_size?: number
  status?: TopupOrderPaymentStatus | ''
  invoice_status?: TopupOrderInvoiceStatus | ''
  search?: string
  start_time?: string
  end_time?: string
}): Promise<TopupOrderListResponse> {
  const { data } = await apiClient.get<TopupOrderListResponse>('/admin/topup/orders', {
    params,
  })
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
  const { data } = await apiClient.get<InvoiceRequestListResponse>('/admin/invoice/requests', {
    params,
  })
  return data
}

export async function completeInvoiceRequest(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/invoice/requests/${id}/complete`)
  return data
}

export async function rejectInvoiceRequest(id: number, rejectReason: string): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/invoice/requests/${id}/reject`, {
    reject_reason: rejectReason,
  })
  return data
}

export async function exportInvoiceRequests(payload?: {
  status?: string
  search?: string
  start_time?: string
  end_time?: string
  include_exported?: boolean
}): Promise<Blob> {
  const response = await apiClient.post('/admin/invoice/requests/export', payload ?? {}, {
    responseType: 'blob',
  })
  return response.data
}

export const invoiceAdminAPI = {
  listTopupOrders,
  listInvoiceRequests,
  completeInvoiceRequest,
  rejectInvoiceRequest,
  exportInvoiceRequests,
}

export default invoiceAdminAPI
