import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import type { PublicModelPricingCatalog } from '@/api/publicPricing'

const getPublicModelPricingMock = vi.fn()

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/api/publicPricing', () => ({
  getPublicModelPricing: () => getPublicModelPricingMock(),
}))

import PublicModelPricingView from '@/views/PublicModelPricingView.vue'

function price(model: string) {
  return { model, input_price: 1, output_price: 2, cache_read_price: 0.1 }
}

const catalog: PublicModelPricingCatalog = {
  updated_at: '2026-08-18T00:00:00Z',
  currency: 'CNY',
  unit: 'per_1m_tokens',
  groups: [
    { group_id: 1, name: 'GPT 按量组', platform: 'openai', rate_multiplier: 0.5, is_exclusive: false, subscription_type: 'standard', models: [price('gpt-5.4')] },
    { group_id: 2, name: 'Claude 按量组', platform: 'anthropic', rate_multiplier: 1, is_exclusive: false, subscription_type: 'standard', models: [price('claude-sonnet-5')] },
    { group_id: 7, name: 'GPT Lite 月卡组', platform: 'openai', rate_multiplier: 0.3774, is_exclusive: false, subscription_type: 'credit', models: [price('gpt-5.4')] },
  ],
}

function mountView() {
  return mount(PublicModelPricingView, {
    global: {
      stubs: {
        ModelIcon: defineComponent({ props: ['model'], template: '<span class="model-icon-stub" :data-model="model" />' }),
        Icon: defineComponent({ props: ['name'], template: '<span class="icon-stub" :data-name="name" />' }),
        PricingBillingExample: defineComponent({ template: '<div />' }),
        ModelPricingGroupSection: defineComponent({ props: ['group'], template: '<div class="group-stub">{{ group.name }}</div>' }),
        RouterLink: defineComponent({ template: '<a><slot /></a>' }),
      },
    },
  })
}

function tabTexts(wrapper: ReturnType<typeof mountView>) {
  return wrapper.findAll('.model-pricing-tabs__item').map((b) => b.text())
}

function visibleGroupNames(wrapper: ReturnType<typeof mountView>) {
  return wrapper.findAll('.group-stub').map((g) => g.text())
}

async function clickTab(wrapper: ReturnType<typeof mountView>, labelKey: string) {
  const tab = wrapper.findAll('.model-pricing-tabs__item').find((b) => b.text() === labelKey)
  expect(tab, `tab ${labelKey} should exist`).toBeTruthy()
  await tab!.trigger('click')
  await flushPromises()
}

describe('PublicModelPricingView', () => {
  beforeEach(() => {
    getPublicModelPricingMock.mockReset()
    getPublicModelPricingMock.mockResolvedValue(catalog)
  })

  it('shows 全部 plus provider tabs and the Builder Pass tab, defaulting to all groups', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(tabTexts(wrapper)).toEqual([
      'modelPricing.block.all',
      'modelPricing.block.gpt',
      'modelPricing.block.claude',
      'modelPricing.block.builderPass',
    ])
    // 默认「全部」：按量组和月卡组全部展示
    expect(visibleGroupNames(wrapper)).toEqual(['GPT 按量组', 'Claude 按量组', 'GPT Lite 月卡组'])
    expect(wrapper.find('.model-pricing-tabs__item--active')?.text()).toBe('modelPricing.block.all')
  })

  it('each provider tab carries its brand icon', async () => {
    const wrapper = mountView()
    await flushPromises()

    const tabs = wrapper.findAll('.model-pricing-tabs__item')
    expect(tabs[0].find('.icon-stub').attributes('data-name')).toBe('grid')
    expect(tabs[1].find('.model-icon-stub').attributes('data-model')).toBe('gpt')
    expect(tabs[2].find('.model-icon-stub').attributes('data-model')).toBe('claude')
    expect(tabs[3].find('.icon-stub').attributes('data-name')).toBe('creditCard')
  })

  it('filters to a single provider block when its tab is selected', async () => {
    const wrapper = mountView()
    await flushPromises()

    await clickTab(wrapper, 'modelPricing.block.claude')
    expect(visibleGroupNames(wrapper)).toEqual(['Claude 按量组'])

    await clickTab(wrapper, 'modelPricing.block.gpt')
    expect(visibleGroupNames(wrapper)).toEqual(['GPT 按量组'])
  })

  it('Builder Pass tab shows only monthly-card (credit) groups', async () => {
    const wrapper = mountView()
    await flushPromises()

    await clickTab(wrapper, 'modelPricing.block.builderPass')
    expect(visibleGroupNames(wrapper)).toEqual(['GPT Lite 月卡组'])
  })

  it('switching back to 全部 restores all blocks', async () => {
    const wrapper = mountView()
    await flushPromises()

    await clickTab(wrapper, 'modelPricing.block.gpt')
    expect(visibleGroupNames(wrapper)).toEqual(['GPT 按量组'])

    await clickTab(wrapper, 'modelPricing.block.all')
    expect(visibleGroupNames(wrapper)).toEqual(['GPT 按量组', 'Claude 按量组', 'GPT Lite 月卡组'])
  })
})
