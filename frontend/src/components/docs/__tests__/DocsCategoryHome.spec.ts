import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DocsCategoryHome from '@/components/docs/DocsCategoryHome.vue'
import { docsConfig } from '@/docs/config'
import { clientMatrix } from '@/generated/clientMatrix'

const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

describe('DocsCategoryHome', () => {
  it('shows only the documents from the selected section', () => {
    const clients = docsConfig.find(category => category.key === 'integrations')!
    const wrapper = mount(DocsCategoryHome, {
      props: { category: clients },
      global: { stubs: { RouterLink: RouterLinkStub } },
    })

    expect(wrapper.get('h1').text()).toBe('工具集成')
    expect(wrapper.findAll('.docs-category-list > a')).toHaveLength(clients.items.length)
    expect(wrapper.text()).toContain('Codex')
    expect(wrapper.text()).toContain('Claude Code')
    expect(wrapper.text()).toContain('Kimi Code')
    expect(wrapper.text()).toContain('OpenCode')
    expect(wrapper.text()).toContain('ZCode')
    expect(wrapper.text()).not.toContain('API 概览')
    expect(wrapper.text()).not.toContain('Images')
    expect(wrapper.findAll('.docs-category-list > a')).toHaveLength(14)
    for (const client of clientMatrix) expect(wrapper.text()).toContain(client.name)
  })
})
