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
        { id: 'plus', name: 'Plus', price: '¥259', directPrice: '¥255', displayMonthlyCreditsText: '3,000', displayWeeklyCreditsText: '0', showWeeklyLimit: false, description: '', accent: 'plus', cardShopUrl: 'https://shop.example/plus' },
        { id: 'pro', name: 'Pro', price: '¥729', directPrice: '¥715', displayMonthlyCreditsText: '9,000', displayWeeklyCreditsText: '0', showWeeklyLimit: false, description: '', accent: 'pro', cardShopUrl: 'https://shop.example/pro' },
        { id: 'max', name: 'Max', price: '¥1549', directPrice: '¥1525', displayMonthlyCreditsText: '20,000', displayWeeklyCreditsText: '0', showWeeklyLimit: false, description: '', accent: 'max', cardShopUrl: 'https://shop.example/max' },
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
    findNativeOfferByCode: () => undefined,
  }),
}))

function mountView() {
  return mount(GetSubscriptionView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        NativeCheckoutTrialOffer: {
          props: ['offerCode', 'compact'],
          template: '<div data-testid="native-checkout-offer" :data-offer-code="offerCode" :data-compact="compact" />',
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
    mocks.refreshUser.mockResolvedValue({ balance: 48.48 })
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,QR')
  })

  it('shows Alipay, WeChat, and backup payment methods inside the right summary card', async () => {
    const wrapper = mountView()
    await flushPromises()

    const summary = wrapper.find('.topup-summary-card')
    expect(summary.find('[data-testid="topup-method-alipay"]').exists()).toBe(true)
    expect(summary.find('[data-testid="topup-method-wechat"]').exists()).toBe(true)
    expect(summary.find('[data-testid="topup-method-card_shop"]').exists()).toBe(true)
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

  it('keeps the native newcomer checkout and its backup method in the right summary card', async () => {
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
    expect(summary.find('.topup-backup-action').text()).toContain('topup.cardShopAction')
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
