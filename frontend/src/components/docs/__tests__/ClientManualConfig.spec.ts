import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { clientMatrixBySlug } from '@/generated/clientMatrix'
import ClientManualConfig from '../ClientManualConfig.vue'
import DocsTerminalCommand from '../DocsTerminalCommand.vue'

async function fillReady(slug: string, model: string) {
  const wrapper = mount(ClientManualConfig, { props: { client: clientMatrixBySlug[slug] } })
  await wrapper.get('input[type="password"]').setValue('fixture-key')
  await wrapper.get('.model-field input').setValue(model)
  return wrapper
}

describe('ClientManualConfig ready clients', () => {
  it('generates Grok Build manual write and real verification commands', async () => {
    const wrapper = await fillReady('integration-grok-build', 'gpt-5.6-sol')
    const write = wrapper.findAllComponents(DocsTerminalCommand).find(item => String(item.props('label')).includes('写入配置'))
    expect(wrapper.findAll('.protocol-options button').map(button => button.text())).toEqual(['Responses'])
    expect(write?.props('command')).toContain("LAOSHIRENAI_GROK_API_KEY='fixture-key'")
    expect(write?.props('command')).toContain("LAOSHIRENAI_PROTOCOL='responses'")
    expect(write?.props('displayCommand')).not.toContain('fixture-key')
    expect(wrapper.text()).toContain('GROK_OK')
    expect(wrapper.text()).toContain('[model."模型ID"]')
  })

  it('generates Gemini CLI manual write and real verification commands', async () => {
    const wrapper = await fillReady('integration-gemini-cli', 'gemini-3.7-flash')
    const write = wrapper.findAllComponents(DocsTerminalCommand).find(item => String(item.props('label')).includes('写入配置'))
    expect(wrapper.findAll('.protocol-options button').map(button => button.text())).toEqual(['GenerateContent'])
    expect(write?.props('command')).toContain("LAOSHIRENAI_GEMINI_API_KEY='fixture-key'")
    expect(write?.props('command')).toContain("LAOSHIRENAI_PROTOCOL='generate_content'")
    expect(wrapper.text()).toContain('GEMINI_OK')
    expect(wrapper.text()).toContain('settings.model.name')
  })
})

describe('ClientManualConfig one-click-only clients', () => {
  it('shows verified protocol intersections and points to the key page without fabricating a manual write command', async () => {
    const wrapper = mount(ClientManualConfig, { props: { client: clientMatrixBySlug['integration-opencode'] } })
    await wrapper.get('.model-field input').setValue('qwen3.7-max')
    expect(wrapper.findAll('.protocol-options button').map(button => button.text())).toEqual(['Responses', 'Chat Completions'])
    expect(wrapper.findAllComponents(DocsTerminalCommand).some(item => String(item.props('label')).includes('写入配置'))).toBe(false)
    expect(wrapper.text()).toContain('一键配置入口位于 API 密钥页面')
    expect(wrapper.text()).not.toContain('不要执行占位命令')
  })

  it('fails closed when a client has no public protocol', async () => {
    const wrapper = mount(ClientManualConfig, { props: { client: clientMatrixBySlug['integration-qoder'] } })
    expect(wrapper.find('.no-protocol-manual').exists()).toBe(true)
    expect(wrapper.text()).toContain('没有可公开接入的协议')
    expect(wrapper.find('input[type="password"]').exists()).toBe(false)
    expect(wrapper.findAllComponents(DocsTerminalCommand).some(item => String(item.props('label')).includes('写入配置'))).toBe(false)
  })
})
