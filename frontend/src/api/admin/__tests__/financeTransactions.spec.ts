import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../../client'
import financeTransactionsAPI, {
  create,
  deleteTransaction,
  getById,
  getReceiptUrl,
  list,
  summary,
  update,
  uploadReceipt
} from '../financeTransactions'

vi.mock('../../client', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn()
  }
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

describe('Finance Transactions API - Admin', () => {
  describe('list', () => {
    it('calls GET /admin/finance-transactions with pagination and filters', async () => {
      mockClient.get.mockResolvedValue({ data: { items: [], total: 0, page: 1, page_size: 20, pages: 0 } })

      const result = await list(2, 10, { type: 'expense', category: 'server_cost' })

      expect(mockClient.get).toHaveBeenCalledWith('/admin/finance-transactions', {
        params: { page: 2, page_size: 10, type: 'expense', category: 'server_cost' }
      })
      expect(result.total).toBe(0)
    })
  })

  describe('getById', () => {
    it('calls GET /admin/finance-transactions/:id', async () => {
      mockClient.get.mockResolvedValue({ data: { id: 5 } })
      const result = await getById(5)
      expect(mockClient.get).toHaveBeenCalledWith('/admin/finance-transactions/5')
      expect(result.id).toBe(5)
    })
  })

  describe('create', () => {
    it('calls POST /admin/finance-transactions with the payload', async () => {
      mockClient.post.mockResolvedValue({ data: { id: 1, type: 'income' } })

      const payload = {
        type: 'income' as const,
        category: 'sale_revenue' as const,
        amount_fen: 10000
      }
      const result = await create(payload)

      expect(mockClient.post).toHaveBeenCalledWith('/admin/finance-transactions', payload)
      expect(result.id).toBe(1)
    })
  })

  describe('update', () => {
    it('calls PUT /admin/finance-transactions/:id with the payload', async () => {
      mockClient.put.mockResolvedValue({ data: { id: 7, amount_fen: 5000 } })

      const payload = { amount_fen: 5000 }
      const result = await update(7, payload)

      expect(mockClient.put).toHaveBeenCalledWith('/admin/finance-transactions/7', payload)
      expect(result.amount_fen).toBe(5000)
    })
  })

  describe('deleteTransaction', () => {
    it('calls DELETE /admin/finance-transactions/:id', async () => {
      mockClient.delete.mockResolvedValue({ data: { message: 'deleted' } })
      const result = await deleteTransaction(9)
      expect(mockClient.delete).toHaveBeenCalledWith('/admin/finance-transactions/9')
      expect(result.message).toBe('deleted')
    })
  })

  describe('summary', () => {
    it('calls GET /admin/finance-transactions/summary with from/to', async () => {
      mockClient.get.mockResolvedValue({
        data: { total_income_fen: 100, total_expense_fen: 40, net_profit_fen: 60, margin_percent: 60, by_category: [] }
      })

      const result = await summary(1000, 2000)

      expect(mockClient.get).toHaveBeenCalledWith('/admin/finance-transactions/summary', {
        params: { from: 1000, to: 2000 }
      })
      expect(result.net_profit_fen).toBe(60)
    })
  })

  describe('uploadReceipt', () => {
    it('calls POST /admin/finance-transactions/receipts with multipart form data', async () => {
      mockClient.post.mockResolvedValue({ data: { key: 'finance-receipts/2026/07/1-abc.webp', size_bytes: 123, content_type: 'image/webp' } })

      const blob = new Blob(['fake-image-bytes'], { type: 'image/webp' })
      const result = await uploadReceipt(blob, 'receipt.webp')

      expect(mockClient.post).toHaveBeenCalledWith(
        '/admin/finance-transactions/receipts',
        expect.any(FormData),
        { headers: { 'Content-Type': 'multipart/form-data' } }
      )
      expect(result.key).toBe('finance-receipts/2026/07/1-abc.webp')
    })
  })

  describe('getReceiptUrl', () => {
    it('calls GET /admin/finance-transactions/:id/receipt-url', async () => {
      mockClient.get.mockResolvedValue({ data: { url: 'https://example.com/signed', expires_in_seconds: 600 } })
      const result = await getReceiptUrl(3)
      expect(mockClient.get).toHaveBeenCalledWith('/admin/finance-transactions/3/receipt-url')
      expect(result.url).toBe('https://example.com/signed')
    })
  })

  it('exposes all methods on the default export', () => {
    expect(financeTransactionsAPI.list).toBe(list)
    expect(financeTransactionsAPI.getById).toBe(getById)
    expect(financeTransactionsAPI.create).toBe(create)
    expect(financeTransactionsAPI.update).toBe(update)
    expect(financeTransactionsAPI.delete).toBe(deleteTransaction)
    expect(financeTransactionsAPI.summary).toBe(summary)
    expect(financeTransactionsAPI.uploadReceipt).toBe(uploadReceipt)
    expect(financeTransactionsAPI.getReceiptUrl).toBe(getReceiptUrl)
  })
})
