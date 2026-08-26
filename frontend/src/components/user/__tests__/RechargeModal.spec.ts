import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import RechargeModal from '../RechargeModal.vue'

const publicSettings = {
  topup_alipay_enabled: true,
  topup_wechat_enabled: false,
  card_shop_enabled: true,
  card_shop_products: [{
    id: 'legacy-card-shop',
    label: '旧卡密商品',
    amount_cny: 50,
    enabled: true,
    url: 'https://shop.example/legacy',
    sort_order: 0,
  }],
}

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: publicSettings,
    fetchPublicSettings: vi.fn().mockResolvedValue(publicSettings),
    showSuccess: vi.fn(),
    showError: vi.fn(),
  }),
  useAuthStore: () => ({ refreshUser: vi.fn() }),
}))

vi.mock('@/api/topup', () => ({
  createTopupOrder: vi.fn(),
  queryTopupOrderStatus: vi.fn(),
}))

vi.mock('@/composables/useManualNewcomerOffer', () => ({
  useManualNewcomerOffer: () => ({
    mode: { value: 'manual' },
    refresh: vi.fn().mockResolvedValue(undefined),
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('RechargeModal payment methods', () => {
  it('never renders the legacy card-shop method even if stale settings still contain it', async () => {
    const wrapper = mount(RechargeModal, {
      props: { modelValue: true },
      global: {
        stubs: {
          Teleport: true,
          NativeCheckoutTrialOffer: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="recharge-method-alipay"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="recharge-method-wechat"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="recharge-method-card_shop"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('旧卡密商品')

    wrapper.unmount()
  })
})
