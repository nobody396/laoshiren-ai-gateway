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
  it('renders GPT Image 2 token prices in the same table layout as text models', () => {
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

    expect(wrapper.find('.pricing-group__image').exists()).toBe(false)
    expect(wrapper.find('.pricing-group__table').exists()).toBe(true)
    expect(wrapper.findAll('tbody tr').map((row) => row.text())).toEqual([
      'GPT Image 2 · modelPricing.image.textModality¥20.00——¥5.00',
      'GPT Image 2 · modelPricing.image.imageModality¥32.00¥120.00—¥8.00'
    ])
    expect(wrapper.text()).not.toContain('支持 quality、size、output_format 等参数。')
  })

  it('renders fixed image pricing as the first row in the shared model table', () => {
    const wrapper = mount(ModelPricingGroupSection, {
      props: {
        group: {
          group_id: 6,
          name: 'CodeX Pro 20X 分组',
          description: '支持自然语言生图。',
          platform: 'openai',
          rate_multiplier: 0.5,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [
            {
              model: 'gpt-5.6-sol',
              input_price: 2.5,
              output_price: 15,
              cache_write_price: 3.125,
              cache_read_price: 0.25
            }
          ],
          image_generation: {
            mode: 'fixed_per_image',
            price_per_image: 0.3
          }
        }
      }
    })

    expect(wrapper.findAll('tbody tr').map((row) => row.text())).toEqual([
      'GPT Image 2—¥0.3000modelPricing.image.perImageUnit——',
      'gpt-5.6-sol¥2.50¥15.00¥3.13¥0.2500'
    ])
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
              cache_write_price: 0.625,
              cache_read_price: 0.05,
              disabled: true
            }
          ]
        }
      }
    })

    const row = wrapper.get('tbody tr')
    expect(row.classes()).toContain('pricing-group__row--disabled')
    expect(row.findAll('.pricing-group__value')).toHaveLength(5)
    expect(row.get('.pricing-group__disabled-badge').text()).toBe('modelPricing.disabled')
  })
})
