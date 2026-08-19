import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useNativeCheckoutOffers } from '../useNativeCheckoutOffers'

const mocks = vi.hoisted(() => ({
  listOffers: vi.fn(),
}))

vi.mock('@/api/nativeCheckout', () => ({
  listNativeCheckoutOffers: mocks.listOffers,
}))

const easypayMonthlyOffer = {
  code: 'plus',
  name: 'Plus 月卡',
  description: '',
  product_kind: 'subscription' as const,
  provider: 'easypay' as const,
  pay_amount_cny_fen: 25900,
  benefit_amount_cny_fen: 25900,
  redeem_validity_days: 31,
  once_per_user: false,
  claimed: false,
}

const ldxpBalanceOffer = {
  code: 'newcomer-balance-5-to-10',
  name: '新人专享 · 10 元余额包',
  description: '',
  product_kind: 'balance' as const,
  provider: 'ldxp' as const,
  pay_amount_cny_fen: 500,
  benefit_amount_cny_fen: 1000,
  once_per_user: true,
  claimed: false,
}

describe('useNativeCheckoutOffers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listOffers.mockResolvedValue([easypayMonthlyOffer, ldxpBalanceOffer])
  })

  it('dedupes concurrent loads into one API call', async () => {
    const first = useNativeCheckoutOffers()
    const second = useNativeCheckoutOffers()
    const [a, b] = await Promise.all([first.loadOffers(), second.loadOffers()])
    expect(mocks.listOffers).toHaveBeenCalledTimes(1)
    expect(a).toHaveLength(2)
    expect(b).toHaveLength(2)
  })

  it('finds a native offer only by the product-id convention with easypay provider', async () => {
    const { loadOffers, findNativeOfferByCode } = useNativeCheckoutOffers()
    await loadOffers()

    expect(findNativeOfferByCode('plus', 'subscription')?.code).toBe('plus')
    expect(findNativeOfferByCode('plus', 'balance')).toBeUndefined()
    expect(findNativeOfferByCode('pro', 'subscription')).toBeUndefined()
    // LDXP 或其他通道的同名 offer 不触发站内扫码回退必须保持外链。
    expect(findNativeOfferByCode('newcomer-balance-5-to-10', 'balance')).toBeUndefined()
  })
})
