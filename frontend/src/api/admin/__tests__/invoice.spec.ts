import {beforeEach, describe, expect, it, vi} from 'vitest'
import {apiClient} from '../../client'
import {
    completeInvoiceRequest,
    exportInvoiceRequests,
    listInvoiceRequests,
    listTopupOrders,
    rejectInvoiceRequest,
} from '../invoice'

// Mock the API client
vi.mock('../../client', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

const mockClient = apiClient as unknown as {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('Invoice API - Admin', () => {
  describe('listTopupOrders', () => {
    it('should call GET /admin/topup/orders with params', async () => {
      const params = { page: 1, page_size: 10, search: 'user@test.com' }
      mockClient.get.mockResolvedValue({
        data: { items: [], total: 0 },
      })

      const result = await listTopupOrders(params)
      expect(mockClient.get).toHaveBeenCalledWith('/admin/topup/orders', { params })
      expect(result.total).toBe(0)
    })
  })

  describe('listInvoiceRequests', () => {
    it('should call GET /admin/invoice/requests', async () => {
      const params = { status: 'pending', page: 1 }
      mockClient.get.mockResolvedValue({
        data: { items: [], total: 0, pending_count: 3 },
      })

      const result = await listInvoiceRequests(params)
      expect(mockClient.get).toHaveBeenCalledWith('/admin/invoice/requests', { params })
      expect(result.pending_count).toBe(3)
    })
  })

  describe('completeInvoiceRequest', () => {
    it('should call POST /admin/invoice/requests/:id/complete', async () => {
      mockClient.post.mockResolvedValue({ data: { message: 'completed' } })

      const result = await completeInvoiceRequest(42)
      expect(mockClient.post).toHaveBeenCalledWith('/admin/invoice/requests/42/complete')
      expect(result.message).toBe('completed')
    })
  })

  describe('rejectInvoiceRequest', () => {
    it('should call POST /admin/invoice/requests/:id/reject with reason', async () => {
      mockClient.post.mockResolvedValue({ data: { message: 'rejected' } })

      const result = await rejectInvoiceRequest(42, '信息不完整')
      expect(mockClient.post).toHaveBeenCalledWith(
        '/admin/invoice/requests/42/reject',
        { reject_reason: '信息不完整' },
      )
      expect(result.message).toBe('rejected')
    })
  })

  describe('exportInvoiceRequests', () => {
    it('should call POST /admin/invoice/requests/export with blob response', async () => {
      const blob = new Blob(['test'], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
      mockClient.post.mockResolvedValue({ data: blob })

      const result = await exportInvoiceRequests({ status: 'pending' })
      expect(mockClient.post).toHaveBeenCalledWith(
        '/admin/invoice/requests/export',
        { status: 'pending' },
        { responseType: 'blob' },
      )
      expect(result).toBeInstanceOf(Blob)
    })

    it('should default to empty payload when no params', async () => {
      const blob = new Blob(['test'])
      mockClient.post.mockResolvedValue({ data: blob })

      await exportInvoiceRequests()
      expect(mockClient.post).toHaveBeenCalledWith(
        '/admin/invoice/requests/export',
        {},
        { responseType: 'blob' },
      )
    })
  })
})
