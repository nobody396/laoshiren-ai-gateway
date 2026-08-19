import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  shouldShowManualNewcomerProduct,
  useManualNewcomerOffer,
} from '../useManualNewcomerOffer'

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  requestPurchase: vi.fn(),
  listOffers: vi.fn(),
}))

vi.mock('@/api/nativeCheckout', () => ({
  getNativeCheckoutManualOfferStatus: mocks.getStatus,
  requestNativeCheckoutManualOfferPurchase: mocks.requestPurchase,
  listNativeCheckoutOffers: mocks.listOffers,
}))

describe('useManualNewcomerOffer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listOffers.mockResolvedValue([])
  })

  it('fails closed until the current account is confirmed eligible', async () => {
    const offer = useManualNewcomerOffer()
    expect(offer.state.value).toBe('loading')
    expect(offer.canPurchase.value).toBe(false)
    expect(shouldShowManualNewcomerProduct(5, offer.state.value)).toBe(false)
    expect(shouldShowManualNewcomerProduct(20, offer.state.value)).toBe(true)

    mocks.getStatus.mockResolvedValue({ code: 'newcomer-balance-5-to-10', claimed: false })
    await offer.refresh()
    expect(offer.state.value).toBe('available')
    expect(shouldShowManualNewcomerProduct(5, offer.state.value)).toBe(true)

    mocks.getStatus.mockRejectedValue(new Error('network unavailable'))
    await offer.refresh()
    expect(offer.state.value).toBe('unavailable')
    expect(shouldShowManualNewcomerProduct(5, offer.state.value)).toBe(false)
  })

  it('removes the newcomer product when the account already claimed it', async () => {
    const offer = useManualNewcomerOffer()
    mocks.getStatus.mockResolvedValue({ code: 'newcomer-balance-5-to-10', claimed: true })

    await offer.refresh()

    expect(offer.state.value).toBe('claimed')
    expect(offer.canPurchase.value).toBe(false)
    expect(shouldShowManualNewcomerProduct(5, offer.state.value)).toBe(false)
  })

  it('rechecks at click time and never returns a purchase URL after a racing claim', async () => {
    const offer = useManualNewcomerOffer()
    mocks.getStatus.mockResolvedValue({ code: 'newcomer-balance-5-to-10', claimed: false })
    mocks.requestPurchase.mockResolvedValue({ code: 'newcomer-balance-5-to-10', claimed: true })
    await offer.refresh()

    await expect(offer.requestPurchaseURL()).resolves.toBe('')
    expect(offer.state.value).toBe('claimed')
  })

  it('returns only the server-authorized purchase URL for an eligible account', async () => {
    const offer = useManualNewcomerOffer()
    mocks.requestPurchase.mockResolvedValue({
      code: 'newcomer-balance-5-to-10',
      claimed: false,
      purchase_url: 'https://pay.ldxp.cn/item/oc3w4r',
    })

    await expect(offer.requestPurchaseURL()).resolves.toBe('https://pay.ldxp.cn/item/oc3w4r')
    expect(offer.state.value).toBe('available')
  })

  it('fails closed when the server does not return a purchase URL', async () => {
    const offer = useManualNewcomerOffer()
    mocks.requestPurchase.mockResolvedValue({ code: 'newcomer-balance-5-to-10', claimed: false })

    await expect(offer.requestPurchaseURL()).resolves.toBe('')
    expect(offer.state.value).toBe('unavailable')
  })

  it('switches to native mode when the newcomer balance offer is served by easypay', async () => {
    const offer = useManualNewcomerOffer()
    mocks.getStatus.mockResolvedValue({ code: 'newcomer-balance-5-to-10', claimed: false })
    mocks.listOffers.mockResolvedValue([{
      code: 'newcomer-balance-5-to-10',
      name: '新人专享 · 10 元余额包',
      description: '',
      product_kind: 'balance',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      once_per_user: true,
      claimed: false,
      provider: 'easypay',
    }])

    await offer.refresh()

    expect(offer.mode.value).toBe('native')
    expect(shouldShowManualNewcomerProduct(5, offer.state.value, offer.mode.value)).toBe(false)
    expect(shouldShowManualNewcomerProduct(20, offer.state.value, offer.mode.value)).toBe(true)
  })

  it('stays in manual mode for ldxp or provider-less offers', async () => {
    const offer = useManualNewcomerOffer()
    mocks.getStatus.mockResolvedValue({ code: 'newcomer-balance-5-to-10', claimed: false })
    mocks.listOffers.mockResolvedValue([{
      code: 'newcomer-balance-5-to-10',
      name: '新人专享 · 10 元余额包',
      description: '',
      product_kind: 'balance',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      once_per_user: true,
      claimed: false,
      provider: 'ldxp',
    }])

    await offer.refresh()

    expect(offer.mode.value).toBe('manual')
    expect(shouldShowManualNewcomerProduct(5, offer.state.value, offer.mode.value)).toBe(true)
  })

  it('fails closed to manual mode when the offers list cannot be loaded', async () => {
    const offer = useManualNewcomerOffer()
    mocks.getStatus.mockResolvedValue({ code: 'newcomer-balance-5-to-10', claimed: false })
    mocks.listOffers.mockRejectedValue(new Error('network unavailable'))

    await offer.refresh()

    expect(offer.mode.value).toBe('manual')
    expect(offer.state.value).toBe('available')
    expect(shouldShowManualNewcomerProduct(5, offer.state.value, offer.mode.value)).toBe(true)
  })
})
