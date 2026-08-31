import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import GetSubscriptionView from '../GetSubscriptionView.vue'

const mocks = vi.hoisted(() => ({
  createTopupOrder: vi.fn(),
  queryTopupOrderStatus: vi.fn(),
  refreshUser: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  routerPush: vi.fn(),
  toDataURL: vi.fn(),
  loadPlans: vi.fn(),
  loadOffers: vi.fn(),
  refreshNewcomer: vi.fn(),
  requestNewcomerPurchaseURL: vi.fn(),
  newcomerState: { value: 'available' },
  newcomerMode: { value: 'manual' },
  nativeOffersByCode: { value: {} as Record<string, any> },
  getAffiliateWallet: vi.fn(),
  purchaseBalanceWithAffiliateCommission: vi.fn(),
  createCommissionWalletCheckoutOrder: vi.fn(),
}))

const publicSettings = {
  topup_alipay_enabled: true,
  topup_wechat_enabled: true,
  card_shop_enabled: true,
  card_shop_products: [20, 50, 100, 300, 500, 1000].map((amount, index) => ({
    id: `card-${amount}`,
    label: `¥${amount} 余额卡`,
    amount_cny: amount,
    enabled: true,
    url: `https://shop.example/${amount}`,
    sort_order: index,
  })),
}

const appStore = {
  cachedPublicSettings: publicSettings,
  contactInfo: '',
  fetchPublicSettings: vi.fn().mockResolvedValue(publicSettings),
  showSuccess: mocks.showSuccess,
  showError: mocks.showError,
  showInfo: mocks.showInfo,
}

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAuthStore: () => ({ refreshUser: mocks.refreshUser }),
}))

vi.mock('@/api/topup', () => ({
  createTopupOrder: mocks.createTopupOrder,
  queryTopupOrderStatus: mocks.queryTopupOrderStatus,
}))

vi.mock('@/api/agent', () => ({
  getAffiliateWallet: mocks.getAffiliateWallet,
  purchaseBalanceWithAffiliateCommission: mocks.purchaseBalanceWithAffiliateCommission,
  affiliateIdempotencyKey: (action: string) => `test-${action}`,
}))

vi.mock('@/api/nativeCheckout', () => ({
  createCommissionWalletCheckoutOrder: mocks.createCommissionWalletCheckoutOrder,
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: mocks.routerPush }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('qrcode', () => ({ default: { toDataURL: mocks.toDataURL } }))

vi.mock('@/composables/useMonthlyCreditCardPlans', async () => {
  const { ref } = await import('vue')
  return {
    useMonthlyCreditCardPlans: () => ({
      plans: ref([
        { id: 'plus', name: 'Plus', price: '¥259', directPrice: '¥255', displayMonthlyCreditsText: '3,000', displayWeeklyCreditsText: '0', showWeeklyLimit: false, description: '', accent: 'plus' },
        { id: 'pro', name: 'Pro', price: '¥729', directPrice: '¥715', displayMonthlyCreditsText: '9,000', displayWeeklyCreditsText: '0', showWeeklyLimit: false, description: '', accent: 'pro' },
        { id: 'max', name: 'Max', price: '¥1549', directPrice: '¥1525', displayMonthlyCreditsText: '20,000', displayWeeklyCreditsText: '0', showWeeklyLimit: false, description: '', accent: 'max' },
      ]),
      loadMonthlyCreditCardPlans: mocks.loadPlans,
    }),
  }
})

vi.mock('@/composables/useManualNewcomerOffer', () => ({
  shouldShowManualNewcomerProduct: () => true,
  useManualNewcomerOffer: () => ({
    state: mocks.newcomerState,
    mode: mocks.newcomerMode,
    refresh: mocks.refreshNewcomer,
    requestPurchaseURL: mocks.requestNewcomerPurchaseURL,
  }),
}))

vi.mock('@/composables/useNativeCheckoutOffers', () => ({
  useNativeCheckoutOffers: () => ({
    loadOffers: mocks.loadOffers,
    findNativeOfferByCode: (code: string) => mocks.nativeOffersByCode.value[code],
  }),
}))

function mountView() {
  return mount(GetSubscriptionView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        NativeCheckoutTrialOffer: {
          props: ['offerCode', 'compact', 'actionOnly', 'preferredPayMethod', 'actionLabel'],
          template: '<div data-testid="native-checkout-offer" :data-offer-code="offerCode" :data-compact="compact" :data-action-only="actionOnly" :data-pay-method="preferredPayMethod" :data-action-label="actionLabel" />',
        },
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('GetSubscriptionView payment UX', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    appStore.cachedPublicSettings = { ...publicSettings }
    mocks.loadPlans.mockResolvedValue(undefined)
    mocks.loadOffers.mockResolvedValue(undefined)
    mocks.refreshNewcomer.mockResolvedValue(undefined)
    mocks.requestNewcomerPurchaseURL.mockResolvedValue('https://shop.example/5')
    mocks.newcomerState.value = 'available'
    mocks.newcomerMode.value = 'manual'
    mocks.nativeOffersByCode.value = {
      plus: { code: 'plus', product_kind: 'subscription', provider: 'easypay', pay_amount_cny_fen: 25500 },
      pro: { code: 'pro', product_kind: 'subscription', provider: 'easypay', pay_amount_cny_fen: 71500 },
      max: { code: 'max', product_kind: 'subscription', provider: 'easypay', pay_amount_cny_fen: 152500 },
    }
    mocks.getAffiliateWallet.mockRejectedValue(new Error('not an affiliate'))
    mocks.refreshUser.mockResolvedValue({ balance: 48.48 })
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,QR')
  })

  it('reuses the balance checkout rows for developer plans without the nested legacy offer card', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('[role="tab"]')[1].trigger('click')

    const mainProducts = wrapper.find('.topup-main').findAll('.topup-product')
    expect(mainProducts).toHaveLength(3)
    expect(mainProducts[0].text()).toBe('Plus¥255')
    expect(mainProducts[1].text()).toBe('Pro¥715')
    expect(mainProducts[2].text()).toBe('Max¥1525')

    const summary = wrapper.find('.topup-summary-card')
    expect(summary.find('[data-testid="monthly-method-alipay"]').exists()).toBe(true)
    expect(summary.find('[data-testid="monthly-method-wechat"]').exists()).toBe(true)
    expect(summary.find('[data-testid="monthly-method-card_shop"]').exists()).toBe(false)
    const checkout = summary.get('[data-testid="native-checkout-offer"]')
    expect(checkout.attributes('data-action-only')).toBe('')
    expect(checkout.attributes('data-pay-method')).toBe('alipay')
    expect(checkout.attributes('data-action-label')).toBe('立即支付 ¥255')
    expect(wrapper.text()).not.toContain('支付成功后自动到账')

    wrapper.unmount()
  })

  it('shows only the site QR payment methods inside the right summary card', async () => {
    const wrapper = mountView()
    await flushPromises()

    const summary = wrapper.find('.topup-summary-card')
    expect(summary.find('[data-testid="topup-method-alipay"]').exists()).toBe(true)
    expect(summary.find('[data-testid="topup-method-wechat"]').exists()).toBe(true)
    expect(summary.find('[data-testid="topup-method-card_shop"]').exists()).toBe(false)
    expect(summary.find('[data-testid="topup-method-alipay"] img').attributes('src')).toContain('alipay.svg')
    expect(summary.find('[data-testid="topup-method-wechat"] img').attributes('src')).toContain('wechat.svg')
    expect(wrapper.find('.topup-main [data-testid^="topup-method-"]').exists()).toBe(false)
    expect(wrapper.find('.topup-pay-grid').exists()).toBe(false)
    expect(wrapper.text()).toContain('¥300 余额卡')
    expect(wrapper.text()).not.toContain('topup.monthlyDirectAction')
    expect(wrapper.findAll('.topup-promotion-badge')).toHaveLength(2)
    expect(wrapper.find('.topup-product-desc').exists()).toBe(false)

    const tabs = wrapper.findAll('[role="tab"]')
    await tabs[1].trigger('click')
    expect(wrapper.text()).toContain('¥255')
    expect(wrapper.text()).toContain('¥715')
    expect(wrapper.text()).toContain('¥1525')

    wrapper.unmount()
  })

  it('shows the dedicated commission-wallet icon only when the partner wallet is available', async () => {
    mocks.nativeOffersByCode.value = {
      plus: { code: 'plus', product_kind: 'subscription', provider: 'easypay', pay_amount_cny_fen: 29900 },
      pro: { code: 'pro', product_kind: 'subscription', provider: 'easypay', pay_amount_cny_fen: 59900 },
      max: { code: 'max', product_kind: 'subscription', provider: 'easypay', pay_amount_cny_fen: 99900 },
    }
    mocks.getAffiliateWallet.mockResolvedValue({
      agent_id: 47,
      available_cash_micros: 1_000_000_000,
      processing_withdrawal_micros: 0,
      lifetime_earned_micros: 1_000_000_000,
      withdrawal_minimum_micros: 50_000_000,
      withdrawal_sla_hours: 24,
      conversion_multiplier_millis: 1200,
      wallet_checkout_enabled: true,
      wallet_purchase_rate_bps: 8500,
      conversion_enabled: false,
      payment_profile_verified: true,
      can_withdraw: true,
      cash_asset_symbol: '¥',
      credit_asset_symbol: '⚡',
      display_timezone: 'Asia/Shanghai',
    })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('[role="tab"]')[1].trigger('click')
    await wrapper.findAll('.topup-product--monthly')[1].trigger('click')

    const walletMethod = wrapper.get('[data-testid="monthly-method-commission-wallet"]')
    expect(walletMethod.find('img').attributes('src')).toContain('commission-wallet')
    await walletMethod.trigger('click')
    expect(wrapper.text()).toContain('佣金钱包价¥509.15')
    expect(wrapper.text()).toContain('支付后剩余¥490.85')

    wrapper.unmount()
  })

  it('multiplies a balance-card quantity and sends the locked SKU selection', async () => {
    mocks.createTopupOrder.mockResolvedValue({
      order_no: 'TP-QTY-1',
      amount_cny_fen: 6000,
      bonus_amount_cny_fen: 0,
      credited_amount_cny_fen: 6000,
      pay_type: 'alipay',
      qr_code_url: 'alipays://platformapi/startapp?appId=1',
    })
    mocks.queryTopupOrderStatus.mockResolvedValue({ order_no: 'TP-QTY-1', status: 'pending' })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[aria-label="增加数量"]').trigger('click')
    await wrapper.find('[aria-label="增加数量"]').trigger('click')
    expect(wrapper.get('[data-testid="topup-quantity"]').text()).toBe('3')
    expect(wrapper.find('.topup-summary-amount').text()).toBe('¥60')
    expect(wrapper.text()).toContain('立即支付 ¥60')

    await wrapper.find('[data-testid="topup-submit"]').trigger('click')
    await flushPromises()

    expect(mocks.createTopupOrder).toHaveBeenCalledWith(6000, 'alipay', {
      productAmountCnyFen: 2000,
      quantity: 3,
    })

    wrapper.unmount()
  })

  it('keeps the limited-time bonus for every promotional card in the quantity', async () => {
    const wrapper = mountView()
    await flushPromises()

    const promotional500 = wrapper.findAll('.topup-product').find((item) => item.text().includes('实付 ¥500 到账 ¥550'))
    expect(promotional500).toBeDefined()
    await promotional500!.trigger('click')
    await wrapper.find('[aria-label="增加数量"]').trigger('click')
    await wrapper.find('[aria-label="增加数量"]').trigger('click')

    expect(wrapper.find('.topup-summary-amount').text()).toBe('¥1500')
    expect(wrapper.find('.topup-credit-amount').text()).toBe('¥1650.00')

    wrapper.unmount()
  })

  it('keeps the native newcomer checkout without an external backup method', async () => {
    appStore.cachedPublicSettings = {
      ...publicSettings,
      card_shop_products: [
        {
          id: 'newcomer-5-to-10',
          label: '新人特惠 · 10 元余额包',
          amount_cny: 5,
          enabled: true,
          url: 'https://shop.example/5',
          sort_order: -1,
        },
        ...publicSettings.card_shop_products,
      ],
    }
    mocks.newcomerMode.value = 'native'
    const wrapper = mountView()
    await flushPromises()

    const summary = wrapper.find('.topup-summary-card')
    expect(summary.find('[data-testid="native-checkout-offer"]').exists()).toBe(true)
    expect(summary.find('[data-testid="native-checkout-offer"]').attributes('data-compact')).toBe('')
    expect(summary.find('.topup-backup-action').exists()).toBe(false)
    expect(summary.find('[data-testid="topup-method-card_shop"]').exists()).toBe(false)
    expect(wrapper.find('.topup-main [data-testid="native-checkout-offer"]').exists()).toBe(false)

    wrapper.unmount()
  })

  it('refreshes the authenticated user after a completed QR payment', async () => {
    mocks.createTopupOrder.mockResolvedValue({
      order_no: 'TP-TEST-1',
      amount_cny_fen: 2000,
      credited_amount_cny_fen: 2000,
      pay_type: 'alipay',
      qr_code_url: 'alipays://platformapi/startapp?appId=1',
    })
    mocks.queryTopupOrderStatus.mockResolvedValue({
      order_no: 'TP-TEST-1',
      status: 'completed',
      amount_cny_fen: 2000,
      credited_amount_cny_fen: 2000,
      pay_type: 'alipay',
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.find('[data-testid="topup-method-alipay"]').trigger('click')
    await wrapper.find('[data-testid="topup-submit"]').trigger('click')
    await flushPromises()

    expect(mocks.createTopupOrder).toHaveBeenCalledWith(2000, 'alipay', {
      productAmountCnyFen: 2000,
      quantity: 1,
    })
    expect(mocks.refreshUser).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })
})
