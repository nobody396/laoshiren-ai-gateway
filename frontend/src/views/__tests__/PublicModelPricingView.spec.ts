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
    // 默认「全部」：按量组和月卡组全部展示；月卡组同时出现在厂商分块和 Builder Pass 分块
    expect(visibleGroupNames(wrapper)).toEqual(['GPT 按量组', 'GPT Lite 月卡组', 'Claude 按量组', 'GPT Lite 月卡组'])
    expect(wrapper.find('.model-pricing-tabs__item--active')?.text()).toBe('modelPricing.block.all')
    // tab 栏末尾展示模型总数
    expect(wrapper.find('.model-pricing-tabs__count').exists()).toBe(true)
  })

  it('each provider tab carries its brand icon and Builder Pass carries the laoshirenai brand', async () => {
    const wrapper = mountView()
    await flushPromises()

    const tabs = wrapper.findAll('.model-pricing-tabs__item')
    expect(tabs[0].find('.icon-stub').attributes('data-name')).toBe('grid')
    expect(tabs[1].find('.model-icon-stub').attributes('data-model')).toBe('gpt')
    expect(tabs[2].find('.model-icon-stub').attributes('data-model')).toBe('claude')
    expect(tabs[3].find('.model-pricing-tabs__brand').exists()).toBe(true)
  })

  it('filters to a single provider block when its tab is selected', async () => {
    const wrapper = mountView()
    await flushPromises()

    await clickTab(wrapper, 'modelPricing.block.claude')
    expect(visibleGroupNames(wrapper)).toEqual(['Claude 按量组'])

    // 厂商 tab 同时包含该厂商的月卡组
    await clickTab(wrapper, 'modelPricing.block.gpt')
    expect(visibleGroupNames(wrapper)).toEqual(['GPT 按量组', 'GPT Lite 月卡组'])
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
    expect(visibleGroupNames(wrapper)).toEqual(['GPT 按量组', 'GPT Lite 月卡组'])

    await clickTab(wrapper, 'modelPricing.block.all')
    expect(visibleGroupNames(wrapper)).toEqual(['GPT 按量组', 'GPT Lite 月卡组', 'Claude 按量组', 'GPT Lite 月卡组'])
  })

  it('orders public groups before exclusive ones and text groups before image-only ones', async () => {
    getPublicModelPricingMock.mockResolvedValue({
      updated_at: '2026-08-18T00:00:00Z',
      currency: 'CNY',
      unit: 'per_1m_tokens',
      groups: [
        { group_id: 7, name: 'GPT Pro 月卡组', platform: 'openai', rate_multiplier: 0.5, is_exclusive: true, subscription_type: 'credit', models: [price('gpt-5.4')] },
        { group_id: 51, name: 'GPT Image 2 生图分组', platform: 'openai', rate_multiplier: 4, is_exclusive: false, subscription_type: 'standard', models: [], image_generation: { mode: 'fixed_per_image' as const, price_per_image: 0.3 } },
        { group_id: 1, name: 'GPT 按量组', platform: 'openai', rate_multiplier: 0.5, is_exclusive: false, subscription_type: 'standard', models: [price('gpt-5.4')] },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    await clickTab(wrapper, 'modelPricing.block.gpt')
    expect(visibleGroupNames(wrapper)).toEqual(['GPT 按量组', 'GPT Image 2 生图分组', 'GPT Pro 月卡组'])
  })

  it('shows the long-context rule once under the block title with an official link', async () => {
    getPublicModelPricingMock.mockResolvedValue({
      updated_at: '2026-08-18T00:00:00Z',
      currency: 'CNY',
      unit: 'per_1m_tokens',
      groups: [
        {
          group_id: 52, name: 'GPT CYBER 分组（特价！）', platform: 'openai', rate_multiplier: 2, is_exclusive: false, subscription_type: 'standard',
          models: [{ ...price('gpt-daybreak-blue-latest'), long_context: { input_threshold: 272000, input_multiplier: 2, output_multiplier: 1.5 } }],
        },
      ],
    })
    const wrapper = mountView()
    await flushPromises()

    const note = wrapper.get('.model-pricing-block__note')
    expect(note.text()).toContain('modelPricing.longContextRule')
    expect(note.get('a').attributes('href')).toBe('https://openai.com/api/pricing/')
  })
})
