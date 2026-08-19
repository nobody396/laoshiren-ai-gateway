import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import NativeCheckoutTrialOffer from '../NativeCheckoutTrialOffer.vue'

const mocks = vi.hoisted(() => ({
  listOffers: vi.fn(),
  createOrder: vi.fn(),
  getOrder: vi.fn(),
  getDirectQR: vi.fn(),
  toDataURL: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  showSuccess: vi.fn(),
  refreshUser: vi.fn(),
  user: { email: 'buyer@example.com' },
}))

vi.mock('@/api/nativeCheckout', () => ({
  listNativeCheckoutOffers: mocks.listOffers,
  createNativeCheckoutOrder: mocks.createOrder,
  getNativeCheckoutOrder: mocks.getOrder,
  getNativeCheckoutDirectQR: mocks.getDirectQR,
}))

vi.mock('qrcode', () => ({ default: { toDataURL: mocks.toDataURL } }))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: mocks.showError, showInfo: mocks.showInfo, showSuccess: mocks.showSuccess }),
  useAuthStore: () => ({ refreshUser: mocks.refreshUser, user: mocks.user }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => params?.amount ? `${key}:${params.amount}` : key,
    }),
  }
})

const offer = {
  code: 'newcomer-balance-5-to-10',
  name: '新人专享 · 10 元余额包',
  description: 'internal pure-gift semantics',
  product_kind: 'balance' as const,
  pay_amount_cny_fen: 500,
  benefit_amount_cny_fen: 1000,
  once_per_user: true,
  claimed: false,
}

const monthlyOffer = {
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

describe('NativeCheckoutTrialOffer', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listOffers.mockResolvedValue([offer])
    mocks.refreshUser.mockResolvedValue({})
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders the five-to-ten offer without exposing its internal entitlement type or asking for contact details', async () => {
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain(offer.name)
    expect(wrapper.text()).toContain('nativeCheckout.onceOnly')
    expect(wrapper.text()).not.toContain(offer.description)
    expect(wrapper.text()).not.toContain('nativeCheckout.registeredEmail')
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.text()).toContain('¥5')
    expect(wrapper.text()).toContain('¥10')
    wrapper.unmount()
  })

  it('creates one order and honestly falls back to a payment-link QR', async () => {
    mocks.createOrder.mockResolvedValue({
      order_no: 'NC-1',
      status: 'pending',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      payment_url: 'https://pay.ldxp.cn/pay/NC-1',
      payment_method: 'wechat',
      direct_qr_url: '/native-checkout/orders/NC-1/qr',
    })
    mocks.getDirectQR.mockRejectedValue(new Error('provider challenge'))
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,LINKQR')
    mocks.getOrder.mockResolvedValue({
      order_no: 'NC-1',
      status: 'pending',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      payment_url: 'https://pay.ldxp.cn/pay/NC-1',
      payment_method: 'wechat',
    })
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    await wrapper.find('.trial-offer__action').trigger('click')
    await flushPromises()

    expect(mocks.createOrder).toHaveBeenCalledWith(offer.code, undefined)
    expect(mocks.getDirectQR).toHaveBeenCalledWith('NC-1')
    expect(mocks.toDataURL).toHaveBeenCalledWith(
      'https://pay.ldxp.cn/pay/NC-1',
      expect.objectContaining({ width: 320 }),
    )
    expect(wrapper.find('.checkout-modal__qr img').attributes('src')).toBe('data:image/png;base64,LINKQR')
    expect(wrapper.text()).toContain('nativeCheckout.wechatPay')
    expect(wrapper.text()).toContain('nativeCheckout.wechatScanInstruction')
    expect(wrapper.text()).toContain('nativeCheckout.linkQRHint')
    wrapper.unmount()
  })

  it('tells an Alipay order to scan with Alipay instead of WeChat', async () => {
    mocks.createOrder.mockResolvedValue({
      order_no: 'NC-ALIPAY',
      status: 'pending',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      payment_url: 'https://pay.ldxp.cn/pay/NC-ALIPAY',
      payment_method: 'alipay',
      direct_qr_url: '/native-checkout/orders/NC-ALIPAY/qr',
    })
    mocks.getDirectQR.mockRejectedValue(new Error('provider challenge'))
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,ALIPAYLINK')
    mocks.getOrder.mockResolvedValue({
      order_no: 'NC-ALIPAY',
      status: 'pending',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      payment_url: 'https://pay.ldxp.cn/pay/NC-ALIPAY',
      payment_method: 'alipay',
    })
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    await wrapper.find('.trial-offer__action').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('nativeCheckout.alipayPay')
    expect(wrapper.text()).toContain('nativeCheckout.alipayScanInstruction')
    expect(wrapper.text()).not.toContain('nativeCheckout.wechatScanInstruction')
    expect(wrapper.find('.checkout-modal__qr img').attributes('alt')).toBe('nativeCheckout.alipayQRAlt')
    wrapper.unmount()
  })

  it('does not let a completed once-only offer create another order', async () => {
    mocks.listOffers.mockResolvedValue([{
      ...offer,
      order: {
        order_no: 'NC-DONE', status: 'completed', pay_amount_cny_fen: 500, benefit_amount_cny_fen: 1000,
      },
    }])
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    const button = wrapper.find('.trial-offer__action')
    expect(button.attributes('disabled')).toBeDefined()
    await button.trigger('click')
    expect(mocks.createOrder).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('treats an already redeemed inventory card as the one allowed purchase even without an order row', async () => {
    mocks.listOffers.mockResolvedValue([{ ...offer, claimed: true }])
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    const button = wrapper.find('.trial-offer__action')
    expect(button.attributes('disabled')).toBeDefined()
    expect(button.text()).toContain('nativeCheckout.claimed')
    expect(wrapper.text()).toContain('nativeCheckout.completedHint')
    await button.trigger('click')
    expect(mocks.createOrder).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps a delayed paid order in a neutral checking state without saying manual review', async () => {
    vi.useFakeTimers()
    const checkingOrder = {
      order_no: 'NC-CHECKING', status: 'checking' as const,
      pay_amount_cny_fen: 500, benefit_amount_cny_fen: 1000,
      payment_method: 'wechat' as const,
      created_at: '2026-08-13T12:00:00Z',
    }
    mocks.listOffers.mockResolvedValue([{ ...offer, order: checkingOrder }])
    mocks.getOrder.mockResolvedValue(checkingOrder)
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(0)
    await flushPromises()

    expect(mocks.showError).not.toHaveBeenCalled()
    expect(mocks.showInfo).toHaveBeenCalledWith('nativeCheckout.checkingHint')
    const button = wrapper.find('.trial-offer__action')
    expect(button.attributes('disabled')).toBeUndefined()
    await button.trigger('click')
    expect(wrapper.find('.checkout-modal__card').exists()).toBe(true)
    expect(wrapper.find('.checkout-modal__checking').exists()).toBe(true)
    expect(wrapper.text()).toContain('nativeCheckout.orderCheckingTitle')
    wrapper.unmount()
  })

  it('shows the payment countdown and email fallback without provider-expiry or manual-redemption copy', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-13T12:00:00Z'))
    const pendingOrder = {
      order_no: 'NC-TIMED', status: 'pending' as const,
      pay_amount_cny_fen: 500, benefit_amount_cny_fen: 1000,
      payment_url: 'https://pay.ldxp.cn/pay/NC-TIMED',
      payment_method: 'wechat' as const,
      created_at: '2026-08-13T12:00:00Z',
    }
    mocks.listOffers.mockResolvedValue([{ ...offer, order: pendingOrder }])
    mocks.getOrder.mockResolvedValue(pendingOrder)
    mocks.getDirectQR.mockRejectedValue(new Error('provider challenge'))
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,TIMED')
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()
    await wrapper.find('.trial-offer__action').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('nativeCheckout.orderCreatedAt')
    expect(wrapper.text()).toContain('nativeCheckout.recommendedWindow')
    expect(wrapper.text()).toContain('nativeCheckout.automaticEta')
    expect(wrapper.text()).toContain('nativeCheckout.emailCheck')
    expect(wrapper.text()).toContain('buyer@example.com')
    expect(wrapper.text()).not.toContain('nativeCheckout.providerExpiryHint')
    expect(wrapper.text()).not.toContain('nativeCheckout.doNotRepeat')
    wrapper.unmount()
  })

  it('does not show the pay-method picker for ldxp offers', async () => {
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    expect(wrapper.find('.trial-offer__paymethod').exists()).toBe(false)
    wrapper.unmount()
  })

  it('creates easypay orders with the selected pay method and renders the QR client-side without the link fallback notice', async () => {
    const easypayOffer = { ...offer, provider: 'easypay' as const }
    mocks.listOffers.mockResolvedValue([easypayOffer])
    mocks.createOrder.mockResolvedValue({
      order_no: 'EP-1',
      status: 'pending',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      payment_url: 'https://pay.hueling.cc/submit.php?trade_no=EP-1',
      payment_method: 'alipay',
      created_at: '2026-08-19T12:00:00Z',
    })
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,EASYPAYQR')
    mocks.getOrder.mockResolvedValue({
      order_no: 'EP-1',
      status: 'pending',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      payment_url: 'https://pay.hueling.cc/submit.php?trade_no=EP-1',
      payment_method: 'alipay',
      created_at: '2026-08-19T12:00:00Z',
    })
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    const picker = wrapper.find('.trial-offer__paymethod')
    expect(picker.exists()).toBe(true)
    const options = wrapper.findAll('.trial-offer__paymethod-option')
    expect(options).toHaveLength(2)
    expect(options[0].classes()).toContain('trial-offer__paymethod-option--active')

    await wrapper.find('.trial-offer__action').trigger('click')
    await flushPromises()

    expect(mocks.createOrder).toHaveBeenCalledWith(offer.code, 'alipay')
    expect(mocks.getDirectQR).not.toHaveBeenCalled()
    expect(mocks.toDataURL).toHaveBeenCalledWith(
      'https://pay.hueling.cc/submit.php?trade_no=EP-1',
      expect.objectContaining({ width: 320 }),
    )
    expect(wrapper.find('.checkout-modal__qr img').attributes('src')).toBe('data:image/png;base64,EASYPAYQR')
    expect(wrapper.text()).not.toContain('nativeCheckout.linkQRHint')
    wrapper.unmount()
  })

  it('passes pay_type=wechat when the user picks WeChat before creating an easypay order', async () => {
    mocks.listOffers.mockResolvedValue([{ ...offer, provider: 'easypay' as const }])
    mocks.createOrder.mockResolvedValue({
      order_no: 'EP-2',
      status: 'pending',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      payment_url: 'weixin://wxpay/bizpayurl?pr=xyz',
      payment_method: 'wechat',
      created_at: '2026-08-19T12:00:00Z',
    })
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,WECHATQR')
    mocks.getOrder.mockResolvedValue({
      order_no: 'EP-2',
      status: 'pending',
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      payment_url: 'weixin://wxpay/bizpayurl?pr=xyz',
      payment_method: 'wechat',
      created_at: '2026-08-19T12:00:00Z',
    })
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    const options = wrapper.findAll('.trial-offer__paymethod-option')
    await options[1].trigger('click')
    await wrapper.find('.trial-offer__action').trigger('click')
    await flushPromises()

    expect(mocks.createOrder).toHaveBeenCalledWith(offer.code, 'wechat')
    expect(mocks.getDirectQR).not.toHaveBeenCalled()
    expect(mocks.toDataURL).toHaveBeenCalledWith(
      'weixin://wxpay/bizpayurl?pr=xyz',
      expect.objectContaining({ width: 320 }),
    )
    wrapper.unmount()
  })

  it('locks the pay-method picker while an easypay order is pending and keeps paying that order', async () => {
    const pendingOrder = {
      order_no: 'EP-3',
      status: 'pending' as const,
      pay_amount_cny_fen: 500,
      benefit_amount_cny_fen: 1000,
      payment_url: 'https://pay.hueling.cc/submit.php?trade_no=EP-3',
      payment_method: 'alipay' as const,
      created_at: '2026-08-19T12:00:00Z',
    }
    mocks.listOffers.mockResolvedValue([{ ...offer, provider: 'easypay' as const, order: pendingOrder }])
    mocks.getOrder.mockResolvedValue(pendingOrder)
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,PENDINGQR')
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    const options = wrapper.findAll('.trial-offer__paymethod-option')
    expect(options).toHaveLength(2)
    expect(options[0].attributes('disabled')).toBeDefined()
    expect(options[1].attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('nativeCheckout.payMethodLockedHint')

    await wrapper.find('.trial-offer__action').trigger('click')
    await flushPromises()

    expect(mocks.createOrder).not.toHaveBeenCalled()
    expect(mocks.getDirectQR).not.toHaveBeenCalled()
    expect(wrapper.find('.checkout-modal__qr img').attributes('src')).toBe('data:image/png;base64,PENDINGQR')
    wrapper.unmount()
  })

  it('loads the offer selected by the offerCode prop with monthly-card copy', async () => {
    mocks.listOffers.mockResolvedValue([offer, monthlyOffer])
    const wrapper = mount(NativeCheckoutTrialOffer, {
      props: { offerCode: 'plus' },
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Plus 月卡')
    expect(wrapper.text()).toContain('nativeCheckout.subscriptionBadge')
    expect(wrapper.text()).toContain('nativeCheckout.receiveSubscription')
    expect(wrapper.text()).toContain('nativeCheckout.validityDaysText')
    expect(wrapper.text()).toContain('nativeCheckout.buySubscriptionNow')
    expect(wrapper.text()).not.toContain('nativeCheckout.onceOnly')
    expect(wrapper.text()).not.toContain(offer.name)
    wrapper.unmount()
  })

  it('renders nothing when the offerCode prop matches no visible offer', async () => {
    mocks.listOffers.mockResolvedValue([offer])
    const wrapper = mount(NativeCheckoutTrialOffer, {
      props: { offerCode: 'pro' },
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    expect(wrapper.find('.trial-offer').exists()).toBe(false)
    wrapper.unmount()
  })

  it('lets a completed repeatable monthly offer be bought again instead of staying claimed', async () => {
    mocks.listOffers.mockResolvedValue([{
      ...monthlyOffer,
      order: {
        order_no: 'NC-DONE', status: 'completed', product_kind: 'subscription',
        pay_amount_cny_fen: 25900, benefit_amount_cny_fen: 25900, redeem_validity_days: 31,
      },
    }])
    mocks.createOrder.mockResolvedValue({
      order_no: 'NC-NEW',
      status: 'pending',
      provider: 'easypay',
      product_kind: 'subscription',
      pay_amount_cny_fen: 25900,
      benefit_amount_cny_fen: 25900,
      redeem_validity_days: 31,
      payment_url: 'https://pay.hueling.cc/submit.php?trade_no=NC-NEW',
      payment_method: 'alipay',
      created_at: '2026-08-19T12:00:00Z',
    })
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,RENEWQR')
    const wrapper = mount(NativeCheckoutTrialOffer, {
      props: { offerCode: 'plus' },
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    const button = wrapper.find('.trial-offer__action')
    expect(button.attributes('disabled')).toBeUndefined()
    expect(button.text()).toContain('nativeCheckout.buySubscriptionNow')
    await button.trigger('click')
    await flushPromises()

    expect(mocks.createOrder).toHaveBeenCalledWith('plus', 'alipay')
    expect(wrapper.find('.checkout-modal__qr img').attributes('src')).toBe('data:image/png;base64,RENEWQR')
    wrapper.unmount()
  })

  it('announces a completed monthly order as an activated subscription', async () => {
    vi.useFakeTimers()
    const pendingOrder = {
      order_no: 'NC-SUB', status: 'pending' as const,
      provider: 'easypay' as const, product_kind: 'subscription' as const,
      pay_amount_cny_fen: 25900, benefit_amount_cny_fen: 25900, redeem_validity_days: 31,
      payment_url: 'https://pay.hueling.cc/submit.php?trade_no=NC-SUB',
      payment_method: 'alipay' as const,
      created_at: '2026-08-19T12:00:00Z',
    }
    mocks.listOffers.mockResolvedValue([{ ...monthlyOffer, order: pendingOrder }])
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,SUBQR')
    mocks.getOrder.mockResolvedValue({ ...pendingOrder, status: 'completed' })
    const wrapper = mount(NativeCheckoutTrialOffer, {
      props: { offerCode: 'plus' },
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    await wrapper.find('.trial-offer__action').trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(0)
    await flushPromises()

    expect(mocks.showSuccess).toHaveBeenCalledWith('nativeCheckout.subscriptionCompletedToast')
    expect(mocks.refreshUser).toHaveBeenCalled()
    wrapper.unmount()
  })
})
