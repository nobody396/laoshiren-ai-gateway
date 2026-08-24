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
}))

const publicSettings = {
  topup_alipay_enabled: true,
  topup_wechat_enabled: true,
  card_shop_enabled: true,
  card_shop_products: [20, 50, 100, 500, 1000].map((amount, index) => ({
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
    state: { value: 'eligible' },
    mode: { value: 'manual' },
    refresh: mocks.refreshNewcomer,
    requestPurchaseURL: vi.fn(),
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
        NativeCheckoutTrialOffer: true,
        RouterLink: { template: '<a><slot /></a>' },
        Icon: true,
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
    mocks.refreshUser.mockResolvedValue({ balance: 48.48 })
    mocks.toDataURL.mockResolvedValue('data:image/png;base64,QR')
  })

  it('shows Alipay scan, WeChat scan, and card shop as the three direct choices', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="topup-method-alipay"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="topup-method-wechat"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="topup-method-card_shop"]').exists()).toBe(true)
    expect(wrapper.find('.topup-pay-grid').exists()).toBe(false)
    expect(wrapper.text()).toContain('¥255')
    expect(wrapper.text()).toContain('¥715')
    expect(wrapper.text()).toContain('¥1525')
    expect(wrapper.text()).not.toContain('topup.monthlyDirectAction')

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

    expect(mocks.createTopupOrder).toHaveBeenCalledWith(2000, 'alipay')
    expect(mocks.refreshUser).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })
})
