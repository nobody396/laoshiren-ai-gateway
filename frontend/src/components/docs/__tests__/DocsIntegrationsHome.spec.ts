import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DocsIntegrationsHome from '../DocsIntegrationsHome.vue'
import homeSource from '../DocsIntegrationsHome.vue?raw'
import { clientMatrix } from '@/generated/clientMatrix'

const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

describe('DocsIntegrationsHome', () => {
  it('keeps all tools visible but marks every old guide as being rewritten', () => {
    const wrapper = mount(DocsIntegrationsHome, {
      global: { stubs: { RouterLink: RouterLinkStub } },
    })

    const cards = wrapper.findAll('.client-card')
    expect(cards).toHaveLength(14)
    expect(cards.map(card => card.get('.client-card-head > span:not(.client-icon) > b').text())).toEqual(clientMatrix.map(client => client.name))
    expect(wrapper.findAll('[data-client-status="paused"]')).toHaveLength(clientMatrix.length)
    expect(wrapper.findAll('.status-paused')).toHaveLength(clientMatrix.length)
    expect(wrapper.text()).toContain('旧版内容仍保留在代码中')
    expect(wrapper.find('.status-legend').exists()).toBe(false)
    expect(wrapper.find('.flow').exists()).toBe(false)
    expect(wrapper.find('.protocols').exists()).toBe(false)
  })

  it('keeps the legacy overview implementation in source', () => {
    expect(homeSource).toContain('legacyIntegrationOverviewVisible = false')
    expect(homeSource).toContain('CONFIGURATION FLOW')
    expect(homeSource).toContain('一键导入可用')
    expect(homeSource).toContain('协议支持只是候选条件')
  })
})
