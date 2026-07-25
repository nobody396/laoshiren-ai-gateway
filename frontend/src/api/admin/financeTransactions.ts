/**
 * Admin Finance Ledger API endpoints
 *
 * 手工记账：真实现金进出，独立于 costAccounting.ts 的理论毛利率计算。
 */

import { apiClient } from '../client'
import type {
  BasePaginationResponse,
  CreateFinanceTransactionRequest,
  FinanceTransaction,
  FinanceTransactionSummary,
  UpdateFinanceTransactionRequest
} from '@/types'

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    type?: string
    category?: string
    search?: string
    from?: number
    to?: number
    sort_by?: 'occurred_at'
    sort_order?: 'asc' | 'desc'
  }
): Promise<BasePaginationResponse<FinanceTransaction>> {
  const { data } = await apiClient.get<BasePaginationResponse<FinanceTransaction>>('/admin/finance-transactions', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function getById(id: number): Promise<FinanceTransaction> {
  const { data } = await apiClient.get<FinanceTransaction>(`/admin/finance-transactions/${id}`)
  return data
}

export async function create(request: CreateFinanceTransactionRequest): Promise<FinanceTransaction> {
  const { data } = await apiClient.post<FinanceTransaction>('/admin/finance-transactions', request)
  return data
}

export async function update(id: number, request: UpdateFinanceTransactionRequest): Promise<FinanceTransaction> {
  const { data } = await apiClient.put<FinanceTransaction>(`/admin/finance-transactions/${id}`, request)
  return data
}

export async function deleteTransaction(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/finance-transactions/${id}`)
  return data
}

export async function summary(
  from?: number,
  to?: number,
  scope?: 'all' | 'month'
): Promise<FinanceTransactionSummary> {
  const params: Record<string, number | string> = {}
  if (from !== undefined) params.from = from
  if (to !== undefined) params.to = to
  if (scope !== undefined) params.scope = scope
  const { data } = await apiClient.get<FinanceTransactionSummary>('/admin/finance-transactions/summary', {
    params
  })
  return data
}

export async function uploadReceipt(
  file: Blob,
  filename: string
): Promise<{ key: string; size_bytes: number; content_type: string }> {
  const form = new FormData()
  form.append('file', file, filename)
  const { data } = await apiClient.post<{ key: string; size_bytes: number; content_type: string }>(
    '/admin/finance-transactions/receipts',
    form,
    { headers: { 'Content-Type': 'multipart/form-data' } }
  )
  return data
}

export async function getReceiptUrl(id: number): Promise<{ url: string; expires_in_seconds: number }> {
  const { data } = await apiClient.get<{ url: string; expires_in_seconds: number }>(
    `/admin/finance-transactions/${id}/receipt-url`
  )
  return data
}

const financeTransactionsAPI = {
  list,
  getById,
  create,
  update,
  delete: deleteTransaction,
  summary,
  uploadReceipt,
  getReceiptUrl
}

export default financeTransactionsAPI
