import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { spawnSync } from 'node:child_process'
import { mount, type VueWrapper } from '@vue/test-utils'
import DocsTextGuide from '../DocsTextGuide.vue'
import { clientMatrix } from '@/generated/clientMatrix'
import { keySetupCommand, modelsCommand, textClientGuides } from '@/docs/guides/textClientGuides'
import routerSource from '@/router/index.ts?raw'

let wrapper: VueWrapper | undefined
const writeText = vi.fn().mockResolvedValue(undefined)
beforeEach(() => {
  vi.useFakeTimers()
  window.history.replaceState(null, '', '/text-docs-preview')
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.stubGlobal('IntersectionObserver', class { observe() {} unobserve() {} disconnect() {} })
  Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
  writeText.mockClear()
})
afterEach(() => {
  wrapper?.unmount()
  vi.runOnlyPendingTimers()
  vi.useRealTimers()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const open = () => (wrapper = mount(DocsTextGuide))

describe('text-only client guide preview', () => {
  it('covers every matrix client without opening a public route or configuration generator', () => {
    expect(new Set(textClientGuides.map(g => g.id))).toEqual(new Set(clientMatrix.map(c => c.id)))
    expect(textClientGuides).toHaveLength(14)
    const w = open()
    expect(w.findAll('nav[aria-label="工具配置文档"] button')).toHaveLength(14)
    expect(w.findAll('.overview-steps a')).toHaveLength(3)
    expect(w.find('input').exists()).toBe(false)
    expect(w.find('.manual-config').exists()).toBe(false)
    expect(routerSource).not.toContain('DocsTextGuide')
  })
  it('renders a client file example and switches OS-specific paths', async () => {
    const w = open()
    await w.get('[data-client-id="codex"]').trigger('click')
    expect(w.text()).toContain('~/.codex/config.toml')
    expect(w.text()).toContain('https://api.laoshirenai.com/v1')
    await w.findAll('.os-switch button').find(b => b.text() === 'Windows')!.trigger('click')
    expect(w.text()).toContain('%USERPROFILE%\\.codex\\config.toml')
    expect(w.text()).toContain('PowerShell 5.1 / 7')
    expect(w.find('input').exists()).toBe(false)
    expect(window.location.hash).toContain('client=codex&os=windows')
  })
  it('reuses the terminal component and copies the full unmodified file example', async () => {
    const w = open()
    await w.get('[data-client-id="opencode"]').trigger('click')
    expect(w.findAll('.config-example .terminal-dots i')).toHaveLength(3)
    await w.get('.config-example .terminal-actions button').trigger('click')
    const expected = textClientGuides.find(g => g.id === 'opencode')!.blocks[0].content
    expect(writeText).toHaveBeenCalledWith(expected)
    expect(JSON.parse(expected).provider.laoshirenai.options.apiKey).toBe('{env:LAOSHIRENAI_API_KEY}')
    expect(w.find('.run-button').exists()).toBe(false)
  })
  it('shows tool-specific prerequisites without inventing file templates, and does not offer WorkBuddy Linux setup', async () => {
    const w = open()
    await w.get('[data-client-id="qoder"]').trigger('click')
    expect(w.text()).toContain('本文参考 1.1.42')
    expect(w.text()).toContain('Custom URL...')
    expect(w.text()).toContain('尚未完成本站模型调用验收')
    expect(w.text()).toContain('发给它的服务做 BYOK 校验')
    expect(w.text()).not.toContain('这篇配置模板还在核对')
    expect(w.find('.credential-window').exists()).toBe(false)
    expect(w.get('#step-config').text()).not.toContain('文件不存在时先创建')
    await w.get('[data-client-id="minimax-code"]').trigger('click')
    expect(w.text()).toContain('本文参考 0.3.2')
    expect(w.text()).toContain('明文写入 config.yaml')
    expect(w.text()).toContain('openai-responses')
    expect(w.text()).toContain('没有写入真实 Key')
    expect(w.find('.credential-window').exists()).toBe(false)
    expect(w.findAll('.config-example')).toHaveLength(2)
    await w.findAll('.config-example .terminal-actions button')[1].trigger('click')
    expect(writeText).toHaveBeenLastCalledWith('/model')
    expect(w.get('#step-config').text()).not.toContain('文件不存在时先创建')
    await w.get('[data-client-id="workbuddy"]').trigger('click')
    await w.findAll('.os-switch button').find(b => b.text() === 'Linux')!.trigger('click')
    expect(w.text()).toContain('此工具暂不提供 Linux 配置')
    expect(w.find('#step-config').exists()).toBe(false)
  })
  it('provides Kimi environment-only setup and copies only the selected OS syntax', async () => {
    const w = open()
    await w.get('[data-client-id="kimi-code"]').trigger('click')
    expect(w.text()).toContain('本文参考 0.40.1')
    expect(w.text()).not.toContain('这篇配置模板还在核对')
    expect(w.get('#step-key code').text()).toContain('KIMI_MODEL_API_KEY="$LAOSHIRENAI_API_KEY"')
    expect(w.findAll('.config-example')).toHaveLength(1)
    expect(w.get('.config-example code').text()).toContain("export KIMI_MODEL_PROVIDER_TYPE='openai_responses'")
    expect(w.get('#step-config').text()).not.toContain('文件不存在时先创建')
    await w.get('.config-example .terminal-actions button').trigger('click')
    expect(writeText).toHaveBeenLastCalledWith(textClientGuides.find(g => g.id === 'kimi-code')!.blocks[0].content)
    await w.findAll('.os-switch button').find(b => b.text() === 'Windows')!.trigger('click')
    expect(w.findAll('.config-example')).toHaveLength(1)
    expect(w.get('.config-example code').text()).toContain("$env:KIMI_MODEL_NAME = 'YOUR_MODEL_ID'")
    expect(w.get('.config-example code').text()).not.toContain('export ')
    await w.get('.config-example .terminal-actions button').trigger('click')
    expect(writeText).toHaveBeenLastCalledWith(textClientGuides.find(g => g.id === 'kimi-code')!.blocks[1].content)
    await w.findAll('.os-switch button').find(b => b.text() === 'Linux')!.trigger('click')
    expect(w.get('.config-example code').text()).toContain('export KIMI_MODEL_NAME=')
    expect(w.find('input').exists()).toBe(false)
    expect(w.text()).toContain('本教程不修改')
  })
  it('keeps Hermes WSL commands separate from PowerShell', async () => {
    const w = open()
    await w.get('[data-client-id="hermes-agent"]').trigger('click')
    await w.findAll('.os-switch button').find(b => b.text() === 'Windows')!.trigger('click')
    expect(w.text()).toContain('Windows：请先进入 WSL')
    expect(w.text()).toContain('WSL · Bash / Zsh')
    expect(w.get('#step-key .docs-terminal-command code').text()).toContain('read -rs')
  })
  it('binds copy to its audited version rather than silently claiming a newer matrix version', async () => {
    const w = open()
    await w.get('[data-client-id="claude-code"]').trigger('click')
    expect(w.text()).toContain('本文参考 2.1.251')
    const client = clientMatrix.find(c => c.id === 'claude-code')!
    if (client.version_key !== 'cli:2.1.251') expect(w.text()).not.toContain(`本文参考 ${client.version}`)
    expect(w.find('[role="tablist"]').exists()).toBe(false)
    expect(w.get('.os-switch button[aria-pressed="true"]').text()).toBe('macOS')
  })
  it('keeps JSON examples parseable and secret prompts usable without printing the supplied value', () => {
    for (const guide of textClientGuides) {
      for (const block of guide.blocks.filter(block => block.label.includes('.json'))) {
        expect(() => JSON.parse(block.content)).not.toThrow()
      }
    }
    const command = keySetupCommand('macos', 'GEMINI_API_KEY') + '; test "$GEMINI_API_KEY" = fixture-doc-value'
    const result = spawnSync('bash', ['--noprofile', '--norc', '-c', command], { input: 'fixture-doc-value\n', encoding: 'utf8', env: { PATH: process.env.PATH } })
    expect(result.status).toBe(0)
    expect(result.stdout).not.toContain('fixture-doc-value')
    expect(result.stdout.endsWith('\n')).toBe(true)
  })
  it('keeps secret entry on one physical line and out of literal arguments', () => {
    for (const os of ['macos', 'windows', 'linux'] as const) {
      const command = keySetupCommand(os, 'GEMINI_API_KEY', { GOOGLE_GEMINI_BASE_URL: 'https://api.laoshirenai.com' })
      expect(command).not.toMatch(/[\r\n]/)
      expect(command).toContain('LAOSHIRENAI_API_KEY')
      expect(command).not.toContain('YOUR_API_KEY')
    }
    expect(modelsCommand('macos')).toContain('--config -')
    expect(modelsCommand('macos')).not.toContain('-H ')
    expect(keySetupCommand('windows')).toContain('-AsSecureString')
  })
})
