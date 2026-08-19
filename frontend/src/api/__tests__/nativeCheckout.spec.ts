import { beforeEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '../client'
import {
  createNativeCheckoutOrder,
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

describe('native checkout order creation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('creates an order without pay_type when no method is specified (ldxp path)', async () => {
    mockClient.post.mockResolvedValue({
      data: { order_no: 'NC-1', status: 'pending', pay_amount_cny_fen: 500, benefit_amount_cny_fen: 1000 },
    })

    await createNativeCheckoutOrder('newcomer-balance-5-to-10')

    expect(mockClient.post).toHaveBeenCalledWith('/native-checkout/orders', {
      offer_code: 'newcomer-balance-5-to-10',
    })
  })

  it('passes pay_type through for easypay orders', async () => {
    mockClient.post.mockResolvedValue({
      data: {
        order_no: 'NC-2',
        status: 'pending',
        pay_amount_cny_fen: 500,
        benefit_amount_cny_fen: 1000,
        payment_url: 'weixin://wxpay/bizpayurl?pr=abc',
        payment_method: 'wechat',
      },
    })

    const order = await createNativeCheckoutOrder('newcomer-balance-5-to-10', 'wechat')

    expect(mockClient.post).toHaveBeenCalledWith('/native-checkout/orders', {
      offer_code: 'newcomer-balance-5-to-10',
      pay_type: 'wechat',
    })
    expect(order.payment_method).toBe('wechat')
  })
})
