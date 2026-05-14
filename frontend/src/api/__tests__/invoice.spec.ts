import {beforeEach, describe, expect, it, vi} from 'vitest'
import {apiClient} from '../client'
import {
    createInvoiceProfile,
    createInvoiceRequest,
    deleteInvoiceProfile,
    listInvoiceProfiles,
    listInvoiceRequests,
    listMyTopupOrders,
    setDefaultInvoiceProfile,
    updateInvoiceProfile,
} from '../invoice'

// Mock the API client
vi.mock('../client', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

const mockClient = apiClient as unknown as {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
  put: ReturnType<typeof vi.fn>
  delete: ReturnType<typeof vi.fn>
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('Invoice API - User', () => {
  describe('listMyTopupOrders', () => {
    it('should call GET /topup/orders with params', async () => {
      const mockResponse = {
        data: { items: [], total: 0, selectable_amount_fen: 0 },
      }
      mockClient.get.mockResolvedValue(mockResponse)

      const params = { page: 1, page_size: 10, status: 'completed' as const }
      const result = await listMyTopupOrders(params)

      expect(mockClient.get).toHaveBeenCalledWith('/topup/orders', { params })
      expect(result).toEqual(mockResponse.data)
    })

    it('should work without params', async () => {
      mockClient.get.mockResolvedValue({ data: { items: [], total: 0 } })
      await listMyTopupOrders()
      expect(mockClient.get).toHaveBeenCalledWith('/topup/orders', { params: undefined })
    })
  })

  describe('listInvoiceProfiles', () => {
    it('should call GET /invoice/profiles', async () => {
      const profiles = [
        { id: 1, title: '公司A', tax_number: '123', is_default: true },
      ]
      mockClient.get.mockResolvedValue({ data: profiles })

      const result = await listInvoiceProfiles()
      expect(mockClient.get).toHaveBeenCalledWith('/invoice/profiles')
      expect(result).toEqual(profiles)
    })
  })

  describe('createInvoiceProfile', () => {
    it('should call POST /invoice/profiles with payload', async () => {
      const payload = {
        title: '公司B',
        tax_number: '456',
        email: 'test@example.com',
        address: '地址',
        phone: '123456',
        bank_name: '银行',
        bank_account: '账号',
      }
      const created = { id: 2, ...payload, is_default: false }
      mockClient.post.mockResolvedValue({ data: created })

      const result = await createInvoiceProfile(payload)
      expect(mockClient.post).toHaveBeenCalledWith('/invoice/profiles', payload)
      expect(result).toEqual(created)
    })
  })

  describe('updateInvoiceProfile', () => {
    it('should call PUT /invoice/profiles/:id', async () => {
      const payload = { title: '更新', tax_number: '789' }
      mockClient.put.mockResolvedValue({ data: { id: 1, ...payload } })

      const result = await updateInvoiceProfile(1, payload)
      expect(mockClient.put).toHaveBeenCalledWith('/invoice/profiles/1', payload)
      expect(result.id).toBe(1)
    })
  })

  describe('deleteInvoiceProfile', () => {
    it('should call DELETE /invoice/profiles/:id', async () => {
      mockClient.delete.mockResolvedValue({ data: { message: 'ok' } })

      const result = await deleteInvoiceProfile(5)
      expect(mockClient.delete).toHaveBeenCalledWith('/invoice/profiles/5')
      expect(result.message).toBe('ok')
    })
  })

  describe('setDefaultInvoiceProfile', () => {
    it('should call PUT /invoice/profiles/:id/default', async () => {
      const profile = { id: 3, title: 'X', is_default: true }
      mockClient.put.mockResolvedValue({ data: profile })

      const result = await setDefaultInvoiceProfile(3)
      expect(mockClient.put).toHaveBeenCalledWith('/invoice/profiles/3/default')
      expect(result.is_default).toBe(true)
    })
  })

  describe('createInvoiceRequest', () => {
    it('should call POST /invoice/requests with order_ids and profile_id', async () => {
      const payload = { profile_id: 1, order_ids: [10, 20], remark: '备注' }
      const response = { id: 99, serial_number: 'INV-001', total_amount_fen: 7000 }
      mockClient.post.mockResolvedValue({ data: response })

      const result = await createInvoiceRequest(payload)
      expect(mockClient.post).toHaveBeenCalledWith('/invoice/requests', payload)
      expect(result.serial_number).toContain('INV')
    })

    it('should work without remark', async () => {
      const payload = { profile_id: 1, order_ids: [10] }
      mockClient.post.mockResolvedValue({ data: { id: 100 } })

      await createInvoiceRequest(payload)
      expect(mockClient.post).toHaveBeenCalledWith('/invoice/requests', payload)
    })
  })

  describe('listInvoiceRequests', () => {
    it('should call GET /invoice/requests with filters', async () => {
      const params = { page: 1, page_size: 20, status: 'pending' }
      mockClient.get.mockResolvedValue({
        data: { items: [], total: 0, pending_count: 5 },
      })

      const result = await listInvoiceRequests(params)
      expect(mockClient.get).toHaveBeenCalledWith('/invoice/requests', { params })
      expect(result.pending_count).toBe(5)
    })
  })
})
