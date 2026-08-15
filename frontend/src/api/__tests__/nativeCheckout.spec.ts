import { beforeEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '../client'
import {
  getNativeCheckoutManualOfferStatus,
  requestNativeCheckoutManualOfferPurchase,
} from '../nativeCheckout'

vi.mock('../client', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
  },
}))

const mockClient = apiClient as unknown as {
  get: ReturnType<typeof vi.fn>
  post: ReturnType<typeof vi.fn>
}

describe('native checkout manual offer API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads the claim status for the current authenticated account', async () => {
    mockClient.get.mockResolvedValue({
      data: { code: 'newcomer-balance-5-to-10', claimed: true },
    })

    await expect(getNativeCheckoutManualOfferStatus('newcomer-balance-5-to-10')).resolves.toEqual({
      code: 'newcomer-balance-5-to-10',
      claimed: true,
    })
    expect(mockClient.get).toHaveBeenCalledWith(
      '/native-checkout/manual-offers/newcomer-balance-5-to-10',
    )
  })

  it('requests a fresh server-authorized link immediately before navigation', async () => {
    mockClient.post.mockResolvedValue({
      data: {
        code: 'newcomer-balance-5-to-10',
        claimed: false,
        purchase_url: 'https://pay.ldxp.cn/item/oc3w4r',
      },
    })

    const status = await requestNativeCheckoutManualOfferPurchase('newcomer-balance-5-to-10')

    expect(mockClient.post).toHaveBeenCalledWith(
      '/native-checkout/manual-offers/newcomer-balance-5-to-10/purchase',
    )
    expect(status.purchase_url).toBe('https://pay.ldxp.cn/item/oc3w4r')
  })
})
