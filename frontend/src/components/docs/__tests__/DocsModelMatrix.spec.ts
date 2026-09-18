import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import DocsModelMatrix from '../DocsModelMatrix.vue'

describe('DocsModelMatrix', () => {
  it('renders every model contract with symbol-only protocol cells and reasoning levels', () => {
    const wrapper = mount(DocsModelMatrix, {
      global: { stubs: { 'router-link': { template: '<a><slot /></a>' } } },
    })
    const text = wrapper.text()

    expect(text).toContain('模型能力矩阵')
    expect(text).toContain('客户端能力矩阵')

    const solRow = wrapper.findAll('tbody tr').find(row => row.text().includes('gpt-5.6-sol'))
    expect(solRow).toBeDefined()
    const solCells = solRow!.findAll('td')
    expect(solCells[0].text()).toBe('★')
    expect(solCells[1].text()).toBe('✗')
    expect(solCells[2].text()).toBe('✗')
    expect(solCells[3].text()).toBe('✗')
    expect(solRow!.text()).toContain('关闭 / Min / Low / Med / High / XHigh / Max')
    expect(solRow!.text()).toContain('1.05M')
    expect(solCells[0].find('span').attributes('aria-label')).toContain('推荐')
    expect(solCells[1].find('span').attributes('aria-label')).toContain('未在此协议上开放')

    const glmRow = wrapper.findAll('tbody tr').find(row => row.text().includes('glm-5.3'))
    const glmCells = glmRow!.findAll('td')
    expect(glmCells[0].text()).toBe('✗')
    expect(glmCells[1].text()).toBe('★')
    expect(glmCells[0].find('span').attributes('aria-label')).toContain('实测负向')

    const glm52Row = wrapper.findAll('tbody tr').find(row => row.text().includes('glm-5.2'))
    const glm52Cells = glm52Row!.findAll('td')
    expect(glm52Cells[0].text()).toBe('✓')
    expect(glm52Cells[1].text()).toBe('★')

    const haikuRow = wrapper.findAll('tbody tr').find(row => row.text().includes('claude-haiku-4-5'))
    expect(haikuRow!.text()).toContain('模型默认')
    expect(haikuRow!.text()).toContain('200K')
  })
})
