import { beforeEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '../client'
import { getPublicStats } from '../publicStats'

vi.mock('../client', () => ({
  apiClient: {
    get: vi.fn(),
  },
}))

const mockClient = apiClient as unknown as {
  get: ReturnType<typeof vi.fn>
}

describe('public stats API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches the landing counters from the public endpoint without auth', async () => {
    mockClient.get.mockResolvedValue({
      data: {
        tokens_total: 221_911_140_000,
        requests_total: 1_234_567,
        compensation_cny: 17_838.0,
        updated_at: '2026-08-25T00:00:00Z',
      },
    })

    await expect(getPublicStats()).resolves.toEqual({
      tokens_total: 221_911_140_000,
      requests_total: 1_234_567,
      compensation_cny: 17_838.0,
      updated_at: '2026-08-25T00:00:00Z',
    })
    expect(mockClient.get).toHaveBeenCalledWith('/public/stats')
  })
})
