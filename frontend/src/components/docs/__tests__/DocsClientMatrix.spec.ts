import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DocsClientMatrix from '../DocsClientMatrix.vue'

describe('DocsClientMatrix', () => {
  it('renders the client matrix as per-protocol symbols', () => {
    const wrapper = mount(DocsClientMatrix, {
      global: { stubs: { 'router-link': { template: '<a><slot /></a>' } } },
    })
    const rows = wrapper.findAll('tbody tr')
    expect(rows.length).toBeGreaterThanOrEqual(14)
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
    expect(workbuddy!.text()).toContain('手动配置')

    const vscode = rows.find(row => row.text().includes('Visual Studio Code'))
    const vscodeCells = vscode!.findAll('td')
    expect(vscodeCells[1].text()).toBe('○')
    expect(vscodeCells[2].text()).toBe('○')
    expect(vscodeCells[3].text()).toBe('○')
    expect(vscodeCells[4].text()).toBe('✗')
  })
})
