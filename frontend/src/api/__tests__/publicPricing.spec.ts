import { beforeEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '../client'
import { ANONYMOUS_REQUEST_HEADER } from '../client'
import { getPublicModelPricing } from '../publicPricing'

vi.mock('../client', () => ({
  ANONYMOUS_REQUEST_HEADER: 'X-Anonymous-Request',
  apiClient: { get: vi.fn() },
}))

const mockClient = apiClient as unknown as { get: ReturnType<typeof vi.fn> }

describe('public model pricing API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.spyOn(Date, 'now').mockReturnValue(1_788_196_800_000)
  })

  it('cache-busts the catalog so an older null-model response cannot hide the directory', async () => {
    const catalog = { updated_at: '2026-09-01T00:00:00Z', currency: 'CNY', unit: 'per_1m_tokens', groups: [] }
    mockClient.get.mockResolvedValue({ data: catalog })

    await expect(getPublicModelPricing()).resolves.toEqual(catalog)
    expect(mockClient.get).toHaveBeenCalledWith('/public/model-pricing', {
      params: { _nc: 1_788_196_800_000 },
      headers: { [ANONYMOUS_REQUEST_HEADER]: '1' },
    })
  })
})
