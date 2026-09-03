import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DocsIntegrationsHome from '../DocsIntegrationsHome.vue'
import { clientMatrix } from '@/generated/clientMatrix'

const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

describe('DocsIntegrationsHome', () => {
  it('lists all 14 matrix clients and reports their real integration state', () => {
    const wrapper = mount(DocsIntegrationsHome, {
      global: { stubs: { RouterLink: RouterLinkStub } },
    })

    const cards = wrapper.findAll('.client-card')
    expect(cards).toHaveLength(14)
    expect(cards.map(card => card.get('.client-card-head > span:not(.client-icon) > b').text())).toEqual(clientMatrix.map(client => client.name))
    expect(wrapper.findAll('[data-client-status="ready"]')).toHaveLength(clientMatrix.filter(client => client.one_click_status === 'ready').length)
    expect(wrapper.findAll('[data-client-status="prototype"]')).toHaveLength(clientMatrix.filter(client => client.one_click_status === 'prototype').length)
    expect(wrapper.findAll('[data-client-status="disabled"]')).toHaveLength(clientMatrix.filter(client => client.one_click_status === 'disabled').length)
    expect(wrapper.text()).toContain('配置基线')
    expect(wrapper.text()).not.toContain('已验证版本')
    expect(wrapper.text()).not.toContain('交互原型')
    expect(wrapper.find('pre').exists()).toBe(false)
  })
})
