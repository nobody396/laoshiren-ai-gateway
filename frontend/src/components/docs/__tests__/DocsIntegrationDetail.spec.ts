import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DocsIntegrationDetail from '../DocsIntegrationDetail.vue'
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
  it('renders a ready client with automatic and guided manual configuration', async () => {
    const wrapper = mountDetail('integration-codex')

    expect(wrapper.attributes('data-client-status')).toBe('ready')
    expect(wrapper.text()).toContain('配置基线 · 0.151.0')
    expect(wrapper.text()).toContain('一键导入可用')
    expect(wrapper.text()).toContain('配置会修改哪些文件')
    expect(wrapper.text()).toContain('~/.codex/auth.json')
    expect(wrapper.text()).toContain('review_model')
    expect(wrapper.text()).toContain('codex --version + Shell/Read 工具续轮 + 固定完成标记')
    expect(wrapper.text()).not.toContain('手动配置合同')
    expect(wrapper.find('.manual-contract').exists()).toBe(false)
    expect(wrapper.find('.manual-config').exists()).toBe(true)
    expect(wrapper.text()).toContain('读取当前分组模型')
    expect(wrapper.text()).toContain('填充模型槽位')
    expect(wrapper.text()).toContain('codex --version')
    expect(wrapper.text()).not.toContain('已验证版本')
    expect(wrapper.text()).not.toContain('截图待补')
    expect(wrapper.text()).not.toContain('<版本化安装脚本 URL>')

    await wrapper.get('input[type="password"]').setValue('fixture-key')
    await wrapper.get('.model-field input').setValue('gpt-5.6-sol')
    expect(wrapper.findAll('.effort-labels button').map(button => button.text())).toEqual(['自动', '关闭', 'Low', 'Med', 'High', 'XHigh', 'Max'])
    expect(wrapper.text()).toContain('Ultracode 是 Claude Code 的单次工作流模式')
    expect(wrapper.text()).toContain('只用于 Codex 的 /review 和 codex review')
    expect(wrapper.text()).toContain('不是需要手填的参数')
    expect(wrapper.text()).toContain('CODEX_OK')
    expect(wrapper.get('.manual-config').text()).not.toContain('打开 API 密钥页面')
  })

  it('preserves the Claude Code manual interaction and real verification command', () => {
    const wrapper = mountDetail('integration-claude-code')

    expect(wrapper.find('.manual-config').exists()).toBe(true)
    expect(wrapper.text()).toContain('复制真实命令')
    expect(wrapper.text()).toContain('CLAUDE_CODE_OK')
    expect(wrapper.find('.step-terminal').exists()).toBe(true)
  })

  it('renders the same five-step manual flow for every non-Claude client', async () => {
    const wrapper = mountDetail('integration-opencode')
    const mainModel = wrapper.get('.model-field input')

    expect(wrapper.text()).toContain('暂不提供一键命令')
    expect(wrapper.find('.manual-config').exists()).toBe(true)
    expect(wrapper.findAll('.manual-config ol > li')).toHaveLength(5)
    expect(wrapper.text()).toContain('读取当前分组模型')
    expect(wrapper.text()).toContain('填充模型槽位')

    await mainModel.setValue('gpt-5.6-sol')
    expect(wrapper.findAll('.effort-labels button')).toHaveLength(0)
    expect(wrapper.text()).toContain('没有已验证的持久化字段或命令参数')
    expect(wrapper.text()).not.toContain('Ultra')
  })
})
