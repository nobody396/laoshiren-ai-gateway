import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../../client'
import { listAgentQueue } from '../feedbacks'

vi.mock('../../client', () => ({
  apiClient: {
    get: vi.fn(),
  },
}))

const mockClient = apiClient as unknown as {
  get: ReturnType<typeof vi.fn>
}

describe('Admin feedback API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads the exact unreviewed agent queue used by admin alerts', async () => {
    mockClient.get.mockResolvedValue({
      data: { items: [], total: 4, page: 1, page_size: 5, pages: 1 },
    })

    const result = await listAgentQueue({ page: 1, pageSize: 5 })

    expect(mockClient.get).toHaveBeenCalledWith('/admin/feedbacks/agent-queue', {
      params: { page: 1, page_size: 5 },
    })
    expect(result.total).toBe(4)
  })
})
