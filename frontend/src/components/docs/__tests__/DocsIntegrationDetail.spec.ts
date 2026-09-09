import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createPinia } from 'pinia'
import DocsIntegrationDetail from '../DocsIntegrationDetail.vue'
import detailSource from '../DocsIntegrationDetail.vue?raw'
import { clientMatrixBySlug } from '@/generated/clientMatrix'

const RouterLinkStub = {
  props: ['to'],
  template: '<a :href="to"><slot /></a>',
}

const mountDetail = (slug: string) => mount(DocsIntegrationDetail, {
  props: { client: clientMatrixBySlug[slug] },
  global: { plugins: [createPinia()], stubs: { RouterLink: RouterLinkStub } },
})

describe('DocsIntegrationDetail', () => {
  it('renders the new minimal Codex guide without restoring legacy or one-click content', () => {
    const wrapper = mountDetail('integration-codex')

    expect(wrapper.attributes('data-client-status')).toBe('manual')
    expect(wrapper.text()).toContain('手动配置')
    expect(wrapper.text()).toContain('手动配置 Codex')
    expect(wrapper.text()).toContain('~/.codex/config.toml')
    expect(wrapper.text()).toContain('https://api.laoshirenai.com/v1')
    expect(wrapper.text()).toContain('LSRAI_API_KEY')
    expect(wrapper.text()).toContain('YOUR_MODEL_ID')
    expect(wrapper.text()).toContain('验证连接')
    expect(wrapper.text()).not.toContain('supports_websockets = true')
    expect(wrapper.text()).not.toContain('responses_websockets_v2 = true')
    expect(wrapper.find('.manual-config').exists()).toBe(false)
    expect(wrapper.find('.steps').exists()).toBe(false)
    expect(wrapper.find('.files').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('复制真实命令')
    expect(wrapper.text()).not.toContain('读取当前分组模型')
  })

  it('keeps unfinished tools on the temporary hidden-content notice', () => {
    const wrapper = mountDetail('integration-qoder')
    expect(wrapper.attributes('data-client-status')).toBe('paused')
    expect(wrapper.text()).toContain('教程整理中')
    expect(wrapper.text()).toContain('旧版内容已暂时隐藏')
    expect(wrapper.find('.simple-guide').exists()).toBe(false)
  })

  it('keeps the legacy implementation in source for later revision', () => {
    expect(detailSource).toContain('legacyIntegrationDetailsVisible = false')
    expect(detailSource).toContain('<ClientManualConfig')
    expect(detailSource).toContain('<ClaudeCodeManualConfig')
    expect(detailSource).toContain('自动配置')
    expect(detailSource).toContain('配置会修改哪些文件')
  })
})
