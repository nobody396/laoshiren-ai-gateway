import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const getPublicModelPricing = vi.fn()

vi.mock('@/api/publicPricing', () => ({
  getPublicModelPricing: (...args: unknown[]) => getPublicModelPricing(...args),
}))

import DocsModelCatalog from '../DocsModelCatalog.vue'

describe('DocsModelCatalog', () => {
  it('renders one logical model with multiple availability plans', async () => {
    getPublicModelPricing.mockResolvedValue({
      updated_at: '2026-08-28T00:00:00Z',
      currency: 'CNY',
      unit: 'per_million_tokens',
      groups: [
        {
          group_id: 10,
          name: 'GPT Plus 月卡组',
          platform: 'openai',
          rate_multiplier: 0.5,
          is_exclusive: true,
          subscription_type: 'credit',
          models: [{ model: 'gpt-5.6-sol', input_price: 1, output_price: 2, cache_read_price: 0.1 }],
        },
        {
          group_id: 11,
          name: 'GPT 混池分组',
          platform: 'openai',
          rate_multiplier: 0.35,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [
            { model: 'gpt-5.6-sol', input_price: 1, output_price: 2, cache_read_price: 0.1 },
            { model: 'gpt-hidden', input_price: 1, output_price: 2, cache_read_price: 0.1, disabled: true },
          ],
        },
      ],
    })

    const wrapper = mount(DocsModelCatalog, {
      global: {
        stubs: {
          ModelIcon: { props: ['model'], template: '<span class="model-icon" :data-model="model" />' },
        },
      },
    })
    await flushPromises()

    const articles = wrapper.findAll('article')
    expect(articles).toHaveLength(1)
    expect(articles[0].get('h2').text()).toBe('gpt-5.6-sol')
    expect(articles[0].text()).toContain('推荐 Codex')
    expect(articles[0].text()).toContain('2 个可用分组')
    expect(articles[0].text()).toContain('GPT Plus 月卡组')
    expect(articles[0].text()).toContain('GPT 混池分组')
    expect(wrapper.text()).not.toContain('gpt-hidden')

    const monthlyRows = wrapper.findAll('section[aria-labelledby="monthly-plan-heading"] tbody tr')
    expect(monthlyRows.map((row) => row.findAll('td').map((cell) => cell.text()))).toEqual([
      ['Plus', '3,000 AI credits', '¥259', '¥255'],
      ['Pro', '9,000 AI credits', '¥729', '¥715'],
      ['Max', '20,000 AI credits', '¥1549', '¥1525'],
    ])
    expect(wrapper.text()).toContain('GPT、Claude 和 Grok 月卡分组')
  })

  it('keeps the dedicated image product separate from text models', async () => {
    getPublicModelPricing.mockResolvedValue({
      updated_at: '2026-08-28T00:00:00Z',
      currency: 'CNY',
      unit: 'per_million_tokens',
      groups: [
        {
          group_id: 51,
          name: 'GPT Image 2 生图分组',
          platform: 'openai',
          rate_multiplier: 4,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [],
          image_generation: {
            mode: 'token',
            text_input_price: 20,
            image_input_price: 32,
            image_output_price: 120,
          },
        },
      ],
    })

    const wrapper = mount(DocsModelCatalog, {
      global: {
        stubs: {
          ModelIcon: { props: ['model'], template: '<span class="model-icon" :data-model="model" />' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.findAll('article')).toHaveLength(1)
    expect(wrapper.get('h2').text()).toBe('gpt-image-2')
    expect(wrapper.text()).toContain('Images API')
    expect(wrapper.text()).toContain('GPT Image 2 生图分组')
    expect(wrapper.text()).toContain('文本输入 ¥20/M · 图片输入 ¥32/M · 图片输出 ¥120/M')
  })
})
