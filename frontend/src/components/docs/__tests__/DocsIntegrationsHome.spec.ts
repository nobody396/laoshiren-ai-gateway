import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DocsIntegrationsHome from '../DocsIntegrationsHome.vue'
import homeSource from '../DocsIntegrationsHome.vue?raw'
import { simpleClientGuides } from '@/docs/guides/simpleClientGuides'

const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

describe('DocsIntegrationsHome', () => {
  it('shows only tools whose new guide is complete', () => {
    const wrapper = mount(DocsIntegrationsHome, {
      global: { stubs: { RouterLink: RouterLinkStub } },
    })

    const cards = wrapper.findAll('.client-card')
    expect(cards).toHaveLength(simpleClientGuides.length)
    expect(wrapper.findAll('[data-client-status="manual"]')).toHaveLength(8)
    expect(wrapper.findAll('[data-client-status="paused"]')).toHaveLength(0)
    expect(wrapper.findAll('.status-manual')).toHaveLength(8)
    expect(wrapper.findAll('.status-paused')).toHaveLength(0)
    expect(wrapper.get('[href="/docs/integration-claude-code"]').text()).toContain('手动配置')
    expect(wrapper.get('[href="/docs/integration-codex"]').text()).toContain('手动配置')
    expect(wrapper.get('[href="/docs/integration-grok-build"]').text()).toContain('手动配置')
    expect(wrapper.get('[href="/docs/integration-opencode"]').text()).toContain('手动配置')
    expect(wrapper.get('[href="/docs/integration-antigravity"]').text()).toContain('手动配置')
    expect(wrapper.get('[href="/docs/integration-kimi-code"]').text()).toContain('手动配置')
    expect(wrapper.get('[href="/docs/integration-zcode"]').text()).toContain('手动配置')
    expect(wrapper.get('[href="/docs/integration-workbuddy"]').text()).toContain('手动配置')
    expect(wrapper.text()).not.toContain('Gemini CLI')
    expect(wrapper.text()).not.toContain('Hermes Agent')
    expect(wrapper.text()).not.toContain('Qoder')
    expect(wrapper.text()).not.toContain('MiniMax Code')
    expect(wrapper.text()).not.toContain('DeepSeek Harness')
    expect(wrapper.text()).not.toContain('Visual Studio Code Local Agent')
    expect(wrapper.text()).toContain('其他工具仍保留在代码和能力矩阵源文件中')
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
