import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import MonthlyCreditPlans from '../MonthlyCreditPlans.vue'

describe('MonthlyCreditPlans AI energy copy', () => {
  it('explains every plan in user-facing energy and pay-as-you-go equivalents', () => {
    const wrapper = mount(MonthlyCreditPlans, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="to"><slot /></a>'
          }
        }
      }
    })

    const text = wrapper.text().replace(/\s+/g, ' ')
    expect(text).toContain('⚡300 能量 / 月')
    expect(text).toContain('相当于 ¥300 API 按量付费额度')
    expect(text).toContain('⚡900 能量 / 月')
    expect(text).toContain('相当于 ¥900 API 按量付费额度')
    expect(text).toContain('⚡2,000 能量 / 月')
    expect(text).toContain('相当于 ¥2000 API 按量付费额度')
    expect(text).toContain('GPT 月卡、Cloud 月卡和 Grok 月卡')
    expect(wrapper.get('a').attributes('href')).toBe('/models')
  })
})
