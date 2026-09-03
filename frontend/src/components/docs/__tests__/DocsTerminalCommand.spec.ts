import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import DocsTerminalCommand from '../DocsTerminalCommand.vue'

describe('DocsTerminalCommand', () => {
  it('shows structured terminal chrome and copies the executable command', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    const wrapper = mount(DocsTerminalCommand, {
      props: {
        label: '验证',
        command: 'claude -p "OK"',
        displayCommand: 'claude -p \\\n  "OK"',
      },
    })

    expect(wrapper.findAll('.terminal-dots i')).toHaveLength(3)
    expect(wrapper.find('pre').text()).toContain('claude -p')
    await wrapper.get('button').trigger('click')
    expect(writeText).toHaveBeenCalledWith('claude -p "OK"')
  })

  it('emits run only when a runnable command is available', async () => {
    const wrapper = mount(DocsTerminalCommand, { props: { command: 'echo ok', runnable: true } })
    await wrapper.get('.run-button').trigger('click')
    expect(wrapper.emitted('run')).toHaveLength(1)
  })
})
