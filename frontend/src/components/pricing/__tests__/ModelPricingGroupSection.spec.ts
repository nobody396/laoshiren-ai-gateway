import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import ModelPricingGroupSection from '../ModelPricingGroupSection.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

describe('ModelPricingGroupSection', () => {
  it('renders the five GPT Image 2 modal prices without a generic model row', () => {
    const wrapper = mount(ModelPricingGroupSection, {
      props: {
        group: {
          group_id: 51,
          name: 'GPT Image 2 生图分组',
          description: '支持 quality、size、output_format 等参数。',
          platform: 'openai',
          rate_multiplier: 4,
          is_exclusive: false,
          subscription_type: 'standard',
          // Reproduce the stale production response that exposed the boundary:
          // image-only groups used to serialize an empty Go slice as null.
          models: null,
          image_generation: {
            mode: 'token',
            text_input_price: 20,
            text_cached_input_price: 5,
            image_input_price: 32,
            image_cached_input_price: 8,
            image_output_price: 120
          }
        }
      }
    })

    expect(wrapper.find('.pricing-group__image').exists()).toBe(true)
    expect(wrapper.findAll('.pricing-group__image-price').map((item) => item.text())).toEqual([
      'modelPricing.image.textInput¥20.00',
      'modelPricing.image.textCachedInput¥5.00',
      'modelPricing.image.imageInput¥32.00',
      'modelPricing.image.imageCachedInput¥8.00',
      'modelPricing.image.imageOutput¥120.00'
    ])
    expect(wrapper.text()).toContain('支持 quality、size、output_format 等参数。')
    expect(wrapper.find('.pricing-group__table').exists()).toBe(false)
  })

  it('strikes through and labels disabled GPT-5.6 Luna', () => {
    const wrapper = mount(ModelPricingGroupSection, {
      props: {
        group: {
          group_id: 6,
          name: 'CodeX Pro 20X 分组',
          platform: 'openai',
          rate_multiplier: 0.5,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [
            {
              model: 'gpt-5.6-luna',
              input_price: 0.5,
              output_price: 3,
              cache_read_price: 0.05,
              disabled: true
            }
          ]
        }
      }
    })

    const row = wrapper.get('tbody tr')
    expect(row.classes()).toContain('pricing-group__row--disabled')
    expect(row.findAll('.pricing-group__value')).toHaveLength(4)
    expect(row.get('.pricing-group__disabled-badge').text()).toBe('modelPricing.disabled')
  })
})
