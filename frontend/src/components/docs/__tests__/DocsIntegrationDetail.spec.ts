import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DocsIntegrationDetail from '../DocsIntegrationDetail.vue'
import detailSource from '../DocsIntegrationDetail.vue?raw'
import { clientMatrixBySlug } from '@/generated/clientMatrix'

const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

const mountDetail = (slug: string) => mount(DocsIntegrationDetail, {
  props: { client: clientMatrixBySlug[slug] },
  global: { stubs: { RouterLink: RouterLinkStub } },
})

describe('DocsIntegrationDetail', () => {
  it('hides the legacy integration content while the simple guides are rewritten', () => {
    const wrapper = mountDetail('integration-codex')

    expect(wrapper.attributes('data-client-status')).toBe('paused')
    expect(wrapper.text()).toContain('教程整理中')
    expect(wrapper.text()).toContain('旧版内容已暂时隐藏')
    expect(wrapper.text()).toContain('配置位置、Base URL、API Key、模型 ID 和验证方法')
    expect(wrapper.find('.manual-config').exists()).toBe(false)
    expect(wrapper.find('.steps').exists()).toBe(false)
    expect(wrapper.find('.files').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('复制真实命令')
    expect(wrapper.text()).not.toContain('读取当前分组模型')
  })

  it('keeps the legacy implementation in source for later revision', () => {
    expect(detailSource).toContain('legacyIntegrationDetailsVisible = false')
    expect(detailSource).toContain('<ClientManualConfig')
    expect(detailSource).toContain('<ClaudeCodeManualConfig')
    expect(detailSource).toContain('自动配置')
    expect(detailSource).toContain('配置会修改哪些文件')
  })
})
