import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createPinia } from 'pinia'
import DocsHome from '@/components/docs/DocsHome.vue'

const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

const mountDocsHome = () => mount(DocsHome, {
  global: {
    plugins: [createPinia()],
    stubs: { RouterLink: RouterLinkStub },
  },
})

describe('DocsHome', () => {
  it('renders a graphical task portal with all primary documentation sections', () => {
    const wrapper = mountDocsHome()

    expect(wrapper.get('h1').text()).toBe('老实人AI 开发文档')
    expect(wrapper.findAll('.docs-home-protocol-tabs button')).toHaveLength(5)
    expect(wrapper.findAll('.docs-home-card')).toHaveLength(4)
    expect(wrapper.find('.docs-home-quick-links').exists()).toBe(false)
    expect(wrapper.findAll('.docs-home-card').map(card => card.attributes('href'))).toEqual([
      '/docs/quickstart',
      '/docs/category/api',
      '/docs/category/integrations',
      '/docs/category/models',
    ])
    expect(wrapper.text()).toContain('快速开始')
    expect(wrapper.text()).toContain('API 参考')
    expect(wrapper.text()).toContain('工具集成')
    expect(wrapper.text()).toContain('模型目录')
  })

  it('searches the curated documentation catalog', async () => {
    const wrapper = mountDocsHome()

    await wrapper.get('input[type="search"]').setValue('Codex')

    expect(wrapper.text()).toContain('搜索结果')
    expect(wrapper.findAll('.docs-home-result-list > a').length).toBeGreaterThan(0)
    expect(wrapper.text()).toContain('Codex')
    expect(wrapper.find('.docs-home-protocols').exists()).toBe(false)
  })

  it('switches between protocol examples', async () => {
    const wrapper = mountDocsHome()

    const messagesTab = wrapper.findAll('.docs-home-protocol-tabs button').find(button => button.text() === 'Messages')
    expect(messagesTab).toBeTruthy()
    await messagesTab!.trigger('click')

    expect(wrapper.get('.docs-home-code-toolbar').text()).toContain('POST /v1/messages')
    expect(wrapper.get('.docs-home-code-panel code').text()).toContain('anthropic-version')
  })
})
