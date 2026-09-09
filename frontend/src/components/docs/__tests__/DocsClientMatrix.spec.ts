import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DocsClientMatrix from '../DocsClientMatrix.vue'
import { simpleClientGuides } from '@/docs/guides/simpleClientGuides'

describe('DocsClientMatrix', () => {
  it('renders the client matrix as per-protocol symbols', () => {
    const wrapper = mount(DocsClientMatrix, {
      global: { stubs: { 'router-link': { template: '<a><slot /></a>' } } },
    })
    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(simpleClientGuides.length)
    expect(wrapper.text()).toContain('客户端能力矩阵')

    const claude = rows.find(row => row.text().includes('Claude Code'))
    const claudeCells = claude!.findAll('td')
    // Responses ✗ / Chat ✗ / Messages ✓ / GenerateContent ✗
    expect(claudeCells[1].text()).toBe('✗')
    expect(claudeCells[2].text()).toBe('✗')
    expect(claudeCells[3].text()).toBe('✓')
    expect(claudeCells[4].text()).toBe('✗')
    expect(claudeCells[3].find('span').attributes('aria-label')).toContain('已验证')
    expect(claude!.text()).toContain('可用')

    const workbuddy = rows.find(row => row.text().includes('WorkBuddy'))
    const workbuddyCells = workbuddy!.findAll('td')
    // chat_completions 已跑通真实闭环：✓ 已验证，其余 ✗
    expect(workbuddyCells[1].text()).toBe('✗')
    expect(workbuddyCells[2].text()).toBe('✓')
    expect(workbuddyCells[2].find('span').attributes('aria-label')).toContain('已验证')
    expect(workbuddy!.text()).toContain('可用')

    expect(wrapper.text()).not.toContain('Visual Studio Code Local Agent')
    expect(wrapper.text()).not.toContain('Gemini CLI')
    expect(wrapper.text()).not.toContain('Hermes Agent')
    expect(wrapper.text()).not.toContain('Qoder')
    expect(wrapper.text()).not.toContain('MiniMax Code')
    expect(wrapper.text()).not.toContain('DeepSeek Harness')
  })
})
