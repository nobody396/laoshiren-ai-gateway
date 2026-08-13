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
  showSuccess: vi.fn(),
  refreshUser: vi.fn(),
}))

vi.mock('@/api/nativeCheckout', () => ({
  listNativeCheckoutOffers: mocks.listOffers,
  createNativeCheckoutOrder: mocks.createOrder,
  getNativeCheckoutOrder: mocks.getOrder,
  getNativeCheckoutDirectQR: mocks.getDirectQR,
}))

vi.mock('qrcode', () => ({ default: { toDataURL: mocks.toDataURL } }))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: mocks.showError, showSuccess: mocks.showSuccess }),
  useAuthStore: () => ({ refreshUser: mocks.refreshUser }),
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
  code: 'trial-balance-1-to-5',
  name: '1 元体验，到账 5 元赠送额度',
  description: '5 元全部作为体验赠送额度发放，每个账号仅可购买一次。',
  product_kind: 'balance' as const,
  pay_amount_cny_fen: 100,
  benefit_amount_cny_fen: 500,
  once_per_user: true,
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

  it('renders the pure-gift once-only offer without asking for contact details', async () => {
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain(offer.name)
    expect(wrapper.text()).toContain(offer.description)
    expect(wrapper.text()).toContain('nativeCheckout.onceOnly')
    expect(wrapper.text()).toContain('nativeCheckout.registeredEmail')
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.text()).toContain('¥1')
    expect(wrapper.text()).toContain('¥5')
    wrapper.unmount()
  })

  it('creates one order and honestly falls back to a payment-link QR', async () => {
    mocks.createOrder.mockResolvedValue({
      order_no: 'NC-1',
      status: 'pending',
      pay_amount_cny_fen: 100,
      benefit_amount_cny_fen: 500,
      payment_url: 'https://pay.ldxp.cn/pay/NC-1',
      payment_method: 'wechat',
      direct_qr_url: '/native-checkout/orders/NC-1/qr',
    })
    mocks.getDirectQR.mockRejectedValue(new Error('provider challenge'))
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,LINKQR')
    mocks.getOrder.mockResolvedValue({
      order_no: 'NC-1',
      status: 'pending',
      pay_amount_cny_fen: 100,
      benefit_amount_cny_fen: 500,
      payment_url: 'https://pay.ldxp.cn/pay/NC-1',
      payment_method: 'wechat',
    })
    const wrapper = mount(NativeCheckoutTrialOffer, {
      global: { stubs: { Teleport: true } },
    })
    await flushPromises()

    await wrapper.find('.trial-offer__action').trigger('click')
    await flushPromises()

    expect(mocks.createOrder).toHaveBeenCalledWith(offer.code)
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
      pay_amount_cny_fen: 100,
      benefit_amount_cny_fen: 500,
      payment_url: 'https://pay.ldxp.cn/pay/NC-ALIPAY',
      payment_method: 'alipay',
      direct_qr_url: '/native-checkout/orders/NC-ALIPAY/qr',
    })
    mocks.getDirectQR.mockRejectedValue(new Error('provider challenge'))
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,ALIPAYLINK')
    mocks.getOrder.mockResolvedValue({
      order_no: 'NC-ALIPAY',
      status: 'pending',
      pay_amount_cny_fen: 100,
      benefit_amount_cny_fen: 500,
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
        order_no: 'NC-DONE', status: 'completed', pay_amount_cny_fen: 100, benefit_amount_cny_fen: 500,
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
})
