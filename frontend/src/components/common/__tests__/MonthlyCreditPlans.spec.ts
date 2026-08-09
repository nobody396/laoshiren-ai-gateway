import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import MonthlyCreditPlans from '../MonthlyCreditPlans.vue'

describe('MonthlyCreditPlans monthly allowance copy', () => {
  it('keeps every monthly limit compact and explains the pay-as-you-go equivalent', () => {
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
    expect(wrapper.findAll('.monthly-credit-card__credits-label').map((label) => label.text())).toEqual([
      '月限制',
      '月限制',
      '月限制'
    ])
    expect(wrapper.findAll('.monthly-credit-card__credits strong').map((amount) => amount.text())).toEqual([
      '⚡300',
      '⚡900',
      '⚡2,000'
    ])
    expect(text).toContain('相当于 ¥300 API 按量付费额度')
    expect(text).toContain('相当于 ¥900 API 按量付费额度')
    expect(text).toContain('相当于 ¥2000 API 按量付费额度')
    expect(text).not.toContain('能量 / 月')
    expect(wrapper.findAll('.monthly-credit-card__direct-help').map((help) => help.text())).toEqual([
      '直售请登录后联系客服',
      '直售请登录后联系客服',
      '直售请登录后联系客服'
    ])
    expect(text).toContain('GPT 月卡、Claude 月卡和 Grok 月卡')
    expect(wrapper.get('.monthly-credit-plans__note a').attributes('href')).toBe('/models')
  })

  it('turns every landing-page plan into a clear purchase action', () => {
    const wrapper = mount(MonthlyCreditPlans, {
      props: {
        variant: 'home',
        showAction: false
      },
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a :href="to"><slot /></a>'
          }
        }
      }
    })

    const purchaseLinks = wrapper.findAll('.monthly-credit-card__action')
    expect(purchaseLinks).toHaveLength(3)
    expect(purchaseLinks.map((link) => link.text())).toEqual([
      '查看并购买→',
      '查看并购买→',
      '查看并购买→'
    ])
    expect(purchaseLinks.every((link) => link.attributes('target') === '_blank')).toBe(true)
    expect(purchaseLinks.every((link) => link.attributes('rel') === 'noopener noreferrer')).toBe(true)
  })
})
