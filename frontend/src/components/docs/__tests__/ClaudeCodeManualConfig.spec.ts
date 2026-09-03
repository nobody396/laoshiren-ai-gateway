import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const getGatewayModels = vi.hoisted(() => vi.fn())
vi.mock('@/api/gatewayModels', () => ({ getGatewayModels }))
import ClaudeCodeManualConfig from '../ClaudeCodeManualConfig.vue'

describe('ClaudeCodeManualConfig reasoning slider', () => {
  it('defaults to High and filters levels from the model reasoning matrix', async () => {
    const wrapper = mount(ClaudeCodeManualConfig)
    expect(wrapper.find('.effort-heading strong').text()).toBe('High')

    await wrapper.get('.model-field input').setValue('claude-opus-4-6')
    const opus46 = wrapper.findAll('.effort-labels button').map(button => button.text())
    expect(opus46).toEqual(['自动', 'Low', 'Med', 'High', 'XHigh', 'Max', 'Ultra'])
    await wrapper.findAll('.effort-labels button')[4].trigger('click')
    expect(wrapper.find('.effort-mapping').text()).toContain('xhigh 降级为 high')

    await wrapper.get('.model-field input').setValue('claude-haiku-4-5')
    expect(wrapper.findAll('.effort-labels button').map(button => button.text())).toEqual(['自动'])
    expect(wrapper.find('.effort-heading strong').text()).toBe('自动')

    await wrapper.get('.model-field input').setValue('unknown-messages-model')
    expect(wrapper.findAll('.effort-labels button').map(button => button.text())).toEqual(['自动'])
    expect(wrapper.find('.effort-warning').exists()).toBe(true)
  })

  it('runs model discovery in the page and lets the user select a returned model', async () => {
    getGatewayModels.mockResolvedValue(['claude-sonnet-5', 'claude-opus-5'])
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    const wrapper = mount(ClaudeCodeManualConfig)
    await wrapper.get('input[type="password"]').setValue('fixture-key')
    await wrapper.get('.docs-terminal-command .run-button').trigger('click')
    await flushPromises()

    expect(getGatewayModels).toHaveBeenCalledWith('/__gateway', 'fixture-key')
    expect(wrapper.findAll('.model-result-row')).toHaveLength(2)
    await wrapper.findAll('.copy-model')[0].trigger('click')
    expect(writeText).toHaveBeenCalledWith('claude-sonnet-5')
    await wrapper.findAll('.set-main-button')[1].trigger('click')
    expect((wrapper.get('.model-field input').element as HTMLInputElement).value).toBe('claude-opus-5')
  })
})
