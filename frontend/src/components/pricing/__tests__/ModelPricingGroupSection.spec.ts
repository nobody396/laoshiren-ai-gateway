import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
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
      'GPT Image 2modelPricing.image.textModality¥20.00——¥5.00',
      'modelPricing.image.imageModality¥32.00¥120.00—¥8.00'
    ])
    expect(wrapper.text()).not.toContain('支持 quality、size、output_format 等参数。')
  })

  it('renders fixed image pricing as the last row after text models', () => {
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
      'gpt-5.6-solmodelPricing.table.standard¥2.50¥15.00¥3.13¥0.2500',
      'GPT Image 2modelPricing.table.perImage—¥0.3000modelPricing.image.perImageUnit——'
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

  it('does not repeat the long-context surcharge under each model row', () => {
    // 长上下文规则已上移到分块标题下统一展示一次（PublicModelPricingView），
    // 行内不再重复渲染。
    const wrapper = mount(ModelPricingGroupSection, {
      props: {
        group: {
          group_id: 52,
          name: 'GPT CYBER 分组（特价！）',
          platform: 'openai',
          rate_multiplier: 2,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [
            {
              model: 'gpt-daybreak-blue-latest',
              input_price: 10,
              output_price: 60,
              cache_write_price: 12.5,
              cache_read_price: 0.5,
              long_context: {
                input_threshold: 272000,
                input_multiplier: 2,
                output_multiplier: 1.5
              }
            }
          ]
        }
      }
    })

    expect(wrapper.find('.pricing-group__long-context').exists()).toBe(false)
  })

  it('shows brand icon and official protocol name in the group header', () => {
    const wrapper = mount(ModelPricingGroupSection, {
      props: {
        group: {
          group_id: 52,
          name: 'GLM 分组',
          platform: 'openai',
          rate_multiplier: 0.7,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [
            { model: 'glm-5.2', input_price: 1, output_price: 2, cache_read_price: 0.1 }
          ]
        }
      },
      global: {
        stubs: {
          ModelIcon: defineComponent({ props: ['model'], template: '<span class="model-icon-stub" :data-model="model" />' })
        }
      }
    })

    expect(wrapper.get('.model-icon-stub').attributes('data-model')).toBe('glm-5.2')
    expect(wrapper.get('.pricing-group__title .pricing-group__badge').text()).toBe('Responses / Chat Completions')
  })

  it('labels image-only groups with the Images API protocol', () => {
    const wrapper = mount(ModelPricingGroupSection, {
      props: {
        group: {
          group_id: 51,
          name: 'GPT Image 2 生图分组',
          platform: 'openai',
          rate_multiplier: 4,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [],
          image_generation: { mode: 'fixed_per_image', price_per_image: 0.3 }
        }
      },
      global: {
        stubs: {
          ModelIcon: defineComponent({ props: ['model'], template: '<span class="model-icon-stub" :data-model="model" />' })
        }
      }
    })

    expect(wrapper.get('.model-icon-stub').attributes('data-model')).toBe('gpt-image-2')
    expect(wrapper.get('.pricing-group__title .pricing-group__badge').text()).toBe('Images API')
  })

  it('renders one model cell with vertically aligned context-tier price rows', () => {
    const wrapper = mount(ModelPricingGroupSection, {
      props: {
        group: {
          group_id: 6,
          name: '按量 GPT',
          platform: 'openai',
          rate_multiplier: 2,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [{
            model: 'gpt-5.6-sol', input_price: 10, output_price: 60,
            cache_write_price: null, cache_read_price: 1,
            context_intervals: [
              {
                min_tokens: 0, max_tokens: 272000,
                input_price: 8, output_price: 40, cache_write_price: null, cache_read_price: 0.8
              },
              {
                min_tokens: 272000, max_tokens: 1000000,
                input_price: 16, output_price: 60, cache_write_price: null, cache_read_price: 1.6
              }
            ]
          }]
        }
      }
    })

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('gpt-5.6-sol≤272,000¥8.00¥40.00—¥0.8000')
    expect(rows[1].text()).toContain('>272,000–1M¥16.00¥60.00—¥1.60')
    expect(rows[1].text()).not.toContain('gpt-5.6-sol')
  })

  it('renders one model cell with peak and valley prices aligned under the normal price columns', () => {
    const wrapper = mount(ModelPricingGroupSection, {
      props: {
        group: {
          group_id: 61,
          name: 'DeepSeek（阿里云）',
          platform: 'openai',
          rate_multiplier: 0.95,
          is_exclusive: false,
          subscription_type: 'standard',
          models: [{
            model: 'deepseek-v4-pro-0813', input_price: 8.55, output_price: 25.65,
            cache_write_price: null, cache_read_price: 0.855,
            time_pricing: {
              timezone: 'Asia/Shanghai', weekdays_only: false,
              periods: [
                { start_time: '00:00:00', end_time: '08:00:00', multiplier: 0.5 },
                { start_time: '22:00:00', end_time: '00:00:00', multiplier: 0.5 }
              ]
            }
          }]
        }
      }
    })

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('deepseek-v4-pro-0813modelPricing.timePricing.peak · 08:00–22:00¥8.55¥25.65—¥0.8550')
    expect(rows[1].text()).toContain('modelPricing.timePricing.valley · 22:00–modelPricing.timePricing.nextDay08:00¥4.28¥12.82—¥0.4275')
    expect(rows[1].text()).not.toContain('deepseek-v4-pro-0813')
    expect(wrapper.text()).not.toContain('modelPricing.table.input ¥')
  })
})
